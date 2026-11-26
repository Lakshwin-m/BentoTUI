package ui

import (
	"strings"

	"github.com/lakshwin/bento/internal/theme"
)

// TabStrip renders the narrow-mode pane tab strip. active is an index into
// labels. The result is exactly width cells wide and one line tall.
func TabStrip(width int, labels []string, active int) string {
	if width <= 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(" ")
	for i, label := range labels {
		if i > 0 {
			b.WriteString(theme.MutedStyle().Render(" " + theme.Bar + " "))
		}
		if i == active {
			b.WriteString(theme.ModeStyle().Render(" " + label + " "))
		} else {
			b.WriteString(theme.MutedStyle().Render(" " + label + " "))
		}
	}
	return Fit(b.String(), width)
}
