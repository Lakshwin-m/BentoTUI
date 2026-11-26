package finder

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lakshwin/bento/internal/theme"
	"github.com/lakshwin/bento/internal/ui"
)

var matchedStyle = lipgloss.NewStyle().
	Background(theme.P.Match).
	Foreground(theme.P.Text).
	Bold(true)

func highlightPath(path string, indices []int, width int) string {
	if len(indices) == 0 {
		return ui.Fit(path, width)
	}
	runes := []rune(path)
	var b strings.Builder
	last := 0
	for _, idx := range indices {
		if idx < last || idx >= len(runes) {
			continue
		}
		if idx > last {
			b.WriteString(string(runes[last:idx]))
		}
		b.WriteString(matchedStyle.Render(string(runes[idx])))
		last = idx + 1
	}
	if last < len(runes) {
		b.WriteString(string(runes[last:]))
	}
	return ui.Fit(b.String(), width)
}
