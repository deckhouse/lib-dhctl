// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package termui

import (
	"io"
	"strings"
	"sync"
	"time"
)

// Options configures a Block. The unexported fields are injection points for tests;
// production code leaves them nil and New fills defaults.
type Options struct {
	Color  bool
	Banner []string
	Caps   caps

	now    func() time.Time
	width  func() int
	height func() int
	tick   <-chan time.Time
	resize <-chan struct{}
}

// Block is a pinned, fixed-height live region on the alternate screen. It owns all
// pterm usage, its mutex, the spinner ticker and the resize watcher. All exported
// methods are safe for concurrent use.
type Block struct {
	mu   sync.Mutex
	w    io.Writer
	opts Options

	active     bool
	paused     bool
	shownLines int
	start      time.Time
	frac       float64
	title      string
	action     string
	spinnerIdx int
	banner     []string // startup ASCII banner; pinned at top when terminal is tall enough

	connString    string   // pinned connection string; set once via SetConnString
	logbox        []string // capped display ring
	milestonesAll []string // full history; the live region shows the most-recent that fit
	warnsAll      []string // includes ERROR-level lines (Error routes through Warn)

	stop       chan struct{}
	summarized bool // closing dump printed once (guards Finish + Restore)
}

var spinnerFrames = []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")

func New(w io.Writer, opts Options) *Block {
	if w == nil {
		w = io.Discard
	}
	if opts.now == nil {
		opts.now = time.Now
	}
	if opts.width == nil {
		opts.width = terminalWidth
	}
	if opts.height == nil {
		opts.height = terminalHeight
	}
	if opts.Caps == (caps{}) {
		opts.Caps = caps{warn: 5, logboxMin: 3}
	}
	return &Block{w: w, opts: opts}
}

func (b *Block) Start(title string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.active {
		return
	}
	b.active = true
	b.title = title
	b.start = b.opts.now()
	b.stop = make(chan struct{})
	_, _ = io.WriteString(b.w, ansiEnterAlt+ansiHideCur)
	b.repaintLocked()
	b.startTickerLocked()
	b.startResizeLocked()
}

// Finish ends the live UI: leaves the alt screen and prints the closing summary.
func (b *Block) Finish() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.summarizeLocked()
}

// Restore is the safety teardown (signal/panic/early error exit). Like Finish it
// leaves the alt screen and prints the closing summary, so a fatal error — which
// never reaches Finish — is still surfaced on the main screen instead of vanishing
// with the alt buffer. Safe to call multiple times and after Finish.
func (b *Block) Restore() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.summarizeLocked()
}

// summarizeLocked leaves the alt screen (idempotent, no-op if never started) and
// dumps the banner + milestones + warns/errors to the main screen exactly once. It
// dumps even when the block was never active (e.g. an error before the first
// progress session), so early failures are not lost.
func (b *Block) summarizeLocked() {
	b.restoreLocked()
	if b.summarized {
		return
	}
	b.summarized = true
	for _, bl := range b.banner {
		_, _ = io.WriteString(b.w, bl+"\n")
	}
	for _, m := range b.milestonesAll {
		_, _ = io.WriteString(b.w, m+"\n")
	}
	for _, wln := range b.warnsAll {
		_, _ = io.WriteString(b.w, wln+"\n")
	}
	if b.connString != "" {
		_, _ = io.WriteString(b.w, b.connString+"\n")
	}
}

// restoreLocked leaves the alt screen and stops the goroutines. Idempotent.
func (b *Block) restoreLocked() {
	if !b.active {
		return
	}
	b.active = false
	close(b.stop)
	// Clear from the cursor down as the main screen comes back. Leaving the alternate screen
	// restores the main screen exactly as it was when the run started - the shell prompt, the
	// command that started this run, and whatever the previous run left below it - with the
	// cursor back where it was, i.e. with all of that still sitting on the rows about to be
	// written. The closing summary and everything printed after it are written with \n and no
	// erase, so each line covers only its own width and the tail of the older, longer line
	// underneath survives to the right of it: "Control plane readiness" ending in the remains
	// of a path or of the previous run's command line. Erasing once here, rather than per line,
	// also covers the deprecation notices and anything else printed after the summary.
	_, _ = io.WriteString(b.w, ansiShowCur+ansiLeaveAlt+ansiClearEOS)
}

// repaintLocked renders the current frame in place: home, write each line + clear-EOL,
// then clear everything below. Never a full-screen clear → no flicker.
func (b *Block) repaintLocked() {
	lines := renderFrame(b.frameLocked())
	var sb strings.Builder
	sb.WriteString(ansiHome)
	for _, ln := range lines {
		sb.WriteString(ln)
		sb.WriteString(ansiClearEOL)
		sb.WriteString("\n")
	}
	sb.WriteString(ansiClearEOS)
	_, _ = io.WriteString(b.w, sb.String())
	b.shownLines = len(lines)
}

// SetBanner stores the startup ASCII banner lines and triggers a repaint.
// The banner is shown only when the terminal is tall enough that showing it still
// leaves room for the full live region — it is the first thing dropped under degradation.
func (b *Block) SetBanner(lines []string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.banner = lines
	if b.active && !b.paused {
		b.repaintLocked()
	}
}

// passthroughLocked prints a transient line straight to the writer during a pause (interactive
// input). It is NOT buffered into the block's rings: the line is the caller's transient I/O — e.g.
// a y/n prompt — and Resume repaints the block fresh, consuming whatever was printed here. The
// record is already captured by the always-on file sink, so the block loses nothing.
func (b *Block) passthroughLocked(line string) {
	_, _ = io.WriteString(b.w, line+"\n")
}

// SetConnString stores the SSH connection string and triggers a repaint.
// The string is rendered as a pinned cyan line just above the logbox and is never
// scrolled away by milestones. It is also included in the closing summary dump.
func (b *Block) SetConnString(s string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.connString = formatMilestone(b.opts.Color, "CONN", s)
	if b.active && !b.paused {
		b.repaintLocked()
	}
}

// visibleWarnsLocked picks the warn lines the live region shows, newest record first and each
// record from its head down, up to Caps.warn lines in all.
//
// Per record, not per line: a record that does not fit loses its tail rather than its head. An
// error's first line is its message and the rest is context - a terraform failure ends in the
// file and line it came from - so keeping the newest lines kept the context of the newest error
// and dropped the error. Returned in the order they were logged; the closing summary is not
// bounded by the terminal and still prints every record whole.
func (b *Block) visibleWarnsLocked() []string {
	room := b.opts.Caps.warn
	var out []string
	for i := len(b.warnsAll) - 1; i >= 0 && room > 0; i-- {
		lines := strings.Split(b.warnsAll[i], "\n")
		if len(lines) > room {
			lines = lines[:room]
		}
		room -= len(lines)
		out = append(lines, out...)
	}
	return out
}

func (b *Block) frameLocked() frame {
	width := b.opts.width()
	height := b.opts.height()
	connLine := 0
	if b.connString != "" {
		connLine = 1
	}
	warns := b.visibleWarnsLocked()
	lay := computeLayout(height, len(b.milestonesAll), len(warns), len(b.banner), connLine, b.opts.Caps)
	var banner []string
	if lay.banner {
		banner = b.banner
	}
	return frame{
		title:      b.title,
		frac:       b.frac,
		elapsed:    b.opts.now().Sub(b.start),
		action:     b.action,
		spinner:    spinnerFrames[b.spinnerIdx%len(spinnerFrames)],
		milestones: b.milestonesAll,
		warns:      warns,
		logbox:     b.logbox,
		width:      width,
		color:      b.opts.Color,
		banner:     banner,
		connString: b.connString,
		lay:        lay,
	}
}

// Milestone takes the box prefix of the process block the line came from, on the same terms as
// Warn: it is applied only when the line is printed inline, and dropped for the pinned milestone
// region and the closing summary, which render milestones detached from any block.
func (b *Block) Milestone(prefix, status, text string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	line := formatMilestone(b.opts.Color, status, text)
	if b.paused {
		b.passthroughLocked(prefix + line)
		return
	}
	b.milestonesAll = append(b.milestonesAll, line)
	if b.active {
		b.repaintLocked()
	}
}

func appendCapped(ring []string, line string, capN int) []string {
	ring = append(ring, line)
	if len(ring) > capN {
		ring = ring[len(ring)-capN:]
	}
	return ring
}

func (b *Block) SetProgress(frac float64, title string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.frac = frac
	if title != "" {
		b.title = title
	}
	if b.active && !b.paused {
		b.repaintLocked()
	}
}

func (b *Block) SetAction(text string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.action = text
	if b.active && !b.paused {
		b.repaintLocked()
	}
}

// Warn takes the box prefix of the process block the line came from (empty at top level).
// The prefix is applied only on the paths that print the line inline, interleaved with that
// block's other lines - during a pause, before the block is active, and after the closing dump -
// where dropping it would tear the ┌/│/└ frame. The pinned warn region and the closing summary
// render warnings detached from any block, so there the prefix is dropped: it would draw a frame
// edge next to a frame that is not on screen.
func (b *Block) Warn(prefix, line string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.paused || !b.active {
		b.passthroughLocked(prefix + line)
		return
	}
	if b.summarized {
		b.writeLineLocked(prefix + line)
		return
	}
	b.warnsAll = append(b.warnsAll, line)
	if b.active {
		b.repaintLocked()
	}
}

func (b *Block) writeLineLocked(line string) {
	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}
	_, _ = io.WriteString(b.w, line)
}

func (b *Block) Log(line string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.paused || !b.active {
		b.passthroughLocked(line)
		return
	}
	b.logbox = appendCapped(b.logbox, line, logboxRingCap)
	if b.active {
		b.repaintLocked()
	}
}

func (b *Block) startTickerLocked() {
	tick := b.opts.tick
	stop := b.stop
	if tick == nil {
		tk := time.NewTicker(120 * time.Millisecond)
		tick = tk.C
		go func() { <-stop; tk.Stop() }()
	}
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-tick:
				b.mu.Lock()
				if b.active && !b.paused {
					b.spinnerIdx++
					b.repaintLocked()
				}
				b.mu.Unlock()
			}
		}
	}()
}

func (b *Block) Pause() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.active || b.paused {
		return
	}
	b.paused = true
	_, _ = io.WriteString(b.w, ansiHome+ansiClearEOS+ansiShowCur)
}

func (b *Block) Resume() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.active || !b.paused {
		return
	}
	b.paused = false
	_, _ = io.WriteString(b.w, ansiHideCur)
	b.repaintLocked()
}

func (b *Block) startResizeLocked() {
	rz := b.opts.resize
	stop := b.stop
	if rz == nil {
		rz = notifyResize(stop)
	}
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-rz:
				b.mu.Lock()
				if b.active && !b.paused {
					b.repaintLocked()
				}
				b.mu.Unlock()
			}
		}
	}()
}
