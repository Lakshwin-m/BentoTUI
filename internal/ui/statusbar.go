package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lakshwin/bento/internal/theme"
)

// StatusData is the content of the bottom status bar.
type StatusData struct {
	Project string
	Branch  string
	Dirty   bool
	Hints   string
	Message string
	// MessageIsError colors the transient message red.
	MessageIsError bool
	Mode           string
}

// StatusBar renders a single-line, full-width status bar.
func StatusBar(width int, d StatusData) string {
	if width <= 0 {
		return ""
	}

	left := theme.AccentStyle().Render("bento")
	if d.Project != "" {
		left += theme.MutedStyle().Render(theme.Sep) + theme.TextStyle().Render(d.Project)
	}
	if d.Branch != "" {
		left += theme.MutedStyle().Render(" "+theme.Bar+" ") + theme.TextStyle().Render(d.Branch)
		if d.Dirty {
			left += theme.WarningStyle().Render(" " + theme.Dirty)
		}
	}

	hints := theme.MutedStyle().Render(d.Hints)

	var msg string
	if d.Message != "" {
		style := theme.WarningStyle()
		if d.MessageIsError {
			style = theme.ErrorStyle()
		}
		msg = style.Render(d.Message)
	}

	mode := ""
	if d.Mode != "" {
		mode = theme.ModeStyle().Render(" " + d.Mode + " ")
	}

	parts := []string{left}
	if d.Hints != "" {
		parts = append(parts, hints)
	}
	if msg != "" {
		parts = append(parts, msg)
	}
	if mode != "" {
		parts = append(parts, mode)
	}
	line := strings.Join(parts, "  ")
	line = Fit(line, width)
	return theme.StatusBarStyle().Render(line)
}

// CenteredCard renders a centered title/body block for empty states.
func CenteredCard(width, height int, title, body string) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	text := theme.TitleStyle(true).Render(title)
	if body != "" {
		text += "\n\n" + theme.MutedStyle().Render(body)
	}
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, text)
}
