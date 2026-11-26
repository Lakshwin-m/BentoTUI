package viewer

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lakshwin/bento/internal/theme"
	"github.com/lakshwin/bento/internal/ui"
)

type searchMatch struct {
	line  int
	start int // rune index, inclusive
	end   int // rune index, exclusive
}

var (
	matchStyle = lipgloss.NewStyle().
			Background(theme.P.Match).
			Foreground(theme.P.Text)
	currentMatchStyle = lipgloss.NewStyle().
				Background(theme.P.MatchCurrent).
				Foreground(theme.P.Background).
				Bold(true)
)

// StartSearch enters incremental search mode. It returns false when no file is
// loaded.
func (m *Model) StartSearch() bool {
	if m.path == "" || m.binary || m.tooLarge || m.editing {
		return false
	}
	m.searching = true
	m.query = ""
	m.matches = nil
	m.matchIdx = 0
	return true
}

// Searching reports whether the search input is capturing keys.
func (m *Model) Searching() bool { return m.searching }

// HasQuery reports whether a search is highlighted.
func (m *Model) HasQuery() bool { return m.query != "" }

func (m *Model) hasSearchUI() bool { return m.searching || m.query != "" }

func (m *Model) handleSearchKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.searching = false
		m.clearSearch()
	case "enter":
		m.searching = false
	case "backspace":
		r := []rune(m.query)
		if len(r) > 0 {
			m.query = string(r[:len(r)-1])
		}
		m.recomputeMatches()
	case "ctrl+u":
		m.query = ""
		m.recomputeMatches()
	default:
		switch msg.Type {
		case tea.KeyRunes:
			m.query += string(msg.Runes)
			m.recomputeMatches()
		case tea.KeySpace:
			m.query += " "
			m.recomputeMatches()
		}
	}
	return nil
}

func (m *Model) clearSearch() {
	m.query = ""
	m.matches = nil
	m.matchIdx = 0
}

func (m *Model) recomputeMatches() {
	m.matches = findMatches(m.lines, m.query)
	m.matchIdx = 0
	if len(m.matches) > 0 {
		m.jumpTo(0)
	}
}

func (m *Model) jumpTo(idx int) {
	if len(m.matches) == 0 {
		return
	}
	n := len(m.matches)
	idx = ((idx % n) + n) % n
	m.matchIdx = idx
	mt := m.matches[idx]
	m.revealLine(mt.line)
	m.cursor = mt.line
	m.clamp()
	m.scrollMatchIntoView(mt)
}

func (m *Model) scrollMatchIntoView(mt searchMatch) {
	if m.width < 4 {
		return
	}
	textW := m.width - 2 - (len(strconv.Itoa(max(1, len(m.lines)))) + 3)
	if textW < 1 {
		return
	}
	r := []rune(m.lines[mt.line])
	if mt.start > len(r) {
		mt.start = len(r)
	}
	prefix := lipgloss.Width(string(r[:mt.start]))
	if prefix < m.hscroll {
		m.hscroll = prefix
	} else if prefix >= m.hscroll+textW {
		m.hscroll = prefix - textW/2
		if m.hscroll < 0 {
			m.hscroll = 0
		}
	}
}

func (m *Model) searchPrompt(innerW int) string {
	count := "type to search"
	if m.query != "" {
		if len(m.matches) == 0 {
			count = "no matches"
		} else {
			count = fmt.Sprintf("%d/%d", m.matchIdx+1, len(m.matches))
		}
	}
	line := theme.AccentStyle().Render(" / ") + theme.TextStyle().Render(m.query)
	if m.searching {
		line += theme.AccentStyle().Render(theme.Cursor)
	}
	line += "  " + theme.MutedStyle().Render(count)
	return theme.PanelStyle().Render(ui.Fit(line, innerW))
}

func (m *Model) searchLine(i int) string {
	if i < 0 || i >= len(m.lines) {
		return ""
	}
	line := m.lines[i]
	curStart, curEnd, hasCur := -1, -1, false
	if m.matchIdx >= 0 && m.matchIdx < len(m.matches) {
		mt := m.matches[m.matchIdx]
		if mt.line == i {
			curStart, curEnd, hasCur = mt.start, mt.end, true
		}
	}
	return highlightSearch(line, m.query, curStart, curEnd, hasCur)
}

func highlightSearch(line, query string, curStart, curEnd int, hasCur bool) string {
	q := lowerRunes(query)
	if len(q) == 0 {
		return line
	}
	runes := []rune(line)
	spans := findSpans(lowerRunes(line), q)
	if len(spans) == 0 {
		return line
	}
	var b strings.Builder
	last := 0
	for _, sp := range spans {
		b.WriteString(string(runes[last:sp[0]]))
		style := matchStyle
		if hasCur && sp[0] == curStart && sp[1] == curEnd {
			style = currentMatchStyle
		}
		b.WriteString(style.Render(string(runes[sp[0]:sp[1]])))
		last = sp[1]
	}
	b.WriteString(string(runes[last:]))
	return b.String()
}

func findMatches(lines []string, query string) []searchMatch {
	q := lowerRunes(query)
	if len(q) == 0 {
		return nil
	}
	var out []searchMatch
	for i, line := range lines {
		for _, sp := range findSpans(lowerRunes(line), q) {
			out = append(out, searchMatch{line: i, start: sp[0], end: sp[1]})
		}
	}
	return out
}

func findSpans(hay, needle []rune) [][2]int {
	if len(needle) == 0 || len(hay) < len(needle) {
		return nil
	}
	var out [][2]int
	for i := 0; i+len(needle) <= len(hay); i++ {
		match := true
		for j := range needle {
			if hay[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			out = append(out, [2]int{i, i + len(needle)})
			i += len(needle) - 1
		}
	}
	return out
}

func lowerRunes(s string) []rune {
	r := []rune(s)
	for i := range r {
		r[i] = unicode.ToLower(r[i])
	}
	return r
}
