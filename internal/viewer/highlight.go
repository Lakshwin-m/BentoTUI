package viewer

import (
	"path/filepath"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lakshwin/bento/internal/theme"
)

// HighlightedMsg carries re-highlighted lines for an edited buffer.
type HighlightedMsg struct {
	Path   string
	Styled []string
	Gen    int
}

// HighlightCmd re-tokenizes an edited buffer off the UI goroutine. Gen guards
// against stale results if the buffer changed again before it finished.
func HighlightCmd(path string, lines []string, gen int) tea.Cmd {
	snapshot := append([]string(nil), lines...)
	return func() tea.Msg {
		return HighlightedMsg{
			Path:   path,
			Styled: HighlightLines(strings.Join(snapshot, "\n"), path),
			Gen:    gen,
		}
	}
}

var (
	styleOnce sync.Once
	style     *chroma.Style
)

func chromaStyle() *chroma.Style {
	styleOnce.Do(func() {
		style = chroma.MustNewStyle("bento", chroma.StyleEntries{
			chroma.Text:                hex(theme.P.Text),
			chroma.Comment:             "italic " + hex(theme.P.TextMuted),
			chroma.Keyword:             hex(theme.P.Accent),
			chroma.KeywordType:         hex(theme.P.Accent),
			chroma.KeywordConstant:     hex(theme.P.Modified),
			chroma.Name:                hex(theme.P.Text),
			chroma.NameClass:           hex(theme.P.Conflicted),
			chroma.NameFunction:        hex(theme.P.Untracked),
			chroma.NameBuiltin:         hex(theme.P.Accent),
			chroma.NameAttribute:       hex(theme.P.Modified),
			chroma.NameConstant:        hex(theme.P.Modified),
			chroma.NameVariable:        hex(theme.P.Text),
			chroma.LiteralString:       hex(theme.P.Added),
			chroma.LiteralStringEscape: hex(theme.P.Modified),
			chroma.LiteralNumber:       hex(theme.P.Modified),
			chroma.Operator:            hex(theme.P.Text),
			chroma.Punctuation:         hex(theme.P.TextMuted),
			chroma.GenericHeading:      "bold " + hex(theme.P.Accent),
			chroma.GenericSubheading:   hex(theme.P.Accent),
			chroma.GenericInserted:     hex(theme.P.Added),
			chroma.GenericDeleted:      hex(theme.P.Removed),
			chroma.GenericEmph:         "italic " + hex(theme.P.Text),
			chroma.GenericStrong:       "bold " + hex(theme.P.Text),
			chroma.Error:               hex(theme.P.Error),
		})
	})
	return style
}

func hex(c lipgloss.Color) string { return string(c) }

func tokenStyle(e chroma.StyleEntry) lipgloss.Style {
	s := lipgloss.NewStyle().Background(theme.P.Panel)
	if e.Colour.IsSet() {
		s = s.Foreground(lipgloss.Color(e.Colour.String()))
	}
	if e.Bold == chroma.Yes {
		s = s.Bold(true)
	}
	if e.Italic == chroma.Yes {
		s = s.Italic(true)
	}
	if e.Underline == chroma.Yes {
		s = s.Underline(true)
	}
	return s
}

// HighlightLines tokenizes source and returns one ANSI-styled string per line.
// Line count matches strings.Split(source, "\n"), including a trailing empty
// line when source ends with a newline.
func HighlightLines(source, filename string) []string {
	lexer := lexers.Match(filepath.Base(filename))
	if lexer == nil {
		lexer = lexers.Get("plaintext")
	}
	if lexer == nil {
		return plainLines(source)
	}
	lexer = chroma.Coalesce(lexer)
	it, err := lexer.Tokenise(nil, source)
	if err != nil {
		return plainLines(source)
	}

	st := chromaStyle()
	lines := []string{""}
	for tok := it(); tok != chroma.EOF; tok = it() {
		style := tokenStyle(st.Get(tok.Type))
		parts := strings.Split(tok.Value, "\n")
		for i, part := range parts {
			if i > 0 {
				lines = append(lines, "")
			}
			if part != "" {
				lines[len(lines)-1] += style.Render(part)
			}
		}
	}
	return lines
}

func plainLines(source string) []string {
	return strings.Split(source, "\n")
}
