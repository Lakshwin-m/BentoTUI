// Package viewer implements the read-only preview pane with syntax
// highlighting, line numbers, scrolling and horizontal offset.
package viewer

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/lakshwin/bento/internal/theme"
	"github.com/lakshwin/bento/internal/ui"
)

// Model is the preview pane. Loading happens in commands; Update never reads
// files or highlights.
type Model struct {
	path   string
	lines  []string
	styled []string

	binary   bool
	tooLarge bool
	loading  bool
	size     int64
	errMsg   string

	width  int
	height int

	scroll    int
	cursor    int
	cursorCol int
	hscroll   int

	editing bool
	dirty   bool
	gen     int
	eol     string
	bom     bool

	pendingLine int
	pendingCol  int

	searching bool
	query     string
	matches   []searchMatch
	matchIdx  int

	regions   []FoldRegion
	collapsed map[int]bool
	display   []displayRow
}

// New creates an empty viewer.
func New() *Model { return &Model{pendingLine: -1} }

// Path returns the currently open file, if any.
func (m *Model) Path() string { return m.path }

// Cursor returns the cursor line and rune column.
func (m *Model) Cursor() (int, int) { return m.cursor, m.cursorCol }

// RestoreCursor applies a saved cursor position once the file has loaded.
func (m *Model) RestoreCursor(line, col int) {
	m.pendingLine = line
	m.pendingCol = col
	if !m.loading && len(m.lines) > 0 {
		m.applyPendingCursor()
		m.clamp()
	}
}

func (m *Model) applyPendingCursor() {
	if m.pendingLine < 0 {
		return
	}
	m.cursor = m.pendingLine
	m.cursorCol = m.pendingCol
	if len(m.lines) > 0 {
		if m.cursor >= len(m.lines) {
			m.cursor = len(m.lines) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.cursorCol = clampInt(m.cursorCol, 0, m.lineLen(m.cursor))
	}
	m.pendingLine = -1
	m.pendingCol = 0
}

// Init implements ui.Pane.
func (m *Model) Init() tea.Cmd { return nil }

// Open starts an asynchronous load of path.
func (m *Model) Open(path string) tea.Cmd {
	m.path = path
	m.loading = true
	m.lines = nil
	m.styled = nil
	m.binary = false
	m.tooLarge = false
	m.errMsg = ""
	m.scroll = 0
	m.cursor = 0
	m.cursorCol = 0
	m.pendingLine = -1
	m.pendingCol = 0
	m.hscroll = 0
	m.editing = false
	m.dirty = false
	m.gen++
	m.searching = false
	m.clearSearch()
	m.regions = nil
	m.collapsed = map[int]bool{}
	m.display = nil
	return LoadCmd(path, false)
}

// Update implements ui.Pane.
func (m *Model) Update(msg tea.Msg) (ui.Pane, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.PaneSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clamp()
		return m, nil

	case FileLoadedMsg:
		if msg.Path != m.path {
			return m, nil
		}
		m.loading = false
		if msg.Err != nil {
			m.errMsg = msg.Err.Error()
			return m, nil
		}
		if msg.TooLarge {
			m.tooLarge = true
			m.size = msg.Size
			m.lines = nil
			m.styled = nil
			m.regions = nil
			m.display = nil
			return m, nil
		}
		if msg.Binary {
			m.binary = true
			m.size = msg.Size
			m.lines = nil
			m.styled = nil
			m.regions = nil
			m.display = nil
			return m, nil
		}
		m.tooLarge = false
		m.binary = false
		m.errMsg = ""
		m.size = msg.Size
		m.lines = msg.Lines
		m.styled = msg.Styled
		m.regions = msg.Regions
		m.eol = msg.EOL
		m.bom = msg.BOM
		m.editing = false
		m.dirty = false
		m.gen++
		m.collapsed = map[int]bool{}
		m.rebuildDisplay()
		m.scroll = 0
		m.cursor = 0
		m.cursorCol = 0
		m.hscroll = 0
		m.applyPendingCursor()
		m.clamp()
		return m, nil

	case HighlightedMsg:
		if msg.Path == m.path && msg.Gen == m.gen {
			m.styled = msg.Styled
		}
		return m, nil

	case SavedMsg:
		if msg.Path == m.path && msg.Err == nil {
			m.dirty = false
		}
		return m, nil

	case tea.KeyMsg:
		if m.searching {
			return m, m.handleSearchKey(msg)
		}
		if m.editing {
			return m, m.handleEditKey(msg)
		}
		return m, m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	if m.tooLarge && msg.String() == "l" {
		m.loading = true
		return LoadCmd(m.path, true)
	}
	page := m.innerHeight()
	if page < 1 {
		page = 1
	}
	switch msg.String() {
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case "pgup":
		m.moveCursor(-page)
	case "pgdown":
		m.moveCursor(page)
	case "home", "g":
		if len(m.display) > 0 {
			m.cursor = m.display[0].line
		}
	case "end", "G":
		if len(m.display) > 0 {
			m.cursor = m.display[len(m.display)-1].line
		}
	case "left", "h":
		m.hscroll -= 4
	case "right", "L":
		m.hscroll += 4
	case "0":
		m.hscroll = 0
	case "e", "i":
		m.StartEditing()
	case "ctrl+s":
		return m.Save()
	case "z":
		m.toggleFold()
	case "Z":
		m.toggleAllFolds()
	case "n":
		if len(m.matches) > 0 {
			m.jumpTo(m.matchIdx + 1)
		}
	case "N":
		if len(m.matches) > 0 {
			m.jumpTo(m.matchIdx - 1)
		}
	case "esc":
		if m.query != "" {
			m.clearSearch()
		}
	}
	m.clamp()
	return nil
}

func (m *Model) innerHeight() int {
	if m.height < 2 {
		return 0
	}
	return m.height - 2
}

func (m *Model) clamp() {
	if m.hscroll < 0 {
		m.hscroll = 0
	}
	if len(m.lines) == 0 {
		m.cursor = 0
		m.scroll = 0
		return
	}
	if len(m.display) == 0 {
		m.rebuildDisplay()
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.lines) {
		m.cursor = len(m.lines) - 1
	}
	idx := m.displayIndex(m.cursor)
	if idx < 0 {
		// The cursor was folded away: snap to the nearest visible line at or
		// before it.
		idx = 0
		for i, row := range m.display {
			if row.line <= m.cursor {
				idx = i
			} else {
				break
			}
		}
		if len(m.display) > 0 {
			m.cursor = m.display[idx].line
		}
	}
	h := m.innerHeight()
	if h < 1 {
		h = 1
	}
	if idx < m.scroll {
		m.scroll = idx
	}
	if idx >= m.scroll+h {
		m.scroll = idx - h + 1
	}
	maxScroll := len(m.display) - h
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scroll > maxScroll {
		m.scroll = maxScroll
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
	m.ensureEditVisible()
}

// View implements ui.Pane.
func (m *Model) View(width, height int, focused bool) string {
	innerW := width - 2
	innerH := height - 2
	if innerW < 0 {
		innerW = 0
	}
	if innerH < 0 {
		innerH = 0
	}

	var body string
	switch {
	case m.errMsg != "":
		body = ui.CenteredCard(innerW, innerH, "Cannot open file", m.errMsg)
	case m.loading && len(m.lines) == 0:
		body = ui.CenteredCard(innerW, innerH, "Loading…", "")
	case m.binary:
		body = ui.CenteredCard(innerW, innerH, "Binary file", humanSize(m.size))
	case m.tooLarge:
		body = ui.CenteredCard(innerW, innerH, humanSize(m.size)+" file", "Press l to load anyway.")
	case m.path == "":
		body = ui.CenteredCard(innerW, innerH, "No file open", "Select a file in the Tree pane and press enter.")
	case len(m.lines) == 0:
		body = ui.CenteredCard(innerW, innerH, "(empty file)", "")
	default:
		searchLines := 0
		if m.hasSearchUI() {
			searchLines = 1
		}
		body = m.renderLines(innerW, innerH-searchLines, focused)
		if searchLines == 1 {
			body = ui.Fit(m.searchPrompt(innerW), innerW) + "\n" + body
		}
	}
	return ui.Island(m.Title(), width, height, focused, body)
}

func (m *Model) renderLines(innerW, innerH int, focused bool) string {
	digits := len(strconv.Itoa(max(1, len(m.lines))))
	gutterW := digits + 3
	textW := innerW - gutterW
	showGutter := true
	if textW < 1 {
		showGutter = false
		textW = innerW
	}

	out := make([]string, 0, innerH)
	end := m.scroll + innerH
	if end > len(m.display) {
		end = len(m.display)
	}
	for di := m.scroll; di < end; di++ {
		row := m.display[di]
		i := row.line
		line := m.plainOrStyled(i)
		if m.editing {
			line = m.editCursorLine(i)
		}
		if m.query != "" {
			line = m.searchLine(i)
		}
		seg := ansi.Cut(line, m.hscroll, m.hscroll+max(textW, 0))
		if row.hidden > 0 {
			seg += theme.AccentStyle().Render(fmt.Sprintf(" %s %d lines", theme.Ellipsis, row.hidden))
		}
		gutter := ""
		if showGutter {
			num := fmt.Sprintf("%*d", digits, i+1)
			marker, isFold := m.foldMarker(i)
			style := theme.MutedStyle()
			if isFold {
				style = theme.AccentStyle()
			} else {
				marker = " "
			}
			if i == m.cursor && focused {
				style = theme.AccentStyle().Bold(true)
				if !isFold {
					marker = theme.TreeCollapsed
				}
			}
			if m.editing && i == m.cursor {
				marker = theme.Cursor
				style = theme.AccentStyle().Bold(true)
			}
			gutter = style.Render(num+marker+theme.Bar) + " "
		}
		out = append(out, ui.Fit(gutter+seg, innerW))
	}
	for len(out) < innerH {
		out = append(out, ui.Fit("", innerW))
	}
	if len(out) > innerH {
		out = out[:innerH]
	}
	return strings.Join(out, "\n")
}

func (m *Model) plainOrStyled(i int) string {
	if i < len(m.styled) {
		return m.styled[i]
	}
	if i < len(m.lines) {
		return m.lines[i]
	}
	return ""
}

// Title implements ui.Pane.
func (m *Model) Title() string {
	if m.path == "" {
		return "Preview"
	}
	title := filepath.Base(m.path)
	if m.dirty {
		title += " " + theme.Dirty
	}
	return title
}

// KeyHints implements ui.Pane.
func (m *Model) KeyHints() []ui.KeyHint {
	if m.editing {
		hints := []ui.KeyHint{
			{Key: "type", Desc: "edit"},
			{Key: "enter", Desc: "newline"},
			{Key: "tab", Desc: "indent"},
		}
		if m.dirty {
			hints = append(hints, ui.KeyHint{Key: "ctrl+s", Desc: "save"})
		}
		return append(hints, ui.KeyHint{Key: "esc", Desc: "preview"})
	}
	if m.searching {
		return []ui.KeyHint{
			{Key: "type", Desc: "search"},
			{Key: "enter", Desc: "keep highlights"},
			{Key: "esc", Desc: "clear"},
		}
	}
	hints := []ui.KeyHint{
		{Key: "↑↓", Desc: "scroll"},
		{Key: "ctrl+f", Desc: "search"},
	}
	if m.query != "" {
		hints = append(hints, ui.KeyHint{Key: "n/N", Desc: "match"})
	}
	hints = append(hints,
		ui.KeyHint{Key: "g/G", Desc: "ends"},
		ui.KeyHint{Key: "h/L", Desc: "horiz"},
	)
	if len(m.lines) > 0 && !m.binary && !m.tooLarge {
		hints = append(hints, ui.KeyHint{Key: "e", Desc: "edit"})
	}
	if m.dirty {
		hints = append(hints, ui.KeyHint{Key: "ctrl+s", Desc: "save"})
	}
	if len(m.regions) > 0 {
		hints = append(hints, ui.KeyHint{Key: "z/Z", Desc: "fold"})
	}
	return hints
}

func humanSize(n int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case n >= gb:
		return fmt.Sprintf("%.1f GB", float64(n)/gb)
	case n >= mb:
		return fmt.Sprintf("%.1f MB", float64(n)/mb)
	case n >= kb:
		return fmt.Sprintf("%.1f KB", float64(n)/kb)
	default:
		return fmt.Sprintf("%d bytes", n)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

var _ ui.Pane = (*Model)(nil)
