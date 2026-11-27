// Package git implements the Changes pane: an async view of the project's
// working tree status. It never runs git inside Update.
package git

import (
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lakshwin/bento/internal/theme"
	"github.com/lakshwin/bento/internal/ui"
	"github.com/lakshwin/bento/internal/winutil"
)

// Entry is one changed path from `git status --porcelain`.
type Entry struct {
	Index    byte
	Worktree byte
	Path     string
	Orig     string
}

// Status is one snapshot of the repository.
type Status struct {
	IsRepo  bool
	Branch  string
	Ahead   int
	Behind  int
	Entries []Entry
	Err     error
}

// LoadedMsg carries an async status result.
type LoadedMsg struct {
	Dir    string
	Status Status
}

// OpenMsg asks the root model to open a file in the editor pane.
type OpenMsg struct {
	Path string
}

// LoadCmd runs git status off the UI goroutine.
func LoadCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		return LoadedMsg{Dir: dir, Status: load(dir)}
	}
}

func load(dir string) Status {
	cmd := exec.Command("git", "-C", dir, "status", "--porcelain=v1", "-z", "--branch")
	winutil.HideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if isNotRepo(out, err) {
			return Status{}
		}
		if _, ok := err.(*exec.ExitError); ok {
			return Status{Err: err}
		}
		return Status{Err: err}
	}
	return parsePorcelain(string(out))
}

func isNotRepo(out []byte, err error) bool {
	if strings.Contains(strings.ToLower(string(out)), "not a git repository") {
		return true
	}
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 128 {
		return true
	}
	return false
}

func parsePorcelain(out string) Status {
	st := Status{IsRepo: true}
	if out == "" {
		return st
	}
	parts := strings.Split(out, "\x00")
	i := 0
	if len(parts) > 0 && strings.HasPrefix(parts[0], "## ") {
		parseHeader(parts[0], &st)
		i = 1
	}
	for ; i < len(parts); i++ {
		tok := parts[i]
		if len(tok) < 4 {
			continue
		}
		e := Entry{Index: tok[0], Worktree: tok[1], Path: tok[3:]}
		if e.Index == 'R' || e.Index == 'C' || e.Worktree == 'R' || e.Worktree == 'C' {
			if i+1 < len(parts) {
				e.Orig = parts[i+1]
				i++
			}
		}
		st.Entries = append(st.Entries, e)
	}
	sort.Slice(st.Entries, func(a, b int) bool {
		return strings.ToLower(st.Entries[a].Path) < strings.ToLower(st.Entries[b].Path)
	})
	return st
}

func parseHeader(tok string, st *Status) {
	h := strings.TrimPrefix(tok, "## ")
	if open := strings.IndexByte(h, '['); open >= 0 {
		if close := strings.IndexByte(h[open:], ']'); close > 0 {
			inside := h[open+1 : open+close]
			st.Ahead = parseCount(inside, "ahead ")
			st.Behind = parseCount(inside, "behind ")
			h = strings.TrimSpace(h[:open])
		}
	}
	if i := strings.Index(h, "..."); i >= 0 {
		h = h[:i]
	}
	switch {
	case strings.HasPrefix(h, "No commits yet on "):
		h = strings.TrimPrefix(h, "No commits yet on ")
	case strings.HasPrefix(h, "HEAD (no branch)"):
		h = "detached"
	}
	st.Branch = h
}

func parseCount(s, key string) int {
	i := strings.Index(s, key)
	if i < 0 {
		return 0
	}
	rest := s[i+len(key):]
	j := 0
	for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
		j++
	}
	n, _ := strconv.Atoi(rest[:j])
	return n
}

// Model is the Changes pane.
type Model struct {
	dir     string
	status  Status
	loading bool

	cursor int
	scroll int
	width  int
	height int
}

// New creates the pane for a project directory.
func New(dir string) *Model {
	return &Model{dir: dir, loading: true}
}

// Init implements ui.Pane.
func (m *Model) Init() tea.Cmd { return LoadCmd(m.dir) }

// Update implements ui.Pane.
func (m *Model) Update(msg tea.Msg) (ui.Pane, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.PaneSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clamp()
		return m, nil
	case LoadedMsg:
		if msg.Dir != m.dir {
			return m, nil
		}
		m.status = msg.Status
		m.loading = false
		m.cursor = clampInt(m.cursor, 0, max(0, len(m.status.Entries)-1))
		m.clamp()
		return m, nil
	case tea.KeyMsg:
		return m, m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "r":
		if !m.loading {
			m.loading = true
			return LoadCmd(m.dir)
		}
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.status.Entries)-1 {
			m.cursor++
		}
	case "g", "home":
		m.cursor = 0
	case "G", "end":
		m.cursor = max(0, len(m.status.Entries)-1)
	case "enter", "o":
		if m.cursor < len(m.status.Entries) {
			path := filepath.Join(m.dir, filepath.FromSlash(m.status.Entries[m.cursor].Path))
			return func() tea.Msg { return OpenMsg{Path: path} }
		}
	}
	m.clamp()
	return nil
}

func (m *Model) rowsHeight() int {
	h := m.height - ui.ChromeRows - 2 // chrome + branch header + spacer
	if h < 1 {
		return 1
	}
	return h
}

func (m *Model) clamp() {
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.status.Entries) && len(m.status.Entries) > 0 {
		m.cursor = len(m.status.Entries) - 1
	}
	h := m.rowsHeight()
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+h {
		m.scroll = m.cursor - h + 1
	}
	maxScroll := len(m.status.Entries) - h
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

// View implements ui.Pane.
func (m *Model) View(width, height int, focused bool) string {
	innerW, innerH := ui.BodySize(width, height)

	var body string
	switch {
	case m.loading && len(m.status.Entries) == 0:
		body = ui.CenteredCard(innerW, innerH, "Reading git status…", "")
	case m.status.Err != nil:
		body = ui.CenteredCard(innerW, innerH, "Git unavailable", m.status.Err.Error())
	case !m.status.IsRepo:
		body = ui.CenteredCard(innerW, innerH, "Not a git repository", "Changes appear here once the project is initialized.")
	default:
		var b strings.Builder
		b.WriteString(ui.Fit(m.headerLine(innerW), innerW))
		b.WriteString("\n")
		b.WriteString(ui.Fit("", innerW))
		if len(m.status.Entries) == 0 {
			b.WriteString("\n")
			b.WriteString(ui.Fit(theme.MutedStyle().Render("Working tree clean"), innerW))
			body = b.String()
			break
		}
		rows := m.rowsHeight()
		end := m.scroll + rows
		if end > len(m.status.Entries) {
			end = len(m.status.Entries)
		}
		for i := m.scroll; i < end; i++ {
			b.WriteString("\n")
			b.WriteString(m.entryLine(i, innerW, focused))
		}
		body = b.String()
	}
	return ui.Panel(ui.Spec(m.Title(), m.KeyHints()), width, height, focused, body)
}

func (m *Model) headerLine(width int) string {
	branch := m.status.Branch
	if branch == "" {
		branch = "HEAD"
	}
	count := len(m.status.Entries)
	left := theme.AccentStyle().Render(branch) + theme.MutedStyle().Render(theme.Sep+plural(count, "change"))
	if m.status.Ahead > 0 || m.status.Behind > 0 {
		var bits []string
		if m.status.Ahead > 0 {
			bits = append(bits, "ahead "+strconv.Itoa(m.status.Ahead))
		}
		if m.status.Behind > 0 {
			bits = append(bits, "behind "+strconv.Itoa(m.status.Behind))
		}
		left += theme.MutedStyle().Render(theme.Sep + strings.Join(bits, ", "))
	}
	return ui.Fit(left, width)
}

func (m *Model) entryLine(i, width int, focused bool) string {
	e := m.status.Entries[i]
	letter := e.Worktree
	if letter == ' ' || letter == '?' {
		letter = e.Index
	}
	code := theme.GitStatusStyle(letter).Render(string(e.Index) + string(e.Worktree))
	path := e.Path
	if e.Orig != "" {
		path = filepath.ToSlash(e.Orig) + " → " + path
	}
	shown := truncatePlain(path, max(1, width-3))
	var text string
	switch {
	case i == m.cursor && focused:
		text = code + " " + theme.SelectedRowStyle(true).Render(shown)
	case e.Index == '?' && e.Worktree == '?':
		text = code + " " + theme.MutedStyle().Render(shown)
	default:
		text = code + " " + theme.TextStyle().Render(shown)
	}
	return ui.Fit(text, width)
}

// Branch is the current git branch, or empty when the directory is not a repo.
func (m *Model) Branch() string {
	if !m.status.IsRepo {
		return ""
	}
	return m.status.Branch
}

// Dirty reports whether the working tree has changes.
func (m *Model) Dirty() bool {
	return m.status.IsRepo && len(m.status.Entries) > 0
}

// Title implements ui.Pane.
func (m *Model) Title() string {
	n := len(m.status.Entries)
	if n == 0 {
		return "Changes"
	}
	return "Changes " + strconv.Itoa(n)
}

// KeyHints implements ui.Pane.
func (m *Model) KeyHints() []ui.KeyHint {
	return []ui.KeyHint{
		{Key: "↑↓", Desc: "select"},
		{Key: "enter", Desc: "open"},
		{Key: "r", Desc: "refresh"},
	}
}

func truncatePlain(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	if width == 1 {
		return theme.Ellipsis
	}
	return string(r[:width-1]) + theme.Ellipsis
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
