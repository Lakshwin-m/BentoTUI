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
			b.WriteString(theme.TabDividerStyle().Render(" " + theme.Bar + " "))
		}
		if i == active {
			b.WriteString(theme.TabActiveStyle().Render(" " + label + " "))
		} else {
			b.WriteString(theme.TabInactiveStyle().Render(" " + label + " "))
		}
	}
	return PaintBackground(Fit(b.String(), width), theme.P.Background)
}
