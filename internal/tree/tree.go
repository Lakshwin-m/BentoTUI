package tree

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lakshwin/bento/internal/ignore"
	"github.com/lakshwin/bento/internal/theme"
	"github.com/lakshwin/bento/internal/ui"
	"github.com/lakshwin/bento/internal/winutil"
)

// DirLoadedMsg carries the result of an asynchronous directory read.
type DirLoadedMsg struct {
	Path    string
	Entries []Entry
	Err     error
}

// OpenFileMsg asks the root model to open a file in the viewer.
type OpenFileMsg struct{ Path string }

// Entry is one directory entry as reported by the loader.
type Entry struct {
	Name   string
	Path   string
	IsDir  bool
	Hidden bool
}

type node struct {
	name     string
	path     string
	dir      bool
	hidden   bool
	status   byte
	loaded   bool
	expanded bool
	loading  bool
	err      error
	children []*node
}

type row struct {
	node  *node
	depth int
}

// Model is the file tree pane.
type Model struct {
	root string

	width  int
	height int

	showHidden bool
	ignoreOn   bool
	ignore     *ignore.Matcher

	rootNode *node
	index    map[string]*node
	flat     []row

	sel    int
	scroll int
	errMsg string
}

// New creates a tree for root. It loads lazily: only the root is read in Init.
func New(root string) *Model {
	m := &Model{
		root:     root,
		ignoreOn: true,
		index:    map[string]*node{},
	}
	m.ignore = ignore.Load(root)
	m.rootNode = &node{path: root, dir: true, loaded: false}
	m.index[root] = m.rootNode
	return m
}

// Init implements ui.Pane.
func (m *Model) Init() tea.Cmd {
	if m.rootNode.loaded || m.rootNode.loading {
		return nil
	}
	m.rootNode.loading = true
	return m.loadCmd(m.root)
}

// Update implements ui.Pane.
func (m *Model) Update(msg tea.Msg) (ui.Pane, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.PaneSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clampView()
		return m, nil

	case DirLoadedMsg:
		n := m.index[msg.Path]
		if n == nil {
			return m, nil
		}
		n.loading = false
		if msg.Err != nil {
			n.err = msg.Err
			m.errMsg = msg.Err.Error()
			return m, nil
		}
		n.err = nil
		n.loaded = true
		n.children = n.children[:0]
		for _, e := range msg.Entries {
			child := &node{
				name:   e.Name,
				path:   e.Path,
				dir:    e.IsDir,
				hidden: e.Hidden,
			}
			n.children = append(n.children, child)
			m.index[child.path] = child
		}
		if n == m.rootNode {
			n.expanded = true
		}
		m.rebuild()
		m.clampView()
		return m, nil

	case tea.KeyMsg:
		return m, m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	page := m.innerHeight()
	if page < 1 {
		page = 1
	}
	switch msg.String() {
	case "up", "k":
		m.sel--
	case "down", "j":
		m.sel++
	case "pgup":
		m.sel -= page
	case "pgdown":
		m.sel += page
	case "home", "g":
		m.sel = 0
	case "end", "G":
		m.sel = len(m.flat) - 1
	case "enter":
		return m.activate()
	case "right", "l":
		if n := m.selected(); n != nil && n.dir {
			return m.expand(n)
		}
	case "left", "h":
		m.collapseOrParent()
	case ".":
		m.showHidden = !m.showHidden
		return m.reload()
	case "r":
		return m.reload()
	}
	m.clampView()
	return nil
}

func (m *Model) selected() *node {
	if m.sel < 0 || m.sel >= len(m.flat) {
		return nil
	}
	return m.flat[m.sel].node
}

func (m *Model) activate() tea.Cmd {
	n := m.selected()
	if n == nil {
		return nil
	}
	if n.dir {
		if n.expanded {
			n.expanded = false
			m.rebuild()
			m.clampView()
			return nil
		}
		return m.expand(n)
	}
	path := n.path
	return func() tea.Msg { return OpenFileMsg{Path: path} }
}

func (m *Model) expand(n *node) tea.Cmd {
	if !n.dir {
		return nil
	}
	if !n.loaded {
		if n.loading {
			return nil
		}
		n.loading = true
		n.expanded = true
		return m.loadCmd(n.path)
	}
	n.expanded = true
	m.rebuild()
	m.clampView()
	return nil
}

func (m *Model) collapseOrParent() {
	n := m.selected()
	if n == nil {
		return
	}
	if n.dir && n.expanded {
		n.expanded = false
		m.rebuild()
		m.clampView()
		return
	}
	parent := filepath.Dir(n.path)
	if parent == n.path || parent == m.root {
		return
	}
	for i, r := range m.flat {
		if r.node.path == parent {
			m.sel = i
			m.clampView()
			return
		}
	}
}

func (m *Model) reload() tea.Cmd {
	m.rootNode.children = nil
	m.rootNode.loaded = false
	m.rootNode.expanded = true
	m.index = map[string]*node{m.root: m.rootNode}
	m.flat = nil
	m.sel = 0
	m.scroll = 0
	m.errMsg = ""
	m.ignore = ignore.Load(m.root)
	m.rootNode.loading = true
	return m.loadCmd(m.root)
}

func (m *Model) loadCmd(path string) tea.Cmd {
	showHidden := m.showHidden
	ignoreOn := m.ignoreOn
	ig := m.ignore
	root := m.root
	return func() tea.Msg {
		entries, err := os.ReadDir(path)
		if err != nil {
			return DirLoadedMsg{Path: path, Err: err}
		}
		out := make([]Entry, 0, len(entries))
		for _, de := range entries {
			name := de.Name()
			if name == ".git" {
				continue
			}
			isDir := de.IsDir()
			if de.Type()&fs.ModeSymlink != 0 {
				// Avoid symlink cycles: show symlinks as plain files.
				isDir = false
			}
			hidden := strings.HasPrefix(name, ".") || winutil.HasHiddenAttr(de)
			if hidden && !showHidden {
				continue
			}
			full := filepath.Join(path, name)
			if ignoreOn && ig != nil {
				if rel, rerr := filepath.Rel(root, full); rerr == nil && ig.Match(rel, isDir) {
					continue
				}
			}
			out = append(out, Entry{Name: name, Path: full, IsDir: isDir, Hidden: hidden})
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].IsDir != out[j].IsDir {
				return out[i].IsDir
			}
			li, lj := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
			if li != lj {
				return li < lj
			}
			return out[i].Name < out[j].Name
		})
		return DirLoadedMsg{Path: path, Entries: out}
	}
}

func (m *Model) rebuild() {
	m.flat = m.flat[:0]
	var walk func(n *node, depth int)
	walk = func(n *node, depth int) {
		for _, c := range n.children {
			m.flat = append(m.flat, row{node: c, depth: depth})
			if c.dir && c.expanded && c.loaded {
				walk(c, depth+1)
			}
		}
	}
	walk(m.rootNode, 0)
}

func (m *Model) clampView() {
	if len(m.flat) == 0 {
		m.sel = 0
		m.scroll = 0
		return
	}
	if m.sel < 0 {
		m.sel = 0
	}
	if m.sel >= len(m.flat) {
		m.sel = len(m.flat) - 1
	}
	h := m.innerHeight()
	if h < 1 {
		h = 1
	}
	if m.sel < m.scroll {
		m.scroll = m.sel
	}
	if m.sel >= m.scroll+h {
		m.scroll = m.sel - h + 1
	}
	maxScroll := len(m.flat) - h
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

func (m *Model) innerHeight() int {
	if m.height < 2 {
		return 0
	}
	return m.height - 2
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
	lines := make([]string, 0, innerH)

	if m.errMsg != "" {
		lines = append(lines, theme.ErrorStyle().Render(ui.Truncate(m.errMsg, innerW)))
	}
	if !m.rootNode.loaded {
		lines = append(lines, theme.MutedStyle().Render("Loading…"))
	} else if len(m.flat) == 0 {
		lines = append(lines, theme.MutedStyle().Render("(empty)"))
	}

	visible := innerH - len(lines)
	if visible < 0 {
		visible = 0
	}
	end := m.scroll + visible
	if end > len(m.flat) {
		end = len(m.flat)
	}
	for i := m.scroll; i < end; i++ {
		selected := i == m.sel
		lines = append(lines, m.rowLine(m.flat[i], innerW, selected))
	}
	for len(lines) < innerH {
		lines = append(lines, ui.Fit("", innerW))
	}
	if len(lines) > innerH {
		lines = lines[:innerH]
	}
	return ui.Island("Tree", width, height, focused, strings.Join(lines, "\n"))
}

func (m *Model) rowLine(r row, width int, selected bool) string {
	n := r.node
	indent := strings.Repeat("  ", r.depth)
	glyph := " "
	if n.dir {
		if n.expanded {
			glyph = theme.TreeExpanded
		} else {
			glyph = theme.TreeCollapsed
		}
	}
	status := ""
	if n.status != 0 {
		status = " " + string(n.status)
	}
	base := indent + glyph + " "
	if selected {
		plain := base + n.name + status
		return theme.SelectedStyle().Render(ui.Fit(plain, width))
	}
	out := base
	if n.dir {
		out += theme.AccentStyle().Render(n.name)
	} else {
		out += theme.TextStyle().Render(n.name)
	}
	if n.status != 0 {
		out += lipglossColor(n.status, status)
	}
	if n.loading {
		out += theme.MutedStyle().Render(" …")
	}
	return ui.Fit(out, width)
}

func lipglossColor(status byte, text string) string {
	return theme.GitStatusStyle(status).Render(text)
}

func (m *Model) Title() string { return "Tree" }

func (m *Model) KeyHints() []ui.KeyHint {
	return []ui.KeyHint{
		{Key: "↑↓", Desc: "move"},
		{Key: "enter", Desc: "open"},
		{Key: "←→", Desc: "fold"},
		{Key: ".", Desc: "hidden"},
	}
}

var _ ui.Pane = (*Model)(nil)
