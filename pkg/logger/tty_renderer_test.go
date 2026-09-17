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
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/deckhouse/lib-dhctl/pkg/logger/termui"
)

// fakeProgressUI records every method call for assertions.
type fakeProgressUI struct {
	calls       []string
	startName   string
	lastFrac    float64
	lastTitle   string
	writtenLine string
}

func (f *fakeProgressUI) Start(name string) {
	f.calls = append(f.calls, "Start")
	f.startName = name
}
func (f *fakeProgressUI) SetProgress(frac float64, title string) {
	f.calls = append(f.calls, "SetProgress")
	f.lastFrac = frac
	f.lastTitle = title
}
func (f *fakeProgressUI) SetAction(string) { f.calls = append(f.calls, "SetAction") }
func (f *fakeProgressUI) Milestone(prefix, status, text string) {
	f.calls = append(f.calls, "Milestone")
	f.writtenLine += prefix + status + " " + text + "\n"
}

// Warn and Log both feed writtenLine (accumulated, so multi-line renders such as nested boxes are
// fully captured) so assertions can match on the rendered text regardless of which routed it.
// Warn receives a whole record, which may be multi-line; like plainSink it prefixes every line.
func (f *fakeProgressUI) Warn(prefix, record string) {
	f.calls = append(f.calls, "Warn")
	for _, line := range strings.Split(record, "\n") {
		f.writtenLine += prefix + line + "\n"
	}
}
func (f *fakeProgressUI) Log(line string) {
	f.calls = append(f.calls, "Log")
	f.writtenLine += line + "\n"
}
func (f *fakeProgressUI) Finish() { f.calls = append(f.calls, "Finish") }
func (f *fakeProgressUI) Pause()  { f.calls = append(f.calls, "Pause") }
func (f *fakeProgressUI) Resume() { f.calls = append(f.calls, "Resume") }
func (f *fakeProgressUI) SetBanner(lines []string) {
	f.calls = append(f.calls, "SetBanner")
	for _, l := range lines {
		f.writtenLine += l + "\n"
	}
}
func (f *fakeProgressUI) SetConnString(s string) {
	f.calls = append(f.calls, "SetConnString")
	f.writtenLine += s + "\n"
}

func (f *fakeProgressUI) has(call string) bool {
	for _, c := range f.calls {
		if c == call {
			return true
		}
	}
	return false
}

func newRecord(level slog.Level, msg string, attrs ...slog.Attr) slog.Record {
	r := slog.NewRecord(time.Now(), level, msg, 0)
	r.AddAttrs(attrs...)
	return r
}

func procStartRec(name string) slog.Record {
	return newRecord(slog.LevelInfo, "Starting: "+name,
		slog.String(attrKeyProcessEvent, string(processStart)),
		slog.String(attrKeyProcessName, name))
}

func procEndRec(name string) slog.Record {
	return newRecord(slog.LevelInfo, "Finished: "+name,
		slog.String(attrKeyProcessEvent, string(processEnd)),
		slog.String(attrKeyProcessName, name))
}

func procFailRec(name string) slog.Record {
	return newRecord(slog.LevelError, "Failed: "+name,
		slog.String(attrKeyProcessEvent, string(processFail)),
		slog.String(attrKeyProcessName, name))
}

// --- progress bar lifecycle ---

func TestRendererStartOpensBar(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug})
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "p",
		slog.String(attrKeyProgressEvent, string(progressStart)),
		slog.String(attrKeyProgressName, "phase")))
	if !ui.has("Start") || ui.startName != "phase" {
		t.Fatalf("Start not handled: calls=%v", ui.calls)
	}
}

func TestRendererProgressValueAdvancesBar(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug})
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "x",
		slog.Float64(attrKeyProgressValue, 0.5), slog.String(attrKeyProgressTitle, "t")))
	if !ui.has("SetProgress") || ui.lastFrac != 0.5 || ui.lastTitle != "t" {
		t.Fatalf("SetProgress not handled: %v frac=%v title=%q", ui.calls, ui.lastFrac, ui.lastTitle)
	}
}

func TestRendererEndClosesBar(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug})
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "x",
		slog.String(attrKeyProgressEvent, string(progressEnd))))
	if !ui.has("Finish") {
		t.Fatalf("end not handled: %v", ui.calls)
	}
}

// --- process boxes ---
// All rendered lines (boxes + ordinary detail text) go through ui.Log, so the pinned progress
// block stays consistent regardless of which (possibly .With()-derived) logger emitted them.

func TestRendererProcessOpensBox(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug})
	_ = rdr.Handle(context.Background(), procStartRec("deploy"))
	if !strings.Contains(ui.writtenLine, boxOpen+" deploy") {
		t.Fatalf("box-open not rendered: %q", ui.writtenLine)
	}
	if len(rdr.stack) != 1 {
		t.Fatalf("process not pushed: %d", len(rdr.stack))
	}
}

func TestRendererProcessClosesBoxWithDuration(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug})
	_ = rdr.Handle(context.Background(), procStartRec("deploy"))
	_ = rdr.Handle(context.Background(), procEndRec("deploy"))
	s := ui.writtenLine
	if !strings.Contains(s, boxClose+" deploy") || !strings.Contains(s, "seconds)") {
		t.Fatalf("box-close/duration not rendered: %q", s)
	}
	if len(rdr.stack) != 0 {
		t.Fatalf("process not popped: %d", len(rdr.stack))
	}
}

// A failed process must emit a persistent FAILED milestone in addition to the ephemeral box-close
// line: in the live Block backend the framed box goes through the logbox ring (wiped on alt-screen
// teardown), so without the milestone the failure would vanish from the compact summary.
// TestRendererProcessFailEmitsPersistentMilestone covers a backend whose detail is ephemeral (the
// live block wipes its log box on teardown): there the FAILED border alone would be lost, so the
// failure is restated as a milestone that survives.
func TestRendererProcessFailEmitsPersistentMilestone(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{
		out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug, ephemeralDetail: true,
	})
	_ = rdr.Handle(context.Background(), procStartRec("(pre-infra)"))
	_ = rdr.Handle(context.Background(), procFailRec("(pre-infra)"))
	if !ui.has("Milestone") {
		t.Fatalf("process fail must emit a Milestone (persists past compact teardown): calls=%v", ui.calls)
	}
	if !strings.Contains(ui.writtenLine, "FAILED (pre-infra)") {
		t.Fatalf("FAILED milestone for the process not rendered: %q", ui.writtenLine)
	}
	// The ephemeral framed close line is still emitted for the live view.
	if !strings.Contains(ui.writtenLine, boxClose+" (pre-infra)") {
		t.Fatalf("box-close line still expected: %q", ui.writtenLine)
	}
}

// TestRendererProcessFailSkipsMilestoneWhenDetailPersists is the other half: a backend that writes
// its lines out permanently already carries the FAILED border, so restating it added nothing but a
// duplicate - and the milestone, printed unprefixed in the middle of the block, tore the frame.
func TestRendererProcessFailSkipsMilestoneWhenDetailPersists(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug})

	ctx := context.Background()
	_ = rdr.Handle(ctx, procStartRec("outer"))
	_ = rdr.Handle(ctx, procStartRec("inner"))
	_ = rdr.Handle(ctx, procFailRec("inner"))
	_ = rdr.Handle(ctx, procFailRec("outer"))

	if ui.has("Milestone") {
		t.Fatalf("no milestone expected when detail persists: calls=%v", ui.calls)
	}

	want := "" +
		boxOpen + " outer\n" +
		boxBody + boxOpen + " inner\n" +
		boxBody + boxClose + " inner FAILED (0.00 seconds)\n" +
		boxClose + " outer FAILED (0.00 seconds)\n"
	if ui.writtenLine != want {
		t.Fatalf("frame broken:\ngot:\n%s\nwant:\n%s", ui.writtenLine, want)
	}
}

func TestRendererNestedProcessesIndent(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug})
	_ = rdr.Handle(context.Background(), procStartRec("outer"))
	_ = rdr.Handle(context.Background(), procStartRec("inner"))
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "body"))
	s := ui.writtenLine
	if !strings.Contains(s, boxBody+boxOpen+" inner") {
		t.Fatalf("nested box not indented: %q", s)
	}
	if !strings.Contains(s, boxBody+boxBody+"body") {
		t.Fatalf("nested body not indented two levels: %q", s)
	}
}

// --- ordinary lines ---

func TestRendererMultiLineMessageIndentsEveryLine(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug})
	_ = rdr.Handle(context.Background(), procStartRec("ng")) // depth 1 → prefix "│ "
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "NAME\nmaster\nworker"))
	for _, want := range []string{boxBody + "NAME", boxBody + "master", boxBody + "worker"} {
		if !strings.Contains(ui.writtenLine, want+"\n") {
			t.Fatalf("multi-line %q not indented per line: %q", want, ui.writtenLine)
		}
	}
}

// TestRendererWarnInsideProcessCarriesBoxPrefix pins the fix for the torn frame: a Warn/Error
// line logged from inside a process block used to reach the sink with no box prefix, so a backend
// that prints lines inline put it flush against the left margin and split the ┌ … └ block in two.
// The prefix now travels with the line; it is the sink that decides whether to apply it.
func TestRendererWarnInsideProcessCarriesBoxPrefix(t *testing.T) {
	for _, level := range []slog.Level{slog.LevelWarn, slog.LevelError} {
		ui := &fakeProgressUI{}
		rdr := newTTYRenderer(rendererConfig{out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug})
		_ = rdr.Handle(context.Background(), procStartRec("outer")) // depth 1 → prefix "│ "
		_ = rdr.Handle(context.Background(), procStartRec("inner")) // depth 2 → prefix "│ │ "
		_ = rdr.Handle(context.Background(), newRecord(level, "first\nsecond"))

		for _, want := range []string{boxBody + boxBody + "first", boxBody + boxBody + "second"} {
			if !strings.Contains(ui.writtenLine, want+"\n") {
				t.Fatalf("level %v: %q escaped the box: %q", level, want, ui.writtenLine)
			}
		}
	}
}

// TestRendererWarnAtTopLevelHasNoPrefix guards the other direction: outside any process block
// there is no frame to line up with, so the line must not be indented.
func TestRendererWarnAtTopLevelHasNoPrefix(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug})
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelWarn, "standalone"))
	if ui.writtenLine != "standalone\n" {
		t.Fatalf("top-level warn was decorated: %q", ui.writtenLine)
	}
}

// TestRendererUntimedProcessOmitsDuration covers WithoutTiming: a block that only frames a report
// closes without the "(N seconds)" tail, which next to such a heading reads as a broken timer.
func TestRendererUntimedProcessOmitsDuration(t *testing.T) {
	tests := map[string]func(string) slog.Record{
		"end":  procEndRec,
		"fail": procFailRec,
	}

	for name, closeRec := range tests {
		t.Run(name, func(t *testing.T) {
			ui := &fakeProgressUI{}
			rdr := newTTYRenderer(rendererConfig{out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug})

			start := procStartRec("report")
			start.AddAttrs(slog.Bool(attrKeyProcessUntimed, true))
			_ = rdr.Handle(context.Background(), start)
			_ = rdr.Handle(context.Background(), closeRec("report"))

			if strings.Contains(ui.writtenLine, "seconds") {
				t.Fatalf("untimed block still reports a duration: %q", ui.writtenLine)
			}
			if !strings.Contains(ui.writtenLine, boxClose+" report") {
				t.Fatalf("block was not closed: %q", ui.writtenLine)
			}
			if name == "fail" && !strings.Contains(ui.writtenLine, "FAILED") {
				t.Fatalf("failed block lost its FAILED marker: %q", ui.writtenLine)
			}
		})
	}
}

// TestRendererTimedProcessKeepsDuration is the control for the test above: without the option the
// duration is still reported.
func TestRendererTimedProcessKeepsDuration(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug})
	_ = rdr.Handle(context.Background(), procStartRec("work"))
	_ = rdr.Handle(context.Background(), procEndRec("work"))
	if !strings.Contains(ui.writtenLine, "seconds") {
		t.Fatalf("timed block lost its duration: %q", ui.writtenLine)
	}
}

func TestRendererOrdinaryLineGoesThroughUI(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug})
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "plainmsg"))
	if !ui.has("Log") || !strings.Contains(ui.writtenLine, "plainmsg") {
		t.Fatalf("line not routed through ui: %q calls=%v", ui.writtenLine, ui.calls)
	}
}

func TestRendererLineScrollsAboveActiveBar(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug})
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "hello"))
	if !ui.has("Log") || !strings.Contains(ui.writtenLine, "hello") {
		t.Fatalf("line not scrolled above bar: %q calls=%v", ui.writtenLine, ui.calls)
	}
}

func TestRendererBoxScrollsAboveActiveBar(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug})
	_ = rdr.Handle(context.Background(), procStartRec("deploy"))
	if !ui.has("Log") || !strings.Contains(ui.writtenLine, boxOpen+" deploy") {
		t.Fatalf("box not scrolled above bar: %q", ui.writtenLine)
	}
}

func TestRendererPauseResume(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug})
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "x",
		slog.String(attrKeyProgressEvent, string(progressPause))))
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "x",
		slog.String(attrKeyProgressEvent, string(progressResume))))
	if !ui.has("Pause") || !ui.has("Resume") {
		t.Fatalf("pause/resume not handled: %v", ui.calls)
	}
}

func TestRendererRespectsLevel(t *testing.T) {
	rdr := newTTYRenderer(rendererConfig{out: &bytes.Buffer{}, sink: &fakeProgressUI{}, bar: &fakeProgressUI{}, level: slog.LevelInfo})
	if rdr.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("debug should be disabled at info level")
	}
	if !rdr.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("info should be enabled")
	}
}

// TestRendererConcurrentHandleIsSerialized exercises the real-world wiring: log lines come from the
// operation goroutine while the progress bar is advanced from consumeProgress's goroutine. Both go
// through the same renderer. Run under -race; without the renderer's mutex this trips the detector
// on the shared ui/stack and, in production, corrupts the pinned block (blink, eaten messages).
func TestRendererConcurrentHandleIsSerialized(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug})
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "start",
		slog.String(attrKeyProgressEvent, string(progressStart)), slog.String(attrKeyProgressName, "p")))

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "line",
				ShowInCompacted()))
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "prog",
				slog.Float64(attrKeyProgressValue, 0.5), slog.String(attrKeyProgressTitle, "t")))
		}
	}()
	wg.Wait()
}

func TestRendererErrorStyledWithColor(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug, color: true})
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelError, "boom"))
	s := ui.writtenLine
	if !strings.Contains(s, "boom") || !strings.Contains(s, "\x1b[") {
		t.Fatalf("error not ANSI-styled: %q", s)
	}
}

// --- repeated lines ---

// fakeClock lets the repeat tests step over repeatFlushInterval without sleeping.
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time      { return c.t }
func (c *fakeClock) add(d time.Duration) { c.t = c.t.Add(d) }

func newRepeatRenderer(ui *fakeProgressUI, clk *fakeClock) *ttyRenderer {
	rdr := newTTYRenderer(rendererConfig{out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug})
	rdr.now = clk.now
	return rdr
}

// TestRendererCollapsesRepeatedLines covers the poll-loop spam: a wait that re-logs the same status
// every second used to print one identical line per attempt. The run is now collapsed and reported
// with its count when the output moves on.
func TestRendererCollapsesRepeatedLines(t *testing.T) {
	ui := &fakeProgressUI{}
	clk := &fakeClock{t: time.Unix(0, 0)}
	rdr := newRepeatRenderer(ui, clk)

	for i := 0; i < 5; i++ {
		_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "pod is Pending"))
		clk.add(time.Second)
	}
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "pod is Running"))

	want := "pod is Pending\npod is Pending [repeated 4 more times]\npod is Running\n"
	if ui.writtenLine != want {
		t.Fatalf("repeats not collapsed:\ngot  %q\nwant %q", ui.writtenLine, want)
	}
}

// TestRendererReportsLongRunningRepeats guards liveness: a status that repeats for longer than
// repeatFlushInterval must still print, or a slow wait is indistinguishable from a hung one.
func TestRendererReportsLongRunningRepeats(t *testing.T) {
	ui := &fakeProgressUI{}
	clk := &fakeClock{t: time.Unix(0, 0)}
	rdr := newRepeatRenderer(ui, clk)

	// One initial line, then two full flush intervals of identical ones.
	for i := 0; i < 3; i++ {
		_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "still waiting"))
		clk.add(repeatFlushInterval)
	}

	if n := strings.Count(ui.writtenLine, "repeated"); n != 2 {
		t.Fatalf("long run must report progress once per interval, got %d:\n%s", n, ui.writtenLine)
	}
}

// TestRendererDoesNotCollapseAcrossDifferentLines is the control: distinct lines are never merged,
// and a line that repeats only once is not annotated at all.
func TestRendererDoesNotCollapseAcrossDifferentLines(t *testing.T) {
	ui := &fakeProgressUI{}
	clk := &fakeClock{t: time.Unix(0, 0)}
	rdr := newRepeatRenderer(ui, clk)

	for _, msg := range []string{"a", "b", "c"} {
		_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, msg))
	}
	if ui.writtenLine != "a\nb\nc\n" {
		t.Fatalf("distinct lines altered: %q", ui.writtenLine)
	}
}

// TestRendererFlushesRepeatsBeforeStructure pins the ordering: the count belongs to the lines above
// it, so it must be printed before a process border or milestone opens the next section.
func TestRendererFlushesRepeatsBeforeStructure(t *testing.T) {
	ui := &fakeProgressUI{}
	clk := &fakeClock{t: time.Unix(0, 0)}
	rdr := newRepeatRenderer(ui, clk)

	_ = rdr.Handle(context.Background(), procStartRec("wait"))
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "tick"))
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "tick"))
	_ = rdr.Handle(context.Background(), procEndRec("wait"))

	lines := strings.Split(strings.TrimRight(ui.writtenLine, "\n"), "\n")
	if len(lines) < 4 {
		t.Fatalf("unexpected output: %q", ui.writtenLine)
	}
	if !strings.Contains(lines[2], "repeated") {
		t.Fatalf("count not flushed before the block closed: %q", ui.writtenLine)
	}
	if !strings.HasPrefix(lines[3], boxClose) {
		t.Fatalf("block border out of order: %q", ui.writtenLine)
	}
}

// TestRendererRepeatRunIsBrokenByDepth guards against merging identical text logged at different
// nesting depths, which would attach the count to the wrong block.
func TestRendererRepeatRunIsBrokenByDepth(t *testing.T) {
	ui := &fakeProgressUI{}
	clk := &fakeClock{t: time.Unix(0, 0)}
	rdr := newRepeatRenderer(ui, clk)

	_ = rdr.Handle(context.Background(), procStartRec("outer"))
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "same"))
	_ = rdr.Handle(context.Background(), procStartRec("inner"))
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo, "same"))

	if strings.Contains(ui.writtenLine, "repeated") {
		t.Fatalf("lines at different depths were merged: %q", ui.writtenLine)
	}
	if !strings.Contains(ui.writtenLine, boxBody+"same\n") || !strings.Contains(ui.writtenLine, boxBody+boxBody+"same\n") {
		t.Fatalf("both depths must print: %q", ui.writtenLine)
	}
}

// --- block separators ---

// TestRendererSeparatorDroppedBeforeEnclosingClose pins the fix for the stray empty row. A closed
// block used to be trailed unconditionally by a line carrying the enclosing guides; when the parent
// closed right after, that row separated the block from its own parent's border and read as a
// rendering artifact rather than as spacing.
func TestRendererSeparatorDroppedBeforeEnclosingClose(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug})
	ctx := context.Background()

	_ = rdr.Handle(ctx, procStartRec("outer"))
	_ = rdr.Handle(ctx, procStartRec("inner"))
	_ = rdr.Handle(ctx, newRecord(slog.LevelInfo, "a"))
	_ = rdr.Handle(ctx, procEndRec("inner"))
	_ = rdr.Handle(ctx, procEndRec("outer")) // parent closes right after

	want := "" +
		boxOpen + " outer\n" +
		boxBody + boxOpen + " inner\n" +
		boxBody + boxBody + "a\n" +
		boxBody + boxClose + " inner (0.00 seconds)\n" +
		boxClose + " outer (0.00 seconds)\n"

	if ui.writtenLine != want {
		t.Fatalf("stray separator before the enclosing border:\ngot:\n%s\nwant:\n%s", ui.writtenLine, want)
	}
}

// TestRendererSiblingBlocksAreSeparated is the other half of the rule: when a sibling block follows,
// the separator is what keeps the two boxes visually apart, so it stays. Inside a block it carries
// the enclosing guides rather than being blank, so the frame is not broken.
func TestRendererSiblingBlocksAreSeparated(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug})
	ctx := context.Background()

	_ = rdr.Handle(ctx, procStartRec("outer"))
	_ = rdr.Handle(ctx, procStartRec("not ready"))
	_ = rdr.Handle(ctx, procEndRec("not ready"))
	_ = rdr.Handle(ctx, procStartRec("ready")) // sibling right after
	_ = rdr.Handle(ctx, procEndRec("ready"))
	_ = rdr.Handle(ctx, procEndRec("outer"))

	want := "" +
		boxOpen + " outer\n" +
		boxBody + boxOpen + " not ready\n" +
		boxBody + boxClose + " not ready (0.00 seconds)\n" +
		strings.TrimRight(boxBody, " ") + "\n" + // separator between the two siblings
		boxBody + boxOpen + " ready\n" +
		boxBody + boxClose + " ready (0.00 seconds)\n" +
		boxClose + " outer (0.00 seconds)\n"

	if ui.writtenLine != want {
		t.Fatalf("sibling blocks not separated:\ngot:\n%q\nwant:\n%q", ui.writtenLine, want)
	}
}

// TestRendererTopLevelBlocksAreSeparated is the other half of the rule: between top-level blocks a
// blank line is real whitespace between major sections and is kept - but only when something
// follows, so the last block of a run is not trailed by a stray blank.
func TestRendererTopLevelBlocksAreSeparated(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug})
	ctx := context.Background()

	_ = rdr.Handle(ctx, procStartRec("first"))
	_ = rdr.Handle(ctx, procEndRec("first"))
	_ = rdr.Handle(ctx, procStartRec("second"))
	_ = rdr.Handle(ctx, procEndRec("second"))

	want := "" +
		boxOpen + " first\n" +
		boxClose + " first (0.00 seconds)\n" +
		"\n" +
		boxOpen + " second\n" +
		boxClose + " second (0.00 seconds)\n"

	if ui.writtenLine != want {
		t.Fatalf("top-level separation wrong:\ngot:\n%q\nwant:\n%q", ui.writtenLine, want)
	}
}

// TestRendererDeprecatedBadgeIsItsOwnMilestone covers the DEPRECATED badge: like SUCCESS and FAILED
// it renders as a curated milestone rather than as a detail line, so the backend can keep it on
// screen after the live block is torn down.
func TestRendererDeprecatedBadgeIsItsOwnMilestone(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug})

	_ = rdr.Handle(context.Background(), procStartRec("parse")) // even inside a block
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo,
		`ModuleConfig "user-authn": publishAPI`, BadgeDeprecated()))

	if !ui.has("Milestone") {
		t.Fatalf("deprecation did not route as a milestone: calls=%v", ui.calls)
	}
	if !strings.Contains(ui.writtenLine, `DEPRECATED ModuleConfig "user-authn": publishAPI`) {
		t.Fatalf("milestone text wrong: %q", ui.writtenLine)
	}
}

// TestRendererMilestoneInsideProcessCarriesBoxPrefix is the Milestone counterpart to
// TestRendererWarnInsideProcessCarriesBoxPrefix: a curated line emitted from inside a block - a
// DEPRECATED badge found while the configuration is parsed, say - reaches the sink with the prefix,
// so a backend that prints it inline can keep the frame intact.
func TestRendererMilestoneInsideProcessCarriesBoxPrefix(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug})

	_ = rdr.Handle(context.Background(), procStartRec("parse"))
	_ = rdr.Handle(context.Background(), newRecord(slog.LevelInfo,
		"ClusterConfiguration: kubernetesVersion", BadgeDeprecated()))

	if !strings.Contains(ui.writtenLine, boxBody+"DEPRECATED ClusterConfiguration: kubernetesVersion") {
		t.Fatalf("milestone escaped the box: %q", ui.writtenLine)
	}
}

// --- milestone collapsing ---
// The backstop under commitOrDefer: an attempt a retry loop absorbed is dropped outright, but a
// step that genuinely failed more than once still repeats, and a pinned row costs a row of the
// live region for the whole run plus a row of the closing summary.

func TestRendererCollapsesRepeatedFailedMilestones(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{
		out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug, ephemeralDetail: true,
	})

	ctx := context.Background()
	for range 40 {
		_ = rdr.Handle(ctx, procStartRec("Control plane readiness"))
		_ = rdr.Handle(ctx, procFailRec("Control plane readiness"))
	}
	// A different milestone ends the run and reports what it stood for.
	_ = rdr.Handle(ctx, newRecord(slog.LevelInfo, "Process all nodes", BadgeSuccess()))

	if got := strings.Count(ui.writtenLine, "FAILED Control plane readiness"); got != 2 {
		t.Fatalf("want the milestone once plus one count line, got %d:\n%s", got, ui.writtenLine)
	}
	if !strings.Contains(ui.writtenLine, "[repeated 39 more times]") {
		t.Fatalf("collapsed run must report its count: %q", ui.writtenLine)
	}
	if !strings.Contains(ui.writtenLine, "Process all nodes") {
		t.Fatalf("the milestone that broke the run must still be printed: %q", ui.writtenLine)
	}
}

func TestRendererDistinctMilestonesAreNeverCollapsed(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug})

	ctx := context.Background()
	for _, m := range []string{"Check cluster configuration", "Base Infrastructure", "Process all nodes"} {
		_ = rdr.Handle(ctx, newRecord(slog.LevelInfo, m, BadgeSuccess()))
	}

	for _, m := range []string{"Check cluster configuration", "Base Infrastructure", "Process all nodes"} {
		if !strings.Contains(ui.writtenLine, m) {
			t.Fatalf("milestone %q lost: %q", m, ui.writtenLine)
		}
	}
	if strings.Contains(ui.writtenLine, "repeated") {
		t.Fatalf("nothing repeated, nothing to report: %q", ui.writtenLine)
	}
}

// A run still open when the bar closes must report its count before the closing summary is
// printed, or the count lands after the summary it belongs in - or is swallowed with the
// alternate screen.
func TestRendererFlushesCollapsedMilestonesBeforeFinish(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{
		out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug, ephemeralDetail: true,
	})

	ctx := context.Background()
	for range 3 {
		_ = rdr.Handle(ctx, procStartRec("Control plane readiness"))
		_ = rdr.Handle(ctx, procFailRec("Control plane readiness"))
	}
	_ = rdr.Handle(ctx, newRecord(slog.LevelInfo, "x",
		slog.String(attrKeyProgressEvent, string(progressEnd))))

	if !strings.Contains(ui.writtenLine, "[repeated 2 more times]") {
		t.Fatalf("pending run must be flushed at teardown: %q", ui.writtenLine)
	}
	countIdx := strings.Index(ui.writtenLine, "[repeated 2 more times]")
	finishIdx := -1
	for i, c := range ui.calls {
		if c == "Finish" {
			finishIdx = i
			break
		}
	}
	if countIdx < 0 || finishIdx < 0 {
		t.Fatalf("expected both the count and Finish: calls=%v", ui.calls)
	}
	if ui.calls[finishIdx-1] != "Milestone" {
		t.Fatalf("the count must be the last thing before Finish: calls=%v", ui.calls)
	}
}

// A run of exactly one repeat reads better as words than as "[repeated 1 more times]".
func TestRendererSingleRepeatedMilestoneReadsAsWords(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{
		out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug, ephemeralDetail: true,
	})

	ctx := context.Background()
	for range 2 {
		_ = rdr.Handle(ctx, procStartRec("Control plane readiness"))
		_ = rdr.Handle(ctx, procFailRec("Control plane readiness"))
	}
	_ = rdr.Handle(ctx, newRecord(slog.LevelInfo, "x",
		slog.String(attrKeyProgressEvent, string(progressEnd))))

	if !strings.Contains(ui.writtenLine, "[repeated once more]") {
		t.Fatalf("want the singular form: %q", ui.writtenLine)
	}
}

// --- transient failures inside a retry loop ---

// TestRendererDropsFailedAttemptsWhenTheEnclosingBlockSucceeds is the converge report: a readiness
// check polled for forty seconds, every attempt opened and failed a block, and every failure was
// restated as a persistent FAILED milestone - which survived into the closing summary of a run
// that then finished successfully. An attempt a retry loop absorbed is not an outcome.
func TestRendererDropsFailedAttemptsWhenTheEnclosingBlockSucceeds(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{
		out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug, ephemeralDetail: true,
	})

	ctx := context.Background()
	_ = rdr.Handle(ctx, procStartRec("Node master-0 readiness check"))
	for range 40 {
		_ = rdr.Handle(ctx, procStartRec("Control plane readiness"))
		_ = rdr.Handle(ctx, procFailRec("Control plane readiness"))
	}
	_ = rdr.Handle(ctx, procEndRec("Node master-0 readiness check"))

	if ui.has("Milestone") {
		t.Fatalf("a retried attempt must leave no milestone behind: calls=%v\n%s", ui.calls, ui.writtenLine)
	}
	// The framed boxes are still drawn: they are ephemeral detail, which is what live progress is for.
	if !strings.Contains(ui.writtenLine, "Control plane readiness") {
		t.Fatalf("the attempts must still show as live detail: %q", ui.writtenLine)
	}
}

// The other half: when the enclosing block fails too, the failure reaches the summary - as the
// one operation that was asked for and did not finish, not as a badge per level it travelled up.
func TestRendererKeepsFailedAttemptsWhenTheEnclosingBlockFails(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{
		out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug, ephemeralDetail: true,
	})

	ctx := context.Background()
	_ = rdr.Handle(ctx, procStartRec("Node master-0 readiness check"))
	_ = rdr.Handle(ctx, procStartRec("Control plane readiness"))
	_ = rdr.Handle(ctx, procFailRec("Control plane readiness"))
	_ = rdr.Handle(ctx, procFailRec("Node master-0 readiness check"))

	if !strings.Contains(ui.writtenLine, "FAILED Node master-0 readiness check") {
		t.Fatalf("the operation that failed must be reported: %q", ui.writtenLine)
	}
	if strings.Contains(ui.writtenLine, "FAILED Control plane readiness") {
		t.Fatalf("a step of that failure must not get a badge of its own: %q", ui.writtenLine)
	}
	// The ancestry is not lost, it just stays where it belongs: the framed detail.
	if !strings.Contains(ui.writtenLine, boxClose+" Control plane readiness FAILED") {
		t.Fatalf("the framed detail must still show how it failed: %q", ui.writtenLine)
	}
}

// The report from the screenshot: a converge that lost three masters, each through four levels of
// blocks. Twelve rows for one converge that did not converge - and the operator wants one.
func TestRendererReportsOneRowForOneFailedOperation(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{
		out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug, ephemeralDetail: true,
	})

	ctx := context.Background()
	_ = rdr.Handle(ctx, procStartRec("Process all nodes"))
	for _, node := range []string{"master-0", "master-1", "master-2"} {
		_ = rdr.Handle(ctx, procStartRec("Update Node "+node+" in NodeGroup master (replicas: 3)"))
		_ = rdr.Handle(ctx, procStartRec("Pipeline master-node for "+node))
		_ = rdr.Handle(ctx, procStartRec("infrastructure apply ..."))
		_ = rdr.Handle(ctx, procFailRec("infrastructure apply ..."))
		_ = rdr.Handle(ctx, procFailRec("Pipeline master-node for "+node))
		_ = rdr.Handle(ctx, procFailRec("Update Node "+node+" in NodeGroup master (replicas: 3)"))
	}
	_ = rdr.Handle(ctx, procFailRec("Process all nodes"))

	rows := 0
	for _, c := range ui.calls {
		if c == "Milestone" {
			rows++
		}
	}
	if rows != 1 {
		t.Fatalf("want one row for one failed operation, got %d:\n%s", rows, ui.writtenLine)
	}
	if !strings.Contains(ui.writtenLine, "FAILED Process all nodes") {
		t.Fatalf("the row must name the operation that was asked for: %q", ui.writtenLine)
	}
}

// Nothing below the top level reports itself, however deep the failure started or how many
// levels it passed through on the way up.
func TestRendererOnlyTheOutermostFailureIsReported(t *testing.T) {
	ctx := context.Background()

	run := func(outerFails bool) *fakeProgressUI {
		ui := &fakeProgressUI{}
		rdr := newTTYRenderer(rendererConfig{
			out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug, ephemeralDetail: true,
		})
		_ = rdr.Handle(ctx, procStartRec("converge"))
		_ = rdr.Handle(ctx, procStartRec("wait"))
		_ = rdr.Handle(ctx, procStartRec("attempt"))
		_ = rdr.Handle(ctx, procFailRec("attempt"))
		_ = rdr.Handle(ctx, procFailRec("wait")) // the wait gave up, but converge may still recover
		if outerFails {
			_ = rdr.Handle(ctx, procFailRec("converge"))
		} else {
			_ = rdr.Handle(ctx, procEndRec("converge"))
		}
		return ui
	}

	if ui := run(false); ui.has("Milestone") {
		t.Fatalf("outermost block succeeded: nothing below it is news: %q", ui.writtenLine)
	}
	ui := run(true)
	if !strings.Contains(ui.writtenLine, "FAILED converge") {
		t.Fatalf("the outermost failure must surface: %q", ui.writtenLine)
	}
	for _, unwanted := range []string{"FAILED attempt", "FAILED wait"} {
		if strings.Contains(ui.writtenLine, unwanted) {
			t.Fatalf("%q is a step of that failure, not news of its own: %q", unwanted, ui.writtenLine)
		}
	}
}

// A top-level failure has nothing above it that could absorb it, so it prints at once - the
// behaviour a failed bootstrap depends on.
func TestRendererTopLevelFailureIsReportedImmediately(t *testing.T) {
	ui := &fakeProgressUI{}
	rdr := newTTYRenderer(rendererConfig{
		out: &bytes.Buffer{}, sink: ui, bar: ui, level: slog.LevelDebug, ephemeralDetail: true,
	})

	ctx := context.Background()
	_ = rdr.Handle(ctx, procStartRec("(pre-infra)"))
	_ = rdr.Handle(ctx, procFailRec("(pre-infra)"))

	if !strings.Contains(ui.writtenLine, "FAILED (pre-infra)") {
		t.Fatalf("top-level failure must not be held: %q", ui.writtenLine)
	}
}

// TestLiveBlockSummaryOmitsAbsorbedRetries drives the real live block, not a fake sink: the
// renderer and termui.Block wired exactly as production wires them. It is the report from the
// screenshot end to end - a readiness check that failed on every attempt until it did not, inside
// a converge that finished - and what it asserts is the one thing the operator is left with once
// the alternate screen is gone.
func TestLiveBlockSummaryOmitsAbsorbedRetries(t *testing.T) {
	var buf bytes.Buffer
	bl := termui.New(&buf, termui.Options{Color: false})
	rdr := newTTYRenderer(rendererConfig{
		out: &buf, sink: bl, bar: bl, level: slog.LevelDebug, ephemeralDetail: true,
	})

	ctx := context.Background()
	_ = rdr.Handle(ctx, newRecord(slog.LevelInfo, "converge",
		slog.String(attrKeyProgressEvent, string(progressStart)),
		slog.String(attrKeyProgressName, "converge")))

	_ = rdr.Handle(ctx, procStartRec("Node master-0 readiness check"))
	for range 40 {
		_ = rdr.Handle(ctx, procStartRec("Control plane readiness"))
		_ = rdr.Handle(ctx, procFailRec("Control plane readiness"))
	}
	_ = rdr.Handle(ctx, procEndRec("Node master-0 readiness check"))
	_ = rdr.Handle(ctx, newRecord(slog.LevelInfo, "Process all nodes", BadgeSuccess()))

	_ = rdr.Handle(ctx, newRecord(slog.LevelInfo, "converge",
		slog.String(attrKeyProgressEvent, string(progressEnd))))

	out := buf.String()
	leave := strings.LastIndex(out, "\x1b[?1049l")
	if leave < 0 {
		t.Fatalf("the block never left the alternate screen: %q", out)
	}
	summary := out[leave:]

	if strings.Contains(summary, "FAILED") {
		t.Fatalf("a converge that finished must not sign off with FAILED:\n%s", summary)
	}
	if !strings.Contains(summary, "Process all nodes") {
		t.Fatalf("the summary lost what actually happened:\n%s", summary)
	}
}
