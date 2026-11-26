package finder

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lakshwin/bento/internal/theme"
	"github.com/lakshwin/bento/internal/ui"
)

// View renders the overlay island at exactly width x height cells.
func (m *Model) View(width, height int) string {
	innerW := max(0, width-2)
	innerH := max(0, height-2)
	if innerW == 0 || innerH < 2 {
		return ui.Island("Find file", width, height, true, "")
	}

	var status string
	switch {
	case m.loading:
		status = "indexing…"
	case m.errMsg != "":
		status = "index error"
	default:
		status = fmt.Sprintf("%d/%d", len(m.results), len(m.files))
	}
	input := theme.AccentStyle().Render("> ") + theme.TextStyle().Render(m.input)
	if m.opened {
		input += theme.AccentStyle().Render(theme.Cursor)
	}
	input += "  " + theme.MutedStyle().Render(status)

	listH := innerH - 2
	if listH < 0 {
		listH = 0
	}
	rows := make([]string, 0, listH)
	end := m.scroll + listH
	if end > len(m.results) {
		end = len(m.results)
	}
	for i := m.scroll; i < end; i++ {
		rows = append(rows, m.row(i, innerW))
	}
	if len(m.results) == 0 && !m.loading {
		msg := "no matches"
		if m.input == "" {
			msg = "no files"
		}
		rows = append(rows, theme.MutedStyle().Render(" "+msg))
	}
	for len(rows) < listH {
		rows = append(rows, ui.Fit("", innerW))
	}
	if len(rows) > listH {
		rows = rows[:listH]
	}

	lines := make([]string, 0, innerH)
	lines = append(lines, ui.Fit(input, innerW))
	lines = append(lines, rows...)
	lines = append(lines, theme.MutedStyle().Render(ui.Fit(" enter open   esc cancel", innerW)))
	for len(lines) < innerH {
		lines = append(lines, ui.Fit("", innerW))
	}
	if len(lines) > innerH {
		lines = lines[:innerH]
	}
	return ui.Island("Find file", width, height, true, strings.Join(lines, "\n"))
}

func (m *Model) row(i, width int) string {
	r := m.results[i]
	selected := i == m.sel
	text, indices := m.displayPath(r, width)
	if selected {
		return theme.SelectedStyle().Render(ui.Fit(text, width))
	}
	return highlightPath(text, indices, width)
}

// displayPath left-truncates long paths so the basename stays visible, and
// shifts match indices to the displayed string.
func (m *Model) displayPath(r result, width int) (string, []int) {
	text := " " + r.path
	indices := make([]int, len(r.indices))
	for i, v := range r.indices {
		indices[i] = v + 1
	}
	if lipgloss.Width(text) <= width {
		return text, indices
	}
	runes := []rune(r.path)
	keep := width - 2
	if keep < 1 {
		return ui.Truncate(" "+r.path, width), nil
	}
	if keep >= len(runes) {
		return text, indices
	}
	cut := len(runes) - keep
	text = " " + theme.Ellipsis + string(runes[cut:])
	shifted := make([]int, 0, len(indices))
	for _, v := range r.indices {
		if v < cut {
			continue
		}
		shifted = append(shifted, v-cut+2)
	}
	return text, shifted
}
