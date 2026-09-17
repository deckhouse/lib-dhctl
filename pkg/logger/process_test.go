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
	"errors"
	"log/slog"
	"strings"
	"testing"
)

// capture is a tiny handler that records every Handle call.
type capture struct{ records []slog.Record }

func (c *capture) Enabled(context.Context, slog.Level) bool { return true }
func (c *capture) Handle(_ context.Context, r slog.Record) error {
	c.records = append(c.records, r)
	return nil
}
func (c *capture) WithAttrs([]slog.Attr) slog.Handler { return c }
func (c *capture) WithGroup(string) slog.Handler      { return c }

func eventsOf(c *capture) []string {
	var out []string
	for _, r := range c.records {
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == attrKeyProcessEvent {
				out = append(out, a.Value.String())
			}
			return true
		})
	}
	return out
}

func TestRunProcessSuccessEmitsStartEnd(t *testing.T) {
	c := &capture{}
	l := slog.New(c)
	err := RunProcess(context.Background(), l, "deploy", func(context.Context) error { return nil })
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	got := eventsOf(c)
	if len(got) != 2 || got[0] != "start" || got[1] != "end" {
		t.Fatalf("events = %v, want [start end]", got)
	}
}

func TestRunProcessFailureEmitsStartFailAndReturnsErr(t *testing.T) {
	c := &capture{}
	l := slog.New(c)
	want := errors.New("boom")
	err := RunProcess(context.Background(), l, "deploy", func(context.Context) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	got := eventsOf(c)
	if len(got) != 2 || got[0] != "start" || got[1] != "fail" {
		t.Fatalf("events = %v, want [start fail]", got)
	}
}

func TestRunProcessMarkersAreRendererMarkers(t *testing.T) {
	c := &capture{}
	l := slog.New(c)
	_ = RunProcess(context.Background(), l, "deploy", func(context.Context) error { return nil })
	for _, r := range c.records {
		if !isRendererMarker(r) {
			t.Fatalf("process record %q is not a renderer marker", r.Message)
		}
	}
}

// TestStreamLoggerKeepsProcessBoxIntact is the end-to-end guard for the torn frame, exercising the
// real plain sink the commander stream and every non-TTY run use: every line a process block emits
// between its ┌ and └ borders must carry the │ prefix, whatever level it was logged at.
func TestStreamLoggerKeepsProcessBoxIntact(t *testing.T) {
	var buf bytes.Buffer
	l := NewStreamLogger(&buf)
	ctx := context.Background()

	err := RunProcess(ctx, l, "Resources failed to become ready", func(ctx context.Context) error {
		l.InfoContext(ctx, "detail line")
		l.WarnContext(ctx, "warning line\nsecond warning line")
		l.ErrorContext(ctx, "error line")
		return nil
	}, WithoutTiming())
	if err != nil {
		t.Fatalf("RunProcess: %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")

	closeAt := -1
	for i, line := range lines {
		if strings.HasPrefix(line, boxClose+" ") {
			closeAt = i
			break
		}
	}
	if closeAt == -1 {
		t.Fatalf("block never closed:\n%s", buf.String())
	}
	if !strings.HasPrefix(lines[0], boxOpen+" ") {
		t.Fatalf("block never opened: %q", lines[0])
	}
	if strings.Contains(lines[closeAt], "seconds") {
		t.Fatalf("WithoutTiming block still reports a duration: %q", lines[closeAt])
	}

	for _, line := range lines[1:closeAt] {
		if !strings.HasPrefix(line, boxBody) {
			t.Fatalf("line escaped the box: %q\nfull output:\n%s", line, buf.String())
		}
	}

	for _, want := range []string{"detail line", "warning line", "second warning line", "error line"} {
		if !strings.Contains(buf.String(), boxBody+want) {
			t.Fatalf("%q missing or unprefixed:\n%s", want, buf.String())
		}
	}
}
