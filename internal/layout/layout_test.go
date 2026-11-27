package layout

import (
	"strings"
	"testing"

	"github.com/lakshwin/bento/internal/ui"
)

func TestModeBreakpoints(t *testing.T) {
	cases := []struct {
		w, h int
		want Mode
	}{
		{200, 50, Wide},
		{120, 30, Wide},
		{119, 40, Medium},
		{140, 29, Medium},
		{90, 24, Medium},
		{89, 30, Narrow},
		{120, 23, Narrow},
		{60, 15, Narrow},
	}
	for _, c := range cases {
		if got := ModeFor(c.w, c.h); got != c.want {
			t.Errorf("ModeFor(%d,%d) = %v, want %v", c.w, c.h, got, c.want)
		}
	}
}

func TestTooSmall(t *testing.T) {
	for _, sz := range [][2]int{{59, 40}, {100, 13}, {10, 5}} {
		g := Solve(sz[0], sz[1], State{ShowTree: true, ShowAgent: true})
		if !g.TooSmall {
			t.Errorf("%dx%d should be too small", sz[0], sz[1])
		}
	}
	g := Solve(60, MinContentHeight, State{ShowTree: true, ShowAgent: true, Active: ui.PaneViewer})
	if g.TooSmall {
		t.Error("minimum content size should not be too small")
	}
}

func TestWideFull(t *testing.T) {
	const w, h = 140, 40
	g := Solve(w, h, State{ShowTree: true, ShowAgent: true, Active: ui.PaneViewer})
	if g.Mode != Wide {
		t.Fatalf("mode = %v", g.Mode)
	}
	assertRectsValid(t, g, w, h)
	for _, id := range []ui.PaneID{ui.PaneTree, ui.PaneViewer, ui.PaneTerminal, ui.PaneTerminal2, ui.PaneChanges, ui.PaneAgent} {
		if _, ok := g.Rects[id]; !ok {
			t.Errorf("missing rect for %v", id)
		}
	}
	r := g.Rects
	if r[ui.PaneViewer].X+r[ui.PaneViewer].W >= r[ui.PaneAgent].X {
		t.Error("viewer and agent overlap or touch without a gutter")
	}
	if r[ui.PaneViewer].Y+r[ui.PaneViewer].H >= r[ui.PaneTerminal].Y {
		t.Error("viewer and terminal need a 1-row gutter")
	}
	if r[ui.PaneTree].X != r[ui.PaneChanges].X || r[ui.PaneTree].W != r[ui.PaneChanges].W {
		t.Error("tree and changes must share the right column")
	}
	if r[ui.PaneAgent].X+r[ui.PaneAgent].W >= r[ui.PaneTree].X {
		t.Error("agent and right column need a 1-column gutter")
	}
}

func TestWideNoTreeNoAgent(t *testing.T) {
	const w, h = 140, 40
	g := Solve(w, h, State{ShowTree: false, ShowAgent: false, Active: ui.PaneViewer})
	assertRectsValid(t, g, w, h)
	if _, ok := g.Rects[ui.PaneTree]; ok {
		t.Error("tree should be hidden")
	}
	if _, ok := g.Rects[ui.PaneAgent]; ok {
		t.Error("agent should be hidden")
	}
	if g.Rects[ui.PaneViewer].X != 0 {
		t.Errorf("viewer should start at x=0, got %d", g.Rects[ui.PaneViewer].X)
	}
	if r := g.Rects[ui.PaneChanges]; r.H >= h {
		t.Errorf("changes should remain in the top grid when tree is hidden, got %d", r.H)
	}
	if got := len(g.Visible); got != 4 {
		t.Errorf("visible panes = %d, want 4", got)
	}
}

func TestMedium(t *testing.T) {
	const w, h = 100, 25
	g := Solve(w, h, State{ShowTree: true, ShowAgent: true, Active: ui.PaneViewer})
	if g.Mode != Medium {
		t.Fatalf("mode = %v", g.Mode)
	}
	assertRectsValid(t, g, w, h)
	if _, ok := g.Rects[ui.PaneAgent]; ok {
		t.Error("agent must be hidden in medium")
	}
	if _, ok := g.Rects[ui.PaneChanges]; ok {
		t.Error("changes must be a tab, not a rect, in medium")
	}
	if _, ok := g.Rects[ui.PaneTerminal]; !ok {
		t.Error("terminal rect missing")
	}
	if _, ok := g.Rects[ui.PaneTerminal2]; !ok {
		t.Error("second terminal rect missing")
	}
}

func TestNarrow(t *testing.T) {
	const w, h = 80, 20
	for _, active := range []ui.PaneID{ui.PaneTree, ui.PaneViewer, ui.PaneTerminal, ui.PaneTerminal2, ui.PaneChanges} {
		g := Solve(w, h, State{ShowTree: true, ShowAgent: false, Active: active})
		if g.Mode != Narrow {
			t.Fatalf("mode = %v", g.Mode)
		}
		if len(g.Rects) != 1 {
			t.Fatalf("narrow should show one pane, got %d", len(g.Rects))
		}
		r, ok := g.Rects[active]
		if !ok {
			t.Fatalf("active pane %v missing", active)
		}
		if r.X != 0 || r.Y != 1 || r.W != w || r.H != h-1 {
			t.Errorf("active rect = %+v, want {0 1 %d %d}", r, w, h-1)
		}
	}
}

func TestNarrowHiddenActiveFallsBack(t *testing.T) {
	g := Solve(80, 20, State{ShowTree: false, ShowAgent: true, Active: ui.PaneTree})
	if _, ok := g.Rects[ui.PaneViewer]; !ok {
		t.Error("hidden active pane should fall back to viewer")
	}
}

func TestVisibleOrder(t *testing.T) {
	got := VisiblePanes(State{ShowTree: true, ShowAgent: true})
	want := []ui.PaneID{ui.PaneTree, ui.PaneViewer, ui.PaneTerminal, ui.PaneTerminal2, ui.PaneChanges, ui.PaneAgent}
	if len(got) != len(want) {
		t.Fatalf("visible = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("visible[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func assertRectsValid(t *testing.T, g Grid, w, h int) {
	t.Helper()
	ids := make([]ui.PaneID, 0, len(g.Rects))
	for id := range g.Rects {
		ids = append(ids, id)
	}
	for id, r := range g.Rects {
		if r.W <= 0 || r.H <= 0 {
			t.Errorf("%v: non-positive rect %+v", id, r)
		}
		if r.X < 0 || r.Y < 0 || r.X+r.W > w || r.Y+r.H > h {
			t.Errorf("%v: rect %+v out of bounds %dx%d", id, r, w, h)
		}
	}
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			a, b := g.Rects[ids[i]], g.Rects[ids[j]]
			if overlap(a, b) {
				t.Errorf("%v %+v overlaps %v %+v", ids[i], a, ids[j], b)
			}
		}
	}
}

func overlap(a, b Rect) bool {
	return a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H
}

func TestNoGapsOverflowInWide(t *testing.T) {
	for w := 120; w <= 300; w += 7 {
		for h := 30; h <= 80; h += 3 {
			g := Solve(w, h, State{ShowTree: true, ShowAgent: true, Active: ui.PaneViewer})
			if g.TooSmall {
				t.Fatalf("%dx%d too small", w, h)
			}
			assertRectsValid(t, g, w, h)
			for id, r := range g.Rects {
				if strings.Contains(id.String(), "Unknown") {
					t.Fatalf("bad id %v", id)
				}
				_ = r
			}
		}
	}
}
