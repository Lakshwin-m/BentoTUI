// Package layout computes the bento grid: a pure function from console size
// and UI state to pane rectangles. It has no knowledge of pane contents.
package layout

import (
	"github.com/lakshwin/bento/internal/ui"
)

// Mode is the responsive layout class for the current console size.
type Mode int

const (
	// Wide is the full bento grid.
	Wide Mode = iota
	// Medium hides the agent column and folds Changes into the bottom tabs.
	Medium
	// Narrow shows one pane at a time with a tab strip.
	Narrow
)

func (m Mode) String() string {
	switch m {
	case Wide:
		return "wide"
	case Medium:
		return "medium"
	case Narrow:
		return "narrow"
	default:
		return "unknown"
	}
}

// Breakpoints. MinHeight is the total console height, including the status
// bar; Solve receives content rows only, so it uses MinContentHeight.
const (
	MinWidth         = 60
	MinHeight        = 15
	MinContentHeight = MinHeight - 1
	WideWidth        = 120
	WideHeight       = 30
	MidWidth         = 90
	MidHeight        = 24
)

// Rect is a pane rectangle in content cells (status bar excluded).
type Rect struct {
	X, Y, W, H int
}

// State is the layout-relevant subset of application state.
type State struct {
	ShowTree  bool
	ShowAgent bool
	// Active is the visible pane in narrow mode.
	Active ui.PaneID
}

// Grid is the solved layout.
type Grid struct {
	Mode     Mode
	TooSmall bool
	Rects    map[ui.PaneID]Rect
	// Visible is the focus-cycle order.
	Visible []ui.PaneID
}

// ModeFor classifies a console size.
func ModeFor(w, h int) Mode {
	switch {
	case w >= WideWidth && h >= WideHeight:
		return Wide
	case w >= MidWidth && h >= MidHeight:
		return Medium
	default:
		return Narrow
	}
}

// VisiblePanes returns the panes reachable in this state, in cycle order.
func VisiblePanes(st State) []ui.PaneID {
	out := make([]ui.PaneID, 0, 6)
	if st.ShowTree {
		out = append(out, ui.PaneTree)
	}
	out = append(out, ui.PaneViewer, ui.PaneTerminal, ui.PaneTerminal2, ui.PaneChanges)
	if st.ShowAgent {
		out = append(out, ui.PaneAgent)
	}
	return out
}

// Solve computes the full grid. width and height are content cells (the status
// bar is drawn by the caller and is not part of height).
func Solve(w, h int, st State) Grid {
	g := Grid{Mode: ModeFor(w, h), Rects: map[ui.PaneID]Rect{}}
	g.Visible = VisiblePanes(st)
	if w < MinWidth || h < MinContentHeight {
		g.TooSmall = true
		return g
	}
	switch g.Mode {
	case Wide:
		solveWide(w, h, st, &g)
	case Medium:
		solveMedium(w, h, st, &g)
	default:
		solveNarrow(w, h, st, &g)
	}
	return g
}

// Compute returns just the rectangles, matching the documented pane contract.
func Compute(w, h int, st State) map[ui.PaneID]Rect {
	return Solve(w, h, st).Rects
}

func solveWide(w, h int, st State, g *Grid) {
	// Wide layout mirrors the reference workspace: viewer and agent across the
	// top, files and changes stacked on the right, and two shells filling the
	// full-width bottom row. The agent pane is intentionally large enough for a
	// full-screen TUI.
	rightW := clamp(w/5, 26, 34)
	leftW := clamp(w*2/5, 38, 58)
	centerW := w - leftW - rightW - 2
	if centerW < 30 {
		centerW = 30
	}
	bottomH := clamp(h/4, 8, h-10)
	topH := h - bottomH - 1

	viewerW := leftW
	if !st.ShowAgent {
		viewerW = leftW + 1 + centerW
	}
	g.Rects[ui.PaneViewer] = Rect{X: 0, Y: 0, W: viewerW, H: topH}
	centerX := leftW + 1
	if st.ShowAgent {
		g.Rects[ui.PaneAgent] = Rect{X: centerX, Y: 0, W: centerW, H: topH}
	}

	rightX := centerX + centerW + 1
	if st.ShowTree {
		treeH := clamp(topH*2/3, 8, topH-5)
		g.Rects[ui.PaneTree] = Rect{X: rightX, Y: 0, W: rightW, H: treeH}
		g.Rects[ui.PaneChanges] = Rect{X: rightX, Y: treeH + 1, W: rightW, H: topH - treeH - 1}
	} else {
		g.Rects[ui.PaneChanges] = Rect{X: rightX, Y: 0, W: rightW, H: topH}
	}

	leftTermW := (w - 1) / 2
	g.Rects[ui.PaneTerminal] = Rect{X: 0, Y: topH + 1, W: leftTermW, H: bottomH}
	g.Rects[ui.PaneTerminal2] = Rect{X: leftTermW + 1, Y: topH + 1, W: w - leftTermW - 1, H: bottomH}
}

func solveMedium(w, h int, st State, g *Grid) {
	treeW := 0
	leftGutter := 0
	if st.ShowTree {
		treeW = clamp(w/5, 18, 28)
		leftGutter = 1
		g.Rects[ui.PaneTree] = Rect{X: 0, Y: 0, W: treeW, H: h}
	}
	midX := treeW + leftGutter
	midW := w - midX
	previewH := clamp(h*3/5, 8, h-7)
	g.Rects[ui.PaneViewer] = Rect{X: midX, Y: 0, W: midW, H: previewH}
	bottomH := h - previewH - 1
	leftTermW := (midW - 1) / 2
	g.Rects[ui.PaneTerminal] = Rect{X: midX, Y: previewH + 1, W: leftTermW, H: bottomH}
	g.Rects[ui.PaneTerminal2] = Rect{X: midX + leftTermW + 1, Y: previewH + 1, W: midW - leftTermW - 1, H: bottomH}
	// Changes is reachable as a tab in the bottom zone, so it has no rect.
}

func solveNarrow(w, h int, st State, g *Grid) {
	active := st.Active
	valid := false
	for _, id := range g.Visible {
		if id == active {
			valid = true
			break
		}
	}
	if !valid {
		active = ui.PaneViewer
	}
	// Row 0 is reserved for the tab strip.
	g.Rects[active] = Rect{X: 0, Y: 1, W: w, H: h - 1}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
