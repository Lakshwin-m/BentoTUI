package palette

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
		return ui.Island("Commands", width, height, true, "")
	}

	input := theme.InputAccentStyle().Render(" > ") + theme.InputStyle().Render(m.input)
	if m.opened {
		input += theme.InputAccentStyle().Render(theme.Cursor)
	}
	input += "  " + theme.InputMutedStyle().Render(fmt.Sprintf("%d/%d", len(m.results), len(m.commands)))

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
	if len(m.results) == 0 {
		rows = append(rows, theme.MutedStyle().Render(" no matching commands"))
	}
	for len(rows) < listH {
		rows = append(rows, ui.Fit("", innerW))
	}
	if len(rows) > listH {
		rows = rows[:listH]
	}

	lines := make([]string, 0, innerH)
	lines = append(lines, theme.InputStyle().Render(ui.PaintBackground(ui.Fit(input, innerW), theme.P.Surface)))
	lines = append(lines, rows...)
	lines = append(lines, ui.Fit(ui.HintLine([]ui.KeyHint{
		{Key: "↑↓", Desc: "move"},
		{Key: "enter", Desc: "run"},
		{Key: "esc", Desc: "close"},
	}), innerW))
	for len(lines) < innerH {
		lines = append(lines, ui.Fit("", innerW))
	}
	if len(lines) > innerH {
		lines = lines[:innerH]
	}
	return ui.Island("Commands", width, height, true, strings.Join(lines, "\n"))
}

func (m *Model) row(i, width int) string {
	c := m.results[i]
	if i == m.sel {
		return theme.SelectedStyle().Render(ui.Fit(plainRow(c, width), width))
	}
	left := " " + theme.TextStyle().Render(c.Title)
	if c.Description != "" {
		left += theme.MutedStyle().Render("  " + c.Description)
	}
	if c.Keys == "" {
		return ui.Fit(left, width)
	}
	keys := theme.KeyStyle().Render(c.Keys)
	gap := width - lipgloss.Width(left) - lipgloss.Width(keys) - 2
	if gap < 1 {
		return ui.Fit(left, width)
	}
	return left + strings.Repeat(" ", gap) + keys + " "
}

func plainRow(c Command, width int) string {
	left := " " + c.Title
	if c.Description != "" {
		left += "  " + c.Description
	}
	if c.Keys != "" {
		gap := width - lipgloss.Width(left) - lipgloss.Width(c.Keys) - 2
		if gap > 0 {
			return left + strings.Repeat(" ", gap) + c.Keys + " "
		}
	}
	return left
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
