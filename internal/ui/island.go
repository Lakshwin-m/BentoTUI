package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/lakshwin/bento/internal/theme"
)

// Island renders a body inside a rounded border with a title embedded in the
// top edge. width and height include the border. The returned string has
// exactly height lines; every line is padded to exactly width cells.
func Island(title string, width, height int, focused bool, body string) string {
	if width < 2 {
		width = 2
	}
	if height < 2 {
		height = 2
	}
	inner := width - 2
	if inner < 2 || height < 3 {
		lines := make([]string, height)
		for i := range lines {
			lines[i] = Fit("", width)
		}
		return strings.Join(lines, "\n")
	}
	bs := theme.BorderStyle(focused)
	ts := theme.TitleStyle(focused)
	b := theme.Border()

	top := bs.Render(b.TopLeft)
	label := " "
	if title != "" {
		label = " " + title + " "
	}
	label = ansi.Truncate(label, max(0, inner-2), theme.Ellipsis)
	top += bs.Render(b.Top) + ts.Render(label)
	fill := inner - 1 - lipgloss.Width(label)
	if fill < 0 {
		fill = 0
	}
	top += bs.Render(strings.Repeat(b.Top, fill)) + bs.Render(b.TopRight)

	lines := strings.Split(body, "\n")
	out := make([]string, 0, height)
	out = append(out, top)
	for i := 0; i < height-2; i++ {
		var line string
		if i < len(lines) {
			line = lines[i]
		}
		line = padLine(line, inner)
		out = append(out, bs.Render(b.Left)+line+bs.Render(b.Right))
	}
	out = append(out, bs.Render(b.BottomLeft)+bs.Render(strings.Repeat(b.Bottom, inner))+bs.Render(b.BottomRight))
	return strings.Join(out, "\n")
}

// padLine truncates and right-pads a possibly ANSI-styled line to width cells.
func padLine(s string, width int) string {
	if width <= 0 {
		return ""
	}
	s = ansi.Truncate(s, width, theme.Ellipsis)
	gap := width - lipgloss.Width(s)
	if gap > 0 {
		s += strings.Repeat(" ", gap)
	}
	return s
}

// Fit truncates and right-pads a string to exactly width cells.
func Fit(s string, width int) string { return padLine(s, width) }

// Truncate shortens s to width cells, adding an ellipsis when it was cut.
func Truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(s, width, theme.Ellipsis)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
