package ui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lakshwin/bento/internal/theme"
)

// Placed is a rendered pane region to composite onto the canvas.
type Placed struct {
	X, Y int
	View string
}

// Compose paints already-rendered pane views onto a width x height canvas.
// Views are expected to be exactly their rectangle size; Compose pads gaps and
// truncates anything that overflows the canvas.
func Compose(width, height int, items []Placed) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	items = append([]Placed(nil), items...)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Y != items[j].Y {
			return items[i].Y < items[j].Y
		}
		return items[i].X < items[j].X
	})

	lines := make([][]string, len(items))
	for i, it := range items {
		lines[i] = strings.Split(it.View, "\n")
	}

	rows := make([]string, height)
	for y := 0; y < height; y++ {
		var b strings.Builder
		cursor := 0
		for i, it := range items {
			rel := y - it.Y
			if rel < 0 || rel >= len(lines[i]) {
				continue
			}
			if it.X > cursor {
				b.WriteString(strings.Repeat(" ", it.X-cursor))
				cursor = it.X
			}
			line := lines[i][rel]
			b.WriteString(line)
			cursor += lipgloss.Width(line)
		}
		if cursor < width {
			b.WriteString(strings.Repeat(" ", width-cursor))
		}
		rows[y] = PaintBackground(Fit(b.String(), width), theme.P.Background)
	}
	return strings.Join(rows, "\n")
}
