package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lakshwin/bento/internal/theme"
)

// Panel chrome: top border + title row + (body) + footer row + bottom border.
// Panes size their body with BodySize so these rows are never double-counted.
const (
	ChromeRows = 4
	ChromeCols = 2
)

// BodySize returns the body area inside a Panel of the given outer size.
func BodySize(width, height int) (w, h int) {
	return max(0, width-ChromeCols), max(0, height-ChromeRows)
}

// Tab is one chip in a pane's title row.
type Tab struct {
	Label  string
	Active bool
}

// PanelSpec describes a pane's title row and footer.
type PanelSpec struct {
	Tabs []Tab
	// Right is optional, already-styled text aligned to the right of the title row.
	Right string
	Hints []KeyHint
}

// Spec builds a PanelSpec with a single active tab.
func Spec(title string, hints []KeyHint) PanelSpec {
	return PanelSpec{Tabs: []Tab{{Label: title, Active: true}}, Hints: hints}
}

// Panel renders a rounded pane with a title row of tab chips, the body, and a
// per-pane key-hint footer. body must be BodySize(width, height) tall.
func Panel(spec PanelSpec, width, height int, focused bool, body string) string {
	if width < 6 || height < ChromeRows+1 {
		title := ""
		for _, t := range spec.Tabs {
			if t.Active || title == "" {
				title = t.Label
			}
		}
		return Island(title, width, height, focused, body)
	}
	inner := width - ChromeCols
	bs := theme.BorderStyle(focused)
	b := theme.Border()

	side := func(content string) string {
		content = PaintBackground(padLine(content, inner), theme.P.Panel)
		return bs.Render(b.Left) + theme.PanelStyle().Render(content) + bs.Render(b.Right)
	}

	lines := strings.Split(body, "\n")
	out := make([]string, 0, height)
	out = append(out, bs.Render(b.TopLeft)+bs.Render(strings.Repeat(b.Top, inner))+bs.Render(b.TopRight))
	out = append(out, side(titleRow(spec, inner, focused)))
	for i := 0; i < height-ChromeRows; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		out = append(out, side(line))
	}
	out = append(out, side(footerRow(spec.Hints, focused)))
	out = append(out, bs.Render(b.BottomLeft)+bs.Render(strings.Repeat(b.Bottom, inner))+bs.Render(b.BottomRight))
	return strings.Join(out, "\n")
}

func titleRow(spec PanelSpec, inner int, focused bool) string {
	sp := theme.PanelStyle().Render(" ")
	var b strings.Builder
	b.WriteString(sp)
	for i, t := range spec.Tabs {
		if i > 0 {
			b.WriteString(sp)
		}
		b.WriteString(theme.TabChipStyle(t.Active, focused).Render(" " + t.Label + " "))
	}
	left := b.String()
	if spec.Right != "" {
		gap := inner - lipgloss.Width(left) - lipgloss.Width(spec.Right) - 1
		if gap >= 1 {
			left += strings.Repeat(" ", gap) + spec.Right + " "
		}
	}
	return left
}

func footerRow(hints []KeyHint, focused bool) string {
	var b strings.Builder
	b.WriteString(" ")
	n := 0
	for _, h := range hints {
		if h.Key == "" && h.Desc == "" {
			continue
		}
		if n > 0 {
			b.WriteString(theme.FooterSepStyle().Render(theme.Sep))
		}
		b.WriteString(theme.FooterKeyStyle(focused).Render(h.Key))
		if h.Desc != "" {
			b.WriteString(theme.FooterLabelStyle(focused).Render(" " + h.Desc))
		}
		n++
	}
	return b.String()
}
