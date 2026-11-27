package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

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
	// Position is an optional cursor/line indicator such as "Ln 12:4".
	Position string
	Mode     string
	// Zoom marks that the focused pane is temporarily maximized.
	Zoom bool
}

// HintLine renders key/description pairs as compact status-bar hints.
func HintLine(hints []KeyHint) string {
	parts := make([]string, 0, len(hints))
	for _, h := range hints {
		if h.Key == "" && h.Desc == "" {
			continue
		}
		part := theme.HintKeyStyle().Render(" " + h.Key + " ")
		if h.Desc != "" {
			part += " " + theme.HintDescStyle().Render(h.Desc)
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, " ")
}

// StatusBar renders a single-line, full-width status bar. The wordmark and
// project sit on the left, transient messages and the mode chip are right
// aligned, and key hints fill the space between.
func StatusBar(width int, d StatusData) string {
	if width <= 0 {
		return ""
	}

	left := theme.BrandStyle().Render("ben") + theme.BrandAltStyle().Render("to")
	if d.Project != "" {
		left += theme.StatusMutedStyle().Render(" "+theme.Bar+" ") + theme.StatusTextStyle().Bold(true).Render(d.Project)
	}
	if d.Branch != "" {
		left += theme.StatusMutedStyle().Render(" "+theme.Bar+" ") + theme.StatusAccentStyle().Render(d.Branch)
		if d.Dirty {
			left += theme.StatusWarningStyle().Render(" " + theme.Dirty)
		}
	}

	right := statusRight(d, true)
	if lipgloss.Width(left)+lipgloss.Width(right)+2 > width && d.Message != "" {
		// Tight terminal: drop the message before sacrificing the mode chip.
		right = statusRight(d, false)
	}
	if lipgloss.Width(right) >= width {
		line := PaintBackground(Fit(ansi.Truncate(right, width, ""), width), theme.P.StatusBarBg)
		return theme.StatusBarStyle().Render(line)
	}
	maxLeft := width - lipgloss.Width(right) - 1
	if maxLeft < 0 {
		maxLeft = 0
	}
	if lipgloss.Width(left) > maxLeft {
		left = ansi.Truncate(left, maxLeft, theme.Ellipsis)
	}

	hints := d.Hints
	if hints != "" {
		hints = strings.TrimSpace(hints)
	}
	avail := width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if avail < 0 {
		avail = 0
	}
	if hints != "" && avail > 0 {
		hints = ansi.Truncate(hints, avail, "")
	} else if avail <= 0 {
		hints = ""
	}

	line := left
	if hints != "" {
		line += " " + hints
	}
	pad := width - lipgloss.Width(line) - lipgloss.Width(right)
	if pad < 0 {
		pad = 0
	}
	line += strings.Repeat(" ", pad) + right
	line = PaintBackground(Fit(line, width), theme.P.StatusBarBg)
	return theme.StatusBarStyle().Render(line)
}

func statusRight(d StatusData, includeMessage bool) string {
	parts := make([]string, 0, 3)
	if includeMessage && d.Message != "" {
		icon := theme.Bullet
		style := theme.StatusWarningStyle()
		switch {
		case d.MessageIsError:
			icon, style = theme.Cross, theme.StatusErrorStyle()
		case strings.HasPrefix(d.Message, "Saved"), strings.HasPrefix(d.Message, "Installed"):
			icon, style = theme.Check, theme.StatusSuccessStyle()
		}
		parts = append(parts, style.Render(icon+" "+d.Message))
	}
	if d.Position != "" {
		parts = append(parts, theme.StatusMutedStyle().Render(" "+d.Position+" "))
	}
	if d.Zoom {
		parts = append(parts, theme.StatusWarningStyle().Render(" ZOOM "))
	}
	if d.Mode != "" {
		parts = append(parts, theme.ModeStyle().Render(" "+d.Mode+" "))
	}
	return strings.Join(parts, "  ")
}

// CenteredCard renders a centered title/body card for empty states. Small
// rectangles fall back to plain centered text so the card never overflows.
func CenteredCard(width, height int, title, body string) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	if width < 24 || height < 7 {
		text := theme.CardTitleStyle().Render(title)
		if body != "" {
			text += "\n" + theme.MutedStyle().Render(Truncate(body, width))
		}
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, text)
	}
	cardW := min(width-4, 58)
	text := theme.CardTitleStyle().Render(title)
	if body != "" {
		text += "\n\n" + theme.MutedStyle().Render(Truncate(body, cardW-6))
	}
	text = PaintBackground(text, theme.P.Panel)
	card := theme.CardStyle().Render(text)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, card)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
