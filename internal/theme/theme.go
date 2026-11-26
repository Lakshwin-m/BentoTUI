// Package theme centralizes every visual decision for Bento.
//
// TODO(user): final font and UI styling will be supplied by the owner.
// Keep every color, border style, and glyph defined here, never inline in panes.
//
// Bento targets Windows Terminal with its default font (Cascadia Mono). Do not
// introduce Nerd Font glyphs. All glyphs live in glyphs.go so they can be
// swapped when the final font and design arrive.
package theme

import "github.com/charmbracelet/lipgloss"

// Color roles. Panes and widgets must reference roles, never raw hex values.
type Palette struct {
	Background    lipgloss.Color
	Panel         lipgloss.Color
	Border        lipgloss.Color
	BorderFocused lipgloss.Color
	Title         lipgloss.Color
	TitleFocused  lipgloss.Color
	Text          lipgloss.Color
	TextMuted     lipgloss.Color
	Accent        lipgloss.Color
	Added         lipgloss.Color
	Removed       lipgloss.Color
	Modified      lipgloss.Color
	Untracked     lipgloss.Color
	Conflicted    lipgloss.Color
	Selection     lipgloss.Color
	Match         lipgloss.Color
	MatchCurrent  lipgloss.Color
	Gutter        lipgloss.Color
	CurrentLine   lipgloss.Color
	StatusBarBg   lipgloss.Color
	StatusBarFg   lipgloss.Color
	ModeBg        lipgloss.Color
	ModeFg        lipgloss.Color
	OverlayBg     lipgloss.Color
	OverlayBorder lipgloss.Color
	Error         lipgloss.Color
	Success       lipgloss.Color
	Warning       lipgloss.Color
}

// Dark is the default (and currently only) palette.
var Dark = Palette{
	Background:    lipgloss.Color("#0f1115"),
	Panel:         lipgloss.Color("#151922"),
	Border:        lipgloss.Color("#2a2f3a"),
	BorderFocused: lipgloss.Color("#7aa2f7"),
	Title:         lipgloss.Color("#7f8ca6"),
	TitleFocused:  lipgloss.Color("#c0caf5"),
	Text:          lipgloss.Color("#c0caf5"),
	TextMuted:     lipgloss.Color("#565f89"),
	Accent:        lipgloss.Color("#7aa2f7"),
	Added:         lipgloss.Color("#9ece6a"),
	Removed:       lipgloss.Color("#f7768e"),
	Modified:      lipgloss.Color("#e0af68"),
	Untracked:     lipgloss.Color("#7dcfff"),
	Conflicted:    lipgloss.Color("#bb9af7"),
	Selection:     lipgloss.Color("#283457"),
	Match:         lipgloss.Color("#3b4261"),
	MatchCurrent:  lipgloss.Color("#e0af68"),
	Gutter:        lipgloss.Color("#565f89"),
	CurrentLine:   lipgloss.Color("#1b1f2a"),
	StatusBarBg:   lipgloss.Color("#1b1f2a"),
	StatusBarFg:   lipgloss.Color("#c0caf5"),
	ModeBg:        lipgloss.Color("#7aa2f7"),
	ModeFg:        lipgloss.Color("#0f1115"),
	OverlayBg:     lipgloss.Color("#151922"),
	OverlayBorder: lipgloss.Color("#7aa2f7"),
	Error:         lipgloss.Color("#f7768e"),
	Success:       lipgloss.Color("#9ece6a"),
	Warning:       lipgloss.Color("#e0af68"),
}

// P is the active palette. Restyling should only require changing this value.
var P = Dark

// Border returns the island border glyphs. Rounded panels are the default look.
func Border() lipgloss.Border { return lipgloss.RoundedBorder() }

// BorderStyle returns the style applied to border glyphs.
func BorderStyle(focused bool) lipgloss.Style {
	c := P.Border
	if focused {
		c = P.BorderFocused
	}
	return lipgloss.NewStyle().Foreground(c)
}

// TitleStyle returns the style for a pane title.
func TitleStyle(focused bool) lipgloss.Style {
	s := lipgloss.NewStyle().Foreground(P.Title)
	if focused {
		s = s.Foreground(P.TitleFocused).Bold(true)
	}
	return s
}

// PanelStyle is the base background style for pane content.
func PanelStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.Panel).Foreground(P.Text)
}

// TextStyle is the default body text style.
func TextStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(P.Text)
}

// MutedStyle dims secondary text.
func MutedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(P.TextMuted)
}

// AccentStyle highlights active elements.
func AccentStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(P.Accent)
}

// ErrorStyle renders errors.
func ErrorStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(P.Error)
}

// SuccessStyle renders success states.
func SuccessStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(P.Success)
}

// WarningStyle renders warnings.
func WarningStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(P.Warning)
}

// StatusBarStyle is the background style for the bottom status bar.
func StatusBarStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(P.StatusBarBg).
		Foreground(P.StatusBarFg)
}

// ModeStyle renders the mode indicator chip.
func ModeStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(P.ModeBg).
		Foreground(P.ModeFg).
		Bold(true)
}

// OverlayStyle is the frame for centered overlays.
func OverlayStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(P.OverlayBg).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(P.OverlayBorder)
}

// SelectedStyle is the background used for the focused row or selection.
func SelectedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.Selection).Foreground(P.Text).Bold(true)
}

// CursorStyle renders the editor's insertion cursor cell.
func CursorStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.Accent).Foreground(P.Background)
}

// GitStatusStyle renders a git status letter.
func GitStatusStyle(letter byte) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(GitStatusColor(letter))
}

// GitStatusColor maps a git status letter to its color role.
func GitStatusColor(letter byte) lipgloss.Color {
	switch letter {
	case 'M':
		return P.Modified
	case 'A':
		return P.Added
	case 'D':
		return P.Removed
	case 'U':
		return P.Conflicted
	case '?':
		return P.Untracked
	default:
		return P.TextMuted
	}
}
