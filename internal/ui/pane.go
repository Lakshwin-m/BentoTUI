// Package ui holds shared widgets and the pane contract used by every pane.
package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// PaneID identifies one slot in the bento grid.
type PaneID int

const (
	PaneTree PaneID = iota
	PaneViewer
	PaneTerminal
	PaneTerminal2
	PaneChanges
	PaneAgent

	// PaneCount is the number of addressable panes.
	PaneCount
)

// String returns a stable, human-readable pane name.
func (p PaneID) String() string {
	switch p {
	case PaneTree:
		return "Tree"
	case PaneViewer:
		return "Viewer"
	case PaneTerminal:
		return "Terminal"
	case PaneTerminal2:
		return "Terminal 2"
	case PaneChanges:
		return "Changes"
	case PaneAgent:
		return "Agent"
	default:
		return "Unknown"
	}
}

// KeyHint is a single status-bar keyboard hint.
type KeyHint struct {
	Key  string
	Desc string
}

// PaneSizeMsg tells a pane its current rectangle size. Panes that need the
// size for paging or wrapping store it; rendering receives size directly.
type PaneSizeMsg struct {
	ID     PaneID
	Width  int
	Height int
}

// ClickMsg is a left-click in pane-local coordinates (0,0 is the pane's
// top-left cell, including the island border).
type ClickMsg struct {
	X, Y int
}

// Pane is the small contract every pane satisfies. Panes must not import each
// other; cross-pane effects travel as messages routed by the root model.
type Pane interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (Pane, tea.Cmd)
	View(width, height int, focused bool) string
	Title() string
	KeyHints() []KeyHint
}
