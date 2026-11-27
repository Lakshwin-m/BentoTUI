// Package palette implements Bento's keyboard command palette overlay.
package palette

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lakshwin/bento/internal/ui"
)

// Command is one runnable entry in the palette.
type Command struct {
	ID          string
	Title       string
	Description string
	Keys        string
}

// RunMsg asks the root model to run a palette command.
type RunMsg struct{ ID string }

// Model is the ctrl+k command palette overlay.
type Model struct {
	opened   bool
	commands []Command
	results  []Command

	input  string
	sel    int
	scroll int

	width  int
	height int
}

// New creates an empty palette. Commands are supplied when it opens.
func New() *Model { return &Model{} }

// Opened reports whether the overlay is active.
func (m *Model) Opened() bool { return m.opened }

// Open resets the query and filters the supplied commands.
func (m *Model) Open(commands []Command) tea.Cmd {
	m.opened = true
	m.commands = append([]Command(nil), commands...)
	m.input = ""
	m.sel = 0
	m.scroll = 0
	m.refilter()
	return nil
}

// Close hides the overlay.
func (m *Model) Close() { m.opened = false }

// Update implements the overlay message loop.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case ui.PaneSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clamp()
		return nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return nil
}

func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "ctrl+k", "ctrl+c":
		m.opened = false
		return nil
	case "enter":
		if m.sel >= 0 && m.sel < len(m.results) {
			id := m.results[m.sel].ID
			m.opened = false
			return func() tea.Msg { return RunMsg{ID: id} }
		}
		return nil
	case "up", "ctrl+p":
		m.sel--
	case "down", "ctrl+n":
		m.sel++
	case "pgup":
		m.sel -= m.listHeight()
	case "pgdown":
		m.sel += m.listHeight()
	case "home":
		m.sel = 0
	case "end":
		m.sel = len(m.results) - 1
	case "backspace":
		r := []rune(m.input)
		if len(r) > 0 {
			m.input = string(r[:len(r)-1])
			m.refilter()
		}
	case "ctrl+u":
		m.input = ""
		m.refilter()
	default:
		switch msg.Type {
		case tea.KeyRunes:
			m.input += string(msg.Runes)
			m.refilter()
		case tea.KeySpace:
			m.input += " "
			m.refilter()
		}
	}
	m.clamp()
	return nil
}

func (m *Model) listHeight() int {
	if m.height < 6 {
		return 1
	}
	return m.height - 4
}

func (m *Model) clamp() {
	if len(m.results) == 0 {
		m.sel = 0
		m.scroll = 0
		return
	}
	if m.sel < 0 {
		m.sel = 0
	}
	if m.sel >= len(m.results) {
		m.sel = len(m.results) - 1
	}
	h := m.listHeight()
	if h < 1 {
		h = 1
	}
	if m.sel < m.scroll {
		m.scroll = m.sel
	}
	if m.sel >= m.scroll+h {
		m.scroll = m.sel - h + 1
	}
	maxScroll := len(m.results) - h
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scroll > maxScroll {
		m.scroll = maxScroll
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
}

func (m *Model) refilter() {
	m.sel = 0
	m.scroll = 0
	query := strings.ToLower(strings.TrimSpace(m.input))
	tokens := strings.Fields(query)
	results := make([]Command, 0, len(m.commands))
	for _, c := range m.commands {
		haystack := strings.ToLower(c.Title + " " + c.Description + " " + c.Keys)
		match := true
		for _, token := range tokens {
			if !strings.Contains(haystack, token) {
				match = false
				break
			}
		}
		if match {
			results = append(results, c)
		}
	}
	if query != "" {
		sort.SliceStable(results, func(i, j int) bool {
			return rank(results[i], query) > rank(results[j], query)
		})
	}
	m.results = results
}

func rank(c Command, query string) int {
	title := strings.ToLower(c.Title)
	switch {
	case strings.HasPrefix(title, query):
		return 3
	case strings.Contains(title, query):
		return 2
	default:
		return 1
	}
}
