package viewer

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"

	"github.com/lakshwin/bento/internal/theme"
)

// StartEditing switches the pane into text-entry mode. It fails when there is
// nothing editable (no file, binary, oversized or still loading).
func (m *Model) StartEditing() bool {
	if m.path == "" || m.binary || m.tooLarge || m.loading || len(m.lines) == 0 {
		return false
	}
	m.searching = false
	m.clearSearch()
	m.editing = true
	m.collapsed = map[int]bool{}
	m.rebuildDisplay()
	m.cursorCol = clampInt(m.cursorCol, 0, m.lineLen(m.cursor))
	m.clamp()
	return true
}

// Editing reports whether the pane is capturing text input.
func (m *Model) Editing() bool { return m.editing }

// Dirty reports whether the buffer has unsaved changes.
func (m *Model) Dirty() bool { return m.dirty }

// Save returns a command that writes the buffer, or nil when there is nothing
// to save. The file is only written off the UI goroutine.
func (m *Model) Save() tea.Cmd {
	if m.path == "" || m.binary || m.tooLarge || !m.dirty {
		return nil
	}
	return SaveCmd(m.path, m.lines, m.eol, m.bom)
}

func (m *Model) lineLen(i int) int {
	if i < 0 || i >= len(m.lines) {
		return 0
	}
	return len([]rune(m.lines[i]))
}

func (m *Model) exitEditing() tea.Cmd {
	if !m.editing {
		return nil
	}
	m.editing = false
	m.regions = computeFoldRegions(m.lines, m.path)
	m.rebuildDisplay()
	m.clamp()
	m.gen++
	return HighlightCmd(m.path, m.lines, m.gen)
}

func (m *Model) handleEditKey(msg tea.KeyMsg) tea.Cmd {
	page := m.innerHeight()
	if page < 1 {
		page = 1
	}
	switch msg.String() {
	case "esc":
		return m.exitEditing()
	case "ctrl+s":
		return m.Save()
	case "up":
		m.moveLine(-1)
	case "down":
		m.moveLine(1)
	case "pgup":
		m.moveLine(-page)
	case "pgdown":
		m.moveLine(page)
	case "home":
		m.cursorCol = 0
	case "end":
		m.cursorCol = m.lineLen(m.cursor)
	case "left":
		m.cursorCol--
	case "right":
		m.cursorCol++
	case "ctrl+left":
		m.cursorCol = wordLeft([]rune(m.lines[m.cursor]), m.cursorCol)
	case "ctrl+right":
		m.cursorCol = wordRight([]rune(m.lines[m.cursor]), m.cursorCol)
	case "backspace":
		m.backspace()
	case "delete":
		m.deleteForward()
	case "enter":
		m.insertNewline()
	case "tab":
		m.insertIndent()
	default:
		switch msg.Type {
		case tea.KeyRunes:
			m.insertText(string(msg.Runes))
		case tea.KeySpace:
			m.insertText(" ")
		}
	}
	m.clamp()
	if m.styled == nil {
		return HighlightCmd(m.path, m.lines, m.gen)
	}
	return nil
}

func (m *Model) moveLine(delta int) {
	if len(m.lines) == 0 {
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.lines) {
		m.cursor = len(m.lines) - 1
	}
	m.cursorCol = clampInt(m.cursorCol, 0, m.lineLen(m.cursor))
}

func (m *Model) insertText(s string) {
	if len(m.lines) == 0 {
		m.lines = []string{""}
	}
	runes := []rune(m.lines[m.cursor])
	col := clampInt(m.cursorCol, 0, len(runes))
	ins := []rune(s)
	out := make([]rune, 0, len(runes)+len(ins))
	out = append(out, runes[:col]...)
	out = append(out, ins...)
	out = append(out, runes[col:]...)
	m.lines[m.cursor] = string(out)
	m.cursorCol = col + len(ins)
	m.afterEdit()
}

func (m *Model) insertIndent() {
	if len(m.lines) == 0 {
		m.lines = []string{""}
	}
	col := clampInt(m.cursorCol, 0, m.lineLen(m.cursor))
	n := 4 - col%4
	m.insertText(strings.Repeat(" ", n))
}

func (m *Model) insertNewline() {
	if len(m.lines) == 0 {
		m.lines = []string{""}
	}
	runes := []rune(m.lines[m.cursor])
	col := clampInt(m.cursorCol, 0, len(runes))
	indent := ""
	if col > 0 {
		lead := string(runes[:col])
		if trimmed := strings.TrimLeft(lead, " \t"); trimmed == "" {
			indent = lead
		}
	}
	head := string(runes[:col])
	tail := indent + string(runes[col:])
	m.lines[m.cursor] = head
	m.lines = append(m.lines, "")
	copy(m.lines[m.cursor+2:], m.lines[m.cursor+1:])
	m.lines[m.cursor+1] = tail
	m.cursor++
	m.cursorCol = len([]rune(indent))
	m.afterEdit()
}

func (m *Model) backspace() {
	if len(m.lines) == 0 {
		return
	}
	runes := []rune(m.lines[m.cursor])
	col := clampInt(m.cursorCol, 0, len(runes))
	if col > 0 {
		m.lines[m.cursor] = string(runes[:col-1]) + string(runes[col:])
		m.cursorCol = col - 1
		m.afterEdit()
		return
	}
	if m.cursor == 0 {
		return
	}
	prev := []rune(m.lines[m.cursor-1])
	joinCol := len(prev)
	m.lines[m.cursor-1] = m.lines[m.cursor-1] + m.lines[m.cursor]
	m.lines = append(m.lines[:m.cursor], m.lines[m.cursor+1:]...)
	m.cursor--
	m.cursorCol = joinCol
	m.afterEdit()
}

func (m *Model) deleteForward() {
	if len(m.lines) == 0 {
		return
	}
	runes := []rune(m.lines[m.cursor])
	col := clampInt(m.cursorCol, 0, len(runes))
	if col < len(runes) {
		m.lines[m.cursor] = string(runes[:col]) + string(runes[col+1:])
		m.afterEdit()
		return
	}
	if m.cursor >= len(m.lines)-1 {
		return
	}
	m.lines[m.cursor] = m.lines[m.cursor] + m.lines[m.cursor+1]
	m.lines = append(m.lines[:m.cursor+1], m.lines[m.cursor+2:]...)
	m.afterEdit()
}

func (m *Model) afterEdit() {
	m.dirty = true
	m.styled = nil
	m.gen++
	m.rebuildDisplay()
}

// editCursorLine renders one buffer line with the edit cursor highlighted.
func (m *Model) editCursorLine(i int) string {
	line := m.lines[i]
	if i != m.cursor {
		return line
	}
	runes := []rune(line)
	col := clampInt(m.cursorCol, 0, len(runes))
	prefix := string(runes[:col])
	rest := runes[col:]
	cur := " "
	if len(rest) > 0 {
		cur = string(rest[0])
		rest = rest[1:]
		if cur == "\t" {
			cur = " "
		}
	}
	return prefix + theme.CursorStyle().Render(cur) + string(rest)
}

// textWidth is the number of body columns available to the text (excluding the
// gutter), mirroring View/renderLines.
func (m *Model) textWidth() int {
	innerW := m.width - 2
	if innerW < 1 {
		return 1
	}
	digits := len(strconv.Itoa(max(1, len(m.lines))))
	tw := innerW - (digits + 3)
	if tw < 1 {
		tw = innerW
	}
	return tw
}

func (m *Model) ensureEditVisible() {
	if !m.editing || len(m.lines) == 0 {
		return
	}
	runes := []rune(m.lines[m.cursor])
	col := clampInt(m.cursorCol, 0, len(runes))
	pos := runewidth.StringWidth(string(runes[:col]))
	tw := m.textWidth()
	if pos < m.hscroll {
		m.hscroll = pos
	}
	if pos >= m.hscroll+tw {
		m.hscroll = pos - tw + 1
	}
}

func wordLeft(runes []rune, col int) int {
	col = clampInt(col, 0, len(runes))
	for col > 0 && runes[col-1] == ' ' {
		col--
	}
	for col > 0 && runes[col-1] != ' ' {
		col--
	}
	return col
}

func wordRight(runes []rune, col int) int {
	col = clampInt(col, 0, len(runes))
	for col < len(runes) && runes[col] == ' ' {
		col++
	}
	for col < len(runes) && runes[col] != ' ' {
		col++
	}
	return col
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
