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
	Surface       lipgloss.Color
	Border        lipgloss.Color
	BorderSubtle  lipgloss.Color
	BorderFocused lipgloss.Color
	Title         lipgloss.Color
	TitleFocused  lipgloss.Color
	Text          lipgloss.Color
	TextMuted     lipgloss.Color
	Accent        lipgloss.Color
	AccentAlt     lipgloss.Color
	Added         lipgloss.Color
	Removed       lipgloss.Color
	Modified      lipgloss.Color
	Untracked     lipgloss.Color
	Conflicted    lipgloss.Color
	Selection     lipgloss.Color
	SelectionDim  lipgloss.Color
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

// Dark is the "Lagoon" palette: an ocean-teal night scene. Three teal surfaces
// (Background < Panel < Surface), sand text, one bright cyan for focus, coral
// for bad/dirty and palm green for good/running.
var Dark = Palette{
	Background:    lipgloss.Color("#071E26"), // ocean: page and gaps
	Panel:         lipgloss.Color("#0C2832"), // pane fill
	Surface:       lipgloss.Color("#14343C"), // raised: keycaps, inputs
	Border:        lipgloss.Color("#24555C"),
	BorderSubtle:  lipgloss.Color("#24555C"),
	BorderFocused: lipgloss.Color("#3DDECE"),
	Title:         lipgloss.Color("#B7A88A"),
	TitleFocused:  lipgloss.Color("#F4E4C4"),
	Text:          lipgloss.Color("#F4E4C4"), // sand
	TextMuted:     lipgloss.Color("#B7A88A"), // sand-dim
	Accent:        lipgloss.Color("#3DDECE"), // lagoon-bright
	AccentAlt:     lipgloss.Color("#E9C46A"), // sun
	Added:         lipgloss.Color("#7DCE82"), // palm
	Removed:       lipgloss.Color("#FF6F61"), // coral
	Modified:      lipgloss.Color("#E9C46A"), // sun
	Untracked:     lipgloss.Color("#5FB3C4"), // reef
	Conflicted:    lipgloss.Color("#F4A261"), // amber
	Selection:     lipgloss.Color("#1B5E5A"), // selected row, focused pane
	SelectionDim:  lipgloss.Color("#14343C"), // selected row, unfocused pane
	Match:         lipgloss.Color("#1F8F86"),
	MatchCurrent:  lipgloss.Color("#E9C46A"),
	Gutter:        lipgloss.Color("#5F7D7B"), // moss
	CurrentLine:   lipgloss.Color("#14343C"),
	StatusBarBg:   lipgloss.Color("#04161C"),
	StatusBarFg:   lipgloss.Color("#F4E4C4"),
	ModeBg:        lipgloss.Color("#3DDECE"),
	ModeFg:        lipgloss.Color("#071E26"),
	OverlayBg:     lipgloss.Color("#0C2832"),
	OverlayBorder: lipgloss.Color("#3DDECE"),
	Error:         lipgloss.Color("#FF6F61"),
	Success:       lipgloss.Color("#7DCE82"),
	Warning:       lipgloss.Color("#E9C46A"),
}

// P is the active palette. Restyling should only require changing this value.
var P = Dark

// Border returns the island border glyphs. Rounded panels are the default look.
func Border() lipgloss.Border { return lipgloss.RoundedBorder() }

// BorderStyle returns the style applied to border glyphs.
func BorderStyle(focused bool) lipgloss.Style {
	c := P.BorderSubtle
	if focused {
		c = P.BorderFocused
	}
	return lipgloss.NewStyle().Foreground(c).Background(P.Panel)
}

// TitleStyle returns the style for a pane title.
func TitleStyle(focused bool) lipgloss.Style {
	s := lipgloss.NewStyle().Foreground(P.Title).Background(P.Panel)
	if focused {
		s = s.Foreground(P.Accent).Bold(true)
	}
	return s
}

// TitleChipStyle renders the focused pane title as a filled accent chip.
func TitleChipStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.Accent).Foreground(P.Background).Bold(true)
}

// BrandStyle renders the Bento wordmark.
func BrandStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.StatusBarBg).Foreground(P.Accent).Bold(true)
}

// BrandAltStyle renders the second half of the Bento wordmark.
func BrandAltStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.StatusBarBg).Foreground(P.AccentAlt).Bold(true)
}

// HintKeyStyle renders a keycap in the status bar.
func HintKeyStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.Surface).Foreground(P.Accent).Bold(true)
}

// HintDescStyle dims the description next to a keycap.
func HintDescStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.StatusBarBg).Foreground(P.TextMuted)
}

// StatusTextStyle is body text on the status bar surface.
func StatusTextStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.StatusBarBg).Foreground(P.Text)
}

// StatusMutedStyle is secondary text on the status bar surface.
func StatusMutedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.StatusBarBg).Foreground(P.TextMuted)
}

// StatusAccentStyle is emphasized text on the status bar surface.
func StatusAccentStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.StatusBarBg).Foreground(P.Accent)
}

// StatusAltStyle is the secondary accent on the status bar surface.
func StatusAltStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.StatusBarBg).Foreground(P.AccentAlt)
}

// StatusErrorStyle is error text on the status bar surface.
func StatusErrorStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.StatusBarBg).Foreground(P.Error).Bold(true)
}

// StatusSuccessStyle is success text on the status bar surface.
func StatusSuccessStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.StatusBarBg).Foreground(P.Success).Bold(true)
}

// StatusWarningStyle is warning text on the status bar surface.
func StatusWarningStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.StatusBarBg).Foreground(P.Warning).Bold(true)
}

// CardStyle frames an empty-state card inside a pane.
func CardStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(P.Panel).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(P.Border).
		Padding(1, 2)
}

// GroupHeaderStyle renders a section heading in the help overlay.
func GroupHeaderStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.Panel).Foreground(P.AccentAlt).Bold(true)
}

// KeycapStyle renders a keyboard key in overlays.
func KeycapStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.Surface).Foreground(P.Accent).Bold(true)
}

// VersionStyle renders a small version badge.
func VersionStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.Surface).Foreground(P.AccentAlt).Bold(true)
}

// RuleStyle renders a horizontal divider line.
func RuleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.Panel).Foreground(P.Border)
}

// InputStyle renders an overlay's search input on a raised surface.
func InputStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.Surface).Foreground(P.Text)
}

// InputAccentStyle is the prompt glyph on an input surface.
func InputAccentStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.Surface).Foreground(P.Accent).Bold(true)
}

// InputMutedStyle is secondary text on an input surface.
func InputMutedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.Surface).Foreground(P.TextMuted)
}

// PanelStyle is the base background style for pane content.
func PanelStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.Panel).Foreground(P.Text)
}

// TextStyle is the default body text style.
func TextStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.Panel).Foreground(P.Text)
}

// MutedStyle dims secondary text.
func MutedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.Panel).Foreground(P.TextMuted)
}

// AccentStyle highlights active elements.
func AccentStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.Panel).Foreground(P.Accent)
}

// KeyStyle renders keyboard keys in status hints and palette rows.
func KeyStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.Panel).Foreground(P.Accent).Bold(true)
}

// CardTitleStyle is used for centered empty states inside pane surfaces.
func CardTitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.Panel).Foreground(P.Accent).Bold(true)
}

// TabActiveStyle renders the selected pane or shell tab.
func TabActiveStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.Accent).Foreground(P.Background).Bold(true)
}

// TabInactiveStyle renders an available but unfocused tab.
func TabInactiveStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(P.TextMuted)
}

// TabDividerStyle renders the quiet separator between tabs.
func TabDividerStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(P.Border)
}

// ErrorStyle renders errors.
func ErrorStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.Panel).Foreground(P.Error)
}

// SuccessStyle renders success states.
func SuccessStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.Panel).Foreground(P.Success)
}

// WarningStyle renders warnings.
func WarningStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.Panel).Foreground(P.Warning)
}

// StatusBarStyle is the background style for the bottom status bar.
func StatusBarStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.StatusBarBg).Foreground(P.StatusBarFg)
}

// ModeStyle renders the mode indicator chip.
func ModeStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.Accent).Foreground(P.Background).Bold(true)
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
	return lipgloss.NewStyle().Background(P.Selection).Foreground(P.TitleFocused).Bold(true)
}

// SelectedAccentStyle is the selected-row style for the active marker and title.
func SelectedAccentStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.Selection).Foreground(P.Accent).Bold(true)
}

// SelectedMutedStyle is the selected-row style for secondary detail text.
func SelectedMutedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.Selection).Foreground(P.Title)
}

// CanvasStyle gives the unused gutter cells a tinted, glass-like surface.
func CanvasStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.Background).Foreground(P.Text)
}

// CursorStyle renders the editor's insertion cursor cell.
func CursorStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.Accent).Foreground(P.Background)
}

// GitStatusStyle renders a git status letter.
func GitStatusStyle(letter byte) lipgloss.Style {
	return lipgloss.NewStyle().Background(P.Panel).Foreground(GitStatusColor(letter))
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

// TabChipStyle renders a title-row tab. The active tab is a lighter chip; it
// brightens when its pane has focus.
func TabChipStyle(active, focused bool) lipgloss.Style {
	switch {
	case active && focused:
		return lipgloss.NewStyle().Background(P.Selection).Foreground(P.Accent).Bold(true)
	case active:
		return lipgloss.NewStyle().Background(P.Selection).Foreground(P.Text)
	}
	return lipgloss.NewStyle().Background(P.SelectionDim).Foreground(P.TextMuted)
}

// FooterKeyStyle is the key in a pane footer hint; cyan when focused.
func FooterKeyStyle(focused bool) lipgloss.Style {
	fg := P.Text
	if focused {
		fg = P.Accent
	}
	return lipgloss.NewStyle().Background(P.Panel).Foreground(fg)
}

// FooterLabelStyle is the label in a pane footer hint.
func FooterLabelStyle(focused bool) lipgloss.Style {
	fg := P.Gutter
	if focused {
		fg = P.Text
	}
	return lipgloss.NewStyle().Background(P.Panel).Foreground(fg)
}

// FooterSepStyle is the " · " between footer hints.
func FooterSepStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(P.Panel).Foreground(P.Gutter)
}

// SelectedRowStyle is the selected-row fill: bright when the pane is focused.
func SelectedRowStyle(focused bool) lipgloss.Style {
	bg := P.SelectionDim
	if focused {
		bg = P.Selection
	}
	return lipgloss.NewStyle().Background(bg).Foreground(P.TitleFocused).Bold(focused)
}
