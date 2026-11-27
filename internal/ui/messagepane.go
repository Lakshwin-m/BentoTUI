package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// MessagePane is a minimal pane used for empty states and placeholders.
type MessagePane struct {
	PaneTitle string
	Body      string
}

// NewMessagePane builds a static message pane.
func NewMessagePane(title, body string) *MessagePane {
	return &MessagePane{PaneTitle: title, Body: body}
}

func (p *MessagePane) Init() tea.Cmd { return nil }

func (p *MessagePane) Update(msg tea.Msg) (Pane, tea.Cmd) { return p, nil }

func (p *MessagePane) View(width, height int, focused bool) string {
	iw, ih := BodySize(width, height)
	body := CenteredCard(iw, ih, p.PaneTitle, p.Body)
	return Panel(Spec(p.PaneTitle, p.KeyHints()), width, height, focused, body)
}

func (p *MessagePane) Title() string { return p.PaneTitle }

func (p *MessagePane) KeyHints() []KeyHint {
	return []KeyHint{{Key: "?", Desc: "help"}, {Key: "ctrl+q", Desc: "quit"}}
}

var _ Pane = (*MessagePane)(nil)
