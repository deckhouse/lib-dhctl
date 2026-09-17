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
	"strings"
	"testing"
	"time"

	"github.com/pterm/pterm"
)

func baseFrame() frame {
	return frame{
		title:      "Install Deckhouse",
		frac:       0.47,
		elapsed:    3 * time.Second,
		action:     "Waiting for readiness",
		spinner:    '⠹',
		milestones: []string{"SUCCESS Common preflight checks"},
		warns:      nil,
		logbox:     []string{"creating mc", "module ran"},
		width:      80,
		lay:        layout{action: true, region: true, logbox: 5, milestones: 5},
		color:      false,
	}
}

func TestRenderFrameBarLine(t *testing.T) {
	lines := renderFrame(baseFrame())
	if len(lines) == 0 {
		t.Fatal("no lines")
	}
	bar := lines[0]
	for _, want := range []string{"Install Deckhouse", "47%", "[047/100]", "3s"} {
		if !strings.Contains(bar, want) {
			t.Fatalf("bar %q missing %q", bar, want)
		}
	}
}

func TestRenderFrameActionHasSpinner(t *testing.T) {
	lines := renderFrame(baseFrame())
	if !strings.Contains(lines[1], "⠹") || !strings.Contains(lines[1], "Current action: Waiting for readiness") {
		t.Fatalf("action line wrong: %q", lines[1])
	}
}

func TestRenderFrameBarOnlyLevel(t *testing.T) {
	f := baseFrame()
	f.lay = layout{}
	if got := renderFrame(f); len(got) != 1 {
		t.Fatalf("barOnly must render 1 line, got %d: %v", len(got), got)
	}
}

func TestRenderFrameBarActionLevelDropsMilestonesAndLogbox(t *testing.T) {
	f := baseFrame()
	f.lay = layout{action: true}
	got := renderFrame(f)
	if len(got) != 2 {
		t.Fatalf("barAction must render bar+action, got %d: %v", len(got), got)
	}
}

func TestRenderFrameNoWrap(t *testing.T) {
	f := baseFrame()
	f.width = 20
	f.title = strings.Repeat("x", 200)
	for _, ln := range renderFrame(f) {
		if len([]rune(ln)) > f.width-1 {
			t.Fatalf("line exceeds width-1: %q", ln)
		}
	}
}

func TestRenderFrameBannerOnTop(t *testing.T) {
	f := baseFrame()
	f.banner = []string{"=== logo ===", "ascii"}
	lines := renderFrame(f)
	if lines[0] != "=== logo ===" || lines[1] != "ascii" {
		t.Fatalf("banner must be the top lines, got %v", lines[:2])
	}
	if !strings.Contains(lines[2], "Install Deckhouse") {
		t.Fatalf("bar must follow the banner: %q", lines[2])
	}
}

func TestFormatMilestoneConnIsCyanNoBadge(t *testing.T) {
	got := formatMilestone(false, "CONN", "ssh user@host")
	if got != "ssh user@host" {
		t.Fatalf("CONN milestone (no color) must be plain text, got %q", got)
	}
}

func TestRenderFrameColorWidthSafe(t *testing.T) {
	// A colored, over-long warn line must end up <= width-1 VISIBLE columns.
	f := baseFrame()
	f.color = true
	f.warns = []string{"\x1b[33mwarn line that is quite long indeed and should be cut\x1b[0m"}
	f.width = 30
	for _, ln := range renderFrame(f) {
		if vis := visLen(ln); vis > f.width-1 {
			t.Fatalf("visible width %d exceeds %d: %q", vis, f.width-1, ln)
		}
	}
	// The colored bar must keep its %% readout at a normal width.
	f2 := baseFrame()
	f2.color = true
	f2.width = 80
	bar := renderFrame(f2)[0]
	if !strings.Contains(pterm.RemoveColorFromString(bar), "47%") {
		t.Fatalf("colored bar lost its %% readout: %q", pterm.RemoveColorFromString(bar))
	}
}

// TestFormatMilestoneBadgesAreAligned pins the column: every badge is padded to one width, so the
// colored blocks end together and the text after them starts together however long the status word
// is. Adding a status must not knock the others out of line.
func TestFormatMilestoneBadgesAreAligned(t *testing.T) {
	const text = "some milestone"

	var width int
	for _, status := range []string{"SUCCESS", "WARNING", "FAILED", "DEPRECATED"} {
		line := formatMilestone(false, status, text)

		at := strings.Index(line, text)
		if at < 0 {
			t.Fatalf("%s: text missing: %q", status, line)
		}
		if width == 0 {
			width = at
		}
		if at != width {
			t.Fatalf("%s: text starts at column %d, expected %d: %q", status, at, width, line)
		}
		if !strings.Contains(line, status) {
			t.Fatalf("%s: badge word missing: %q", status, line)
		}
	}
}

// TestFormatMilestoneDeprecated pins the DEPRECATED badge itself: yellow like WARNING, and degrading
// to the plain padded word when color is off.
func TestFormatMilestoneDeprecated(t *testing.T) {
	plain := formatMilestone(false, "DEPRECATED", "ClusterConfiguration: kubernetesVersion")
	if plain != " DEPRECATED  ClusterConfiguration: kubernetesVersion" {
		t.Fatalf("uncolored badge wrong: %q", plain)
	}

	colored := formatMilestone(true, "DEPRECATED", "x")
	if colored == plain {
		t.Fatalf("colored badge not styled: %q", colored)
	}
	if colored != strings.Replace(formatMilestone(true, "WARNING", "x"), "  WARNING   ", " DEPRECATED ", 1) {
		t.Fatalf("DEPRECATED must be styled exactly like WARNING: %q", colored)
	}
}

// TestFormatMilestoneUnknownStatusRendersAsSuccess keeps the long-standing fallback: an unrecognised
// status must not paint an unstyled word of arbitrary width into the aligned column.
func TestFormatMilestoneUnknownStatusRendersAsSuccess(t *testing.T) {
	if got, want := formatMilestone(false, "WHATEVER", "x"), formatMilestone(false, "SUCCESS", "x"); got != want {
		t.Fatalf("unknown status rendered as %q, expected %q", got, want)
	}
}
