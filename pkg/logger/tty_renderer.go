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

package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/pterm/pterm"
)

// lineSink is the text-output surface every TTY backend implements: it receives the rendered
// milestones, warn/error lines, ordinary detail (including the framed process boxes), the banner,
// and the connection string. termui.Block routes them into its pinned live region; plainProgressUI
// prints them straight to the writer (the logboek-style dump).
type lineSink interface {
	// Milestone receives a curated SUCCESS/WARNING/DEPRECATED/FAILED line together with the box
	// prefix of the process block it was emitted from, on the same terms as Warn below: a backend
	// that prints it inline must prepend the prefix, one that pins it in a region of its own drops it.
	Milestone(prefix, status, text string)
	// Warn receives a Warn+ line together with the box prefix of the process block it was
	// logged from (empty at top level). A backend that prints the line inline, interleaved
	// with the block's other lines, must prepend the prefix or the ┌/│/└ frame tears apart;
	// one that pins the line in a region of its own drops the prefix, which would point at
	// a frame that is not there.
	Warn(prefix, line string)
	Log(line string)          // ephemeral detail line (framed boxes + indented detail)
	SetBanner(lines []string) // pin the startup ASCII banner at the top of the live canvas
	SetConnString(s string)   // pin the SSH connection string just above the logbox
}

// progressBar is the pinned-bar surface only the live termui.Block implements. The plain logboek
// backend has no bar, so the renderer holds it as an optional dependency (nil for plain) and drives
// it only when present — instead of forcing the plain backend to no-op these six methods.
type progressBar interface {
	Start(name string)                      // open a pinned bar titled name
	SetProgress(frac float64, title string) // advance bar to frac (0..1), set bar title
	SetAction(text string)                  // current-action line under the bar
	Finish()                                // close the bar
	Pause()                                 // stop rendering around interactive input
	Resume()                                // restart rendering after a pause
}

type procFrame struct {
	name  string
	start time.Time
	// untimed suppresses the "(N seconds)" tail when the block is closed. A block that only
	// frames a report - a list of failed resources, a summary - is not a measured operation,
	// and "(0.00 seconds)" next to its title is noise.
	untimed bool
}

// ttyRenderer is a slog.Handler that renders the legacy logboek-style UI: process blocks framed
// with ┌/│/└, nested by indentation, with durations; level-styled log lines; and an optional
// pinned progress bar driven by progress markers. Ordinary lines and box borders scroll above the
// bar when one is active.
type ttyRenderer struct {
	// mu serializes Handle across goroutines. Log lines come from the operation goroutine while the
	// progress bar is advanced from consumeProgress's goroutine; both mutate the shared sink/bar and
	// the box stack and write interleaving ANSI. mu is a pointer so WithAttrs/WithGroup clones (which
	// share the same sink) share the same lock. Held for the whole Handle body.
	mu *sync.Mutex

	sink            lineSink
	bar             progressBar // nil when the backend has no pinned bar (the plain logboek dump)
	out             io.Writer
	level           slog.Leveler
	color           bool // ANSI styling, real terminal only
	ephemeralDetail bool // see rendererConfig

	stack []procFrame

	// repeat collapses a run of identical consecutive output lines. Poll loops - waiting for a pod,
	// for a node to join, for resources to become ready - re-log the same status every second, and
	// hundreds of identical lines bury everything around them. Only the output sink collapses them;
	// the file sink is a separate handler and keeps every record.
	repeat repeatRun
	// milestone collapses a run of identical consecutive milestones. It is deliberately separate
	// from repeat: milestones are not consecutive as records - a retried process emits its start
	// marker, its detail and its failure between one milestone and the next - so the run has to
	// survive everything that lands between two of them, and only a different milestone breaks it.
	milestone milestoneRun
	// pendingSep holds the separator owed to a block that has just closed, or nil when none is
	// owed. It is printed only once something actually follows it, so a block that closes as the
	// last thing in its parent - or as the last thing printed at all - is not trailed by a stray
	// empty row.
	pendingSep *string
	// now is time.Now, replaced in tests.
	now func() time.Time
}

// repeatRun tracks the line currently being collapsed and how many times it has been seen since the
// last time it was printed.
type repeatRun struct {
	active bool   // a line has been printed and is a candidate to repeat; the zero run is not
	line   string // the styled text, without the box prefix
	prefix string // box prefix the line was emitted under; a depth change is a different line
	warn   bool   // whether the line routes through Warn, which some backends pin separately
	count  int    // occurrences suppressed since the run was last printed
	since  time.Time
}

// milestoneRun tracks the milestone currently being collapsed and how many further times it has
// been emitted since it was printed.
//
// Milestones are pinned and are dumped again in the closing summary, so a duplicate costs far more
// than a duplicate detail line: it holds a row of the live region for the whole run and a row of
// the summary afterwards.
//
// This is the backstop, not the main defence: a failure a retry loop absorbed never becomes a
// milestone at all (see failureIsNews). What is left for this to catch is repetition that is real
// - a step genuinely reached, and failed, more than once, or the same badge emitted by every item
// of a list - where the count is the news.
//
// There is no time bound here, unlike repeatFlushInterval: the line a run is collapsing is already
// on screen and stays there, so a silent run is never mistaken for a hang.
type milestoneRun struct {
	active               bool
	prefix, status, text string
	count                int // occurrences suppressed since the run was printed
}

// repeatFlushInterval bounds how long a run of identical lines may be collapsed silently. Without
// it a wait that repeats the same status for ten minutes would print nothing at all, and someone
// tailing the log has no way to tell a slow step from a hung one.
const repeatFlushInterval = 30 * time.Second

// rendererConfig holds the parameters of newTTYRenderer. bar is optional: nil for the plain backend.
type rendererConfig struct {
	out   io.Writer
	sink  lineSink
	bar   progressBar
	level slog.Leveler
	color bool // ANSI styling, real terminal only
	// ephemeralDetail marks a sink whose detail lines do not survive the run - the live block's
	// log box is wiped when it leaves the alternate screen. Anything that must outlive the run has
	// to be restated as a milestone there, and only there.
	ephemeralDetail bool
}

func newTTYRenderer(cfg rendererConfig) *ttyRenderer {
	// Resize handling now lives inside the UI (termui.Block watches SIGWINCH itself); the renderer
	// no longer starts a watcher.
	return &ttyRenderer{
		mu: &sync.Mutex{}, sink: cfg.sink, bar: cfg.bar, out: cfg.out,
		level: cfg.level, color: cfg.color, ephemeralDetail: cfg.ephemeralDetail, now: time.Now,
	}
}

func (h *ttyRenderer) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

// Handle renders r. Its message is assumed already redacted: the only production caller is
// TerminalUIHandler.Handle, which runs sanitizeMessage before fan-out, so the renderer never
// re-sanitizes and no render path can leak a secret.
func (h *ttyRenderer) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Progress-bar markers drive the optional bar; they carry no printable text - and they tick
	// often, so flushing a collapsed run on them would defeat the collapsing.
	if h.handleBarMarker(r) {
		return nil
	}

	// Everything below prints text. A record that is not an ordinary log line ends any run of
	// collapsed duplicates: the count must be reported before the next line, or it lands out of
	// order. Ordinary lines flush themselves, but only once they turn out to differ.
	if !isOrdinaryLine(r) {
		h.flushRepeat()
	}

	// The separator belongs between a closed block and whatever comes next. When what comes next is
	// another closing border, nothing came between and it is dropped.
	if ev := recordProcessEvent(r); ev == string(processEnd) || ev == string(processFail) {
		h.pendingSep = nil
	} else {
		h.flushSeparator()
	}

	if hasBanner(r) {
		h.sink.SetBanner(strings.Split(strings.TrimRight(r.Message, "\n"), "\n"))
		return nil
	}

	if hasConnectionString(r) {
		h.sink.SetConnString(strings.TrimRight(r.Message, "\n"))
		return nil
	}

	// Process blocks always render as framed boxes (┌ … └ with duration), matching the old logboek.
	// A process-start also updates the pinned bar's "current action" line when a bar is present (the
	// live Block); the plain logboek dump has no bar, so only the framed box is emitted.
	switch ev := recordProcessEvent(r); ev {
	case string(processStart):
		name := recordProcessName(r)
		if h.bar != nil {
			h.bar.SetAction(name)
		}
		h.scroll(h.prefix(len(h.stack)) + boxOpen + " " + h.styleTitle(name))
		h.stack = append(h.stack, procFrame{name: name, start: time.Now(), untimed: recordProcessUntimed(r)})
		return nil
	case string(processEnd), string(processFail):
		var f procFrame
		if n := len(h.stack); n > 0 {
			f = h.stack[n-1]
			h.stack = h.stack[:n-1]
		}
		title := h.styleTitle(f.name)
		tail := ""
		if !f.untimed {
			tail = fmt.Sprintf(" (%.2f seconds)", time.Since(f.start).Seconds())
		}
		if ev == string(processFail) {
			tail = " FAILED" + tail
			if h.color {
				title = pterm.NewStyle(pterm.FgRed).Sprint(f.name)
			}
		}
		h.scroll(h.prefix(len(h.stack)) + boxClose + " " + title + h.dim(tail))
		// Separator after a closed block: a blank row at top level, the enclosing guides inside a
		// block, so consecutive boxes are spaced apart without breaking the frame. It is held until
		// something actually follows - a sibling block or more detail - because with nothing after
		// it the row separates the block from its own parent's closing border and reads as a
		// rendering artifact. See pendingSep.
		sep := strings.TrimRight(h.prefix(len(h.stack)), " ")
		h.pendingSep = &sep
		if ev == string(processFail) && h.ephemeralDetail {
			// The framed box above is ephemeral detail (sink.Log → the live Block's logbox ring),
			// which is wiped when the Block leaves the alt screen on teardown. A failed process is
			// the primary failure signal and must outlive that: restate it as a persistent FAILED
			// milestone, which summarizeLocked keeps on the main screen.
			//
			// Only there. A backend that writes its lines out permanently already has the FAILED
			// border above, so the milestone added nothing but a duplicate - printed unprefixed,
			// mid-frame, which also tore the block apart.
			// Only at the top level: see failureIsNews.
			if len(h.stack) == 0 {
				h.emitMilestone(h.prefix(0), milestoneStatus(badgeFailed), f.name)
			}
		}
		return nil
	}

	// Curated status line: a milestone the sink renders as its own SUCCESS/WARNING/FAILED line.
	if status := badgeStatus(r); status != "" {
		h.emitMilestone(h.prefix(len(h.stack)), milestoneStatus(status), r.Message)
		return nil
	}

	// Ordinary log line(s). Warn and above are pinned by the sink; lower levels are ephemeral detail
	// indented to the current process depth.
	prefix := h.prefix(len(h.stack))
	lines := strings.Split(strings.TrimRight(r.Message, "\n"), "\n")

	// A warn+ record is pinned as one unit. The pinned region is only a few rows tall, so when a
	// record does not fit, something has to go - and splitting the record into separate lines
	// first made that decision line by line, keeping the newest rows. For a multi-line error that
	// is the wrong half: what survived was the file-and-line footer of a terraform error while the
	// error itself scrolled out from under it, leaving `107: resource "yandex_compute_instance"
	// "master" {` pinned with nothing to say what was wrong with it. Kept whole, the sink can drop
	// the record's tail instead and keep its head, which is the message.
	if r.Level >= slog.LevelWarn {
		for i, ln := range lines {
			lines[i] = h.styleText(r.Level, ln)
		}
		record := strings.Join(lines, "\n")
		if !h.absorbRepeat(prefix, record, true) {
			h.emitLine(prefix, record, true)
		}
		return nil
	}

	// Detail lines are routed individually, and each keeps the indent prefix so nested or tabular
	// content stays aligned under its block.
	for _, ln := range lines {
		styled := h.styleText(r.Level, ln)
		if h.absorbRepeat(prefix, styled, false) {
			continue
		}
		h.emitLine(prefix, styled, false)
	}
	return nil
}

// isOrdinaryLine reports whether r is plain log text, as opposed to a record the renderer turns
// into structure: a banner, a connection string, a process border, or a milestone.
func isOrdinaryLine(r slog.Record) bool {
	return !hasBanner(r) && !hasConnectionString(r) &&
		recordProcessEvent(r) == "" && badgeStatus(r) == ""
}

// emitLine routes one fully rendered line to the sink. Warn and above carry the box prefix
// alongside the text instead of embedded in it, because a backend that pins them in a region of
// their own must be able to drop it.
func (h *ttyRenderer) emitLine(prefix, line string, warn bool) {
	if warn {
		h.sink.Warn(prefix, line)
		return
	}
	h.sink.Log(prefix + line)
}

// absorbRepeat reports whether line was swallowed as a repeat of the line before it. A run is
// broken by any difference - the text, the box depth, or the level - and is printed anyway once it
// has been collapsing for repeatFlushInterval, so a long wait still shows it is making progress.
func (h *ttyRenderer) absorbRepeat(prefix, line string, warn bool) bool {
	now := h.now()
	start := repeatRun{active: true, line: line, prefix: prefix, warn: warn, since: now}

	if !h.repeat.active || line != h.repeat.line || prefix != h.repeat.prefix || warn != h.repeat.warn {
		h.flushRepeat()
		h.repeat = start
		return false
	}

	h.repeat.count++
	if now.Sub(h.repeat.since) < repeatFlushInterval {
		return true
	}

	// Long enough: print the line again with its count, then keep collapsing from zero.
	h.flushRepeat()
	h.repeat = start
	return true
}

// failureIsNews explains the depth test at the processFail branch above: a failed process block
// becomes a persistent FAILED milestone only when nothing encloses it.
//
// A failure inside another block is not an outcome. It is either an attempt - the overwhelming
// case is a retry loop, which frames itself as one block and calls its task inside it, so a task
// that opens a block of its own opens and fails one per attempt, and absorbing those is the whole
// point of the loop - or it is one of the steps by which the enclosing block failed, which the
// enclosing block is about to report under its own name. Either way, restating it in the one
// region that survives to the end is wrong: in the first case it claims a run failed forty times
// next to the SUCCESS line saying it finished, and in the second it states a single failure once
// per level it travels through and once per item it happened to, which is how one converge that
// did not converge came to occupy twelve rows.
//
// A milestone cannot be taken back once printed, which is why this is a test and not a cleanup.
// What is lost is the ancestry, and it is lost on purpose: the error text printed under the
// summary, the framed detail and the file log all say how it failed, and say it far better than a
// stack of badges can. The summary is left saying what failed - once.
//
// The one gap is a silent retry loop (NewSilentLoop opens no block of its own): its attempts are
// top-level and print. Silent loops are silent precisely because they wrap something not worth
// framing, so this has yet to come up in practice.

// emitMilestone prints a milestone unless it repeats the one before it, in which case it is
// counted and the count is reported when the run ends (see flushMilestone).
func (h *ttyRenderer) emitMilestone(prefix, status, text string) {
	if h.milestone.active &&
		h.milestone.prefix == prefix && h.milestone.status == status && h.milestone.text == text {
		h.milestone.count++
		return
	}

	h.flushMilestone()
	h.milestone = milestoneRun{active: true, prefix: prefix, status: status, text: text}
	h.sink.Milestone(prefix, status, text)
}

// flushMilestone reports how many further times the collapsed milestone occurred, and clears the
// run. A run of one has nothing to report: the milestone was already printed.
func (h *ttyRenderer) flushMilestone() {
	run := h.milestone
	h.milestone = milestoneRun{}

	if run.count == 0 {
		return
	}
	tail := fmt.Sprintf(" [repeated %d more times]", run.count)
	if run.count == 1 {
		tail = " [repeated once more]"
	}
	h.sink.Milestone(run.prefix, run.status, run.text+h.dim(tail))
}

// flushSeparator prints the separator owed by a previously closed block, if one is owed.
func (h *ttyRenderer) flushSeparator() {
	if h.pendingSep == nil {
		return
	}
	sep := *h.pendingSep
	h.pendingSep = nil
	h.scroll(sep)
}

// flushRepeat prints the pending run's line once more, tagged with how many occurrences it stands
// for, and clears the run. A run of one has nothing to report: the line was already printed.
func (h *ttyRenderer) flushRepeat() {
	run := h.repeat
	h.repeat = repeatRun{}

	if run.count == 0 {
		return
	}
	tail := fmt.Sprintf(" [repeated %d more times]", run.count)
	if run.count == 1 {
		tail = " [repeated once more]"
	}
	h.emitLine(run.prefix, run.line+h.dim(tail), run.warn)
}

// handleBarMarker drives the optional progress bar from a progress marker record and reports
// whether r was a bar marker (and thus fully consumed). When the backend has no bar (h.bar == nil,
// the plain logboek dump) the marker is silently consumed — it is renderer control, not text.
func (h *ttyRenderer) handleBarMarker(r slog.Record) bool {
	switch progressEvent(r) {
	case progressStart:
		if h.bar != nil {
			h.bar.Start(recordProgressName(r))
		}
		return true
	case progressEnd:
		// Last chance: Finish prints the closing summary, and a count reported after it would
		// land below the summary it belongs in - or, on the live block, be swallowed entirely.
		h.flushMilestone()
		if h.bar != nil {
			h.bar.Finish()
		}
		return true
	case progressPause:
		h.flushMilestone()
		if h.bar != nil {
			h.bar.Pause()
		}
		return true
	case progressResume:
		if h.bar != nil {
			h.bar.Resume()
		}
		return true
	}
	if v, ok := progressValue(r); ok {
		if h.bar != nil {
			h.bar.SetProgress(v, progressTitle(r))
		}
		return true
	}
	return false
}

// milestoneStatus maps an internal badge value to the UI milestone status string.
func milestoneStatus(badge string) string {
	switch badge {
	case badgeFailed:
		return "FAILED"
	case badgeWarning:
		return "WARNING"
	case badgeDeprecated:
		return "DEPRECATED"
	default:
		return "SUCCESS"
	}
}

const (
	boxOpen  = "┌"
	boxClose = "└"
	boxBody  = "│ "
)

func (h *ttyRenderer) prefix(depth int) string {
	if depth <= 0 {
		return ""
	}
	return strings.Repeat(boxBody, depth)
}

// scroll routes one rendered line (the framed-box borders/bodies) to the sink as ephemeral detail.
// Routing through the sink keeps the pinned block consistent even for lines emitted via a
// .With()-derived logger whose renderer clone shares the same sink.
func (h *ttyRenderer) scroll(line string) {
	h.sink.Log(line)
}

// styleText level-styles a single line (Info plain, Warn bold-yellow, Error red) when color is
// enabled — matching the legacy pretty logger. Applied per line so colors never bleed across a
// multi-line message.
func (h *ttyRenderer) styleText(level slog.Level, s string) string {
	if !h.color {
		return s
	}
	switch {
	case level >= slog.LevelError:
		return pterm.NewStyle(pterm.FgRed).Sprint(s)
	case level >= slog.LevelWarn:
		return pterm.NewStyle(pterm.FgYellow, pterm.Bold).Sprint(s)
	default:
		return s
	}
}

func (h *ttyRenderer) styleTitle(name string) string {
	if h.color {
		return pterm.NewStyle(pterm.Bold).Sprint(name)
	}
	return name
}

func (h *ttyRenderer) dim(s string) string {
	if h.color {
		return pterm.NewStyle(pterm.FgGray).Sprint(s)
	}
	return s
}

// WithAttrs / WithGroup intentionally drop the attrs/group: the terminal view is curated text
// (message + the marker attrs read off the record in Handle), not a structured dump, so persisted
// With-attributes are the file (JSON) sink's concern. The clone shares mu, sink and bar by
// value-copy of the interface/pointer values, so a .With()-derived logger keeps rendering into the
// same pinned block under the same lock.
func (h *ttyRenderer) WithAttrs(_ []slog.Attr) slog.Handler {
	clone := *h
	return &clone
}

func (h *ttyRenderer) WithGroup(_ string) slog.Handler {
	clone := *h
	return &clone
}
