package finder

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lakshwin/bento/internal/ignore"
	"github.com/lakshwin/bento/internal/ui"
	"github.com/lakshwin/bento/internal/winutil"
)

// IndexedMsg carries the cached file list after a background walk.
type IndexedMsg struct {
	Files []string
	Err   error
}

// OpenMsg asks the root model to open a file chosen in the finder.
type OpenMsg struct{ Path string }

// Model is the ctrl+p fuzzy file finder overlay.
type Model struct {
	root    string
	matcher matcher

	files   []string
	indexed bool
	loading bool
	errMsg  string

	opened bool
	input  string

	results []result
	sel     int
	scroll  int

	width  int
	height int
}

type matcher interface {
	Match(rel string, isDir bool) bool
}

type result struct {
	path    string
	score   int
	indices []int
}

const maxResults = 200

// New creates a finder for root and loads the ignore rules.
func New(root string) *Model {
	return &Model{root: root, matcher: newMatcher(root)}
}

func newMatcher(root string) matcher { return ignore.Load(root) }

// Opened reports whether the overlay is active.
func (m *Model) Opened() bool { return m.opened }

// Open resets the query and starts indexing if needed.
func (m *Model) Open() tea.Cmd {
	m.opened = true
	m.input = ""
	m.errMsg = ""
	m.sel = 0
	m.scroll = 0
	if m.indexed {
		m.refilter()
		return nil
	}
	m.results = nil
	return m.ensureIndex()
}

// Close hides the overlay.
func (m *Model) Close() { m.opened = false }

// Update implements the overlay message loop.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case IndexedMsg:
		m.loading = false
		if msg.Err != nil {
			m.errMsg = msg.Err.Error()
			return nil
		}
		m.files = msg.Files
		m.indexed = true
		m.refilter()
		return nil

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
	case "esc", "ctrl+c", "ctrl+p":
		m.opened = false
		return nil
	case "enter":
		if m.sel >= 0 && m.sel < len(m.results) {
			rel := m.results[m.sel].path
			m.opened = false
			full := filepath.Join(m.root, filepath.FromSlash(rel))
			return func() tea.Msg { return OpenMsg{Path: full} }
		}
		return nil
	case "up", "ctrl+k":
		m.sel--
	case "down", "ctrl+j":
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
	if len(m.files) == 0 {
		m.results = nil
		return
	}
	if m.input == "" {
		files := append([]string(nil), m.files...)
		sort.Slice(files, func(i, j int) bool {
			bi := strings.ToLower(filepath.Base(files[i]))
			bj := strings.ToLower(filepath.Base(files[j]))
			if bi != bj {
				return bi < bj
			}
			return files[i] < files[j]
		})
		n := len(files)
		if n > maxResults {
			n = maxResults
		}
		m.results = make([]result, 0, n)
		for i := 0; i < n; i++ {
			m.results = append(m.results, result{path: files[i]})
		}
		return
	}

	scored := make([]result, 0, len(m.files))
	for _, f := range m.files {
		if sc, idx, ok := scorePath(f, m.input); ok {
			scored = append(scored, result{path: f, score: sc, indices: idx})
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].path < scored[j].path
	})
	if len(scored) > maxResults {
		scored = scored[:maxResults]
	}
	m.results = scored
}

func (m *Model) ensureIndex() tea.Cmd {
	if m.indexed || m.loading {
		return nil
	}
	m.loading = true
	root := m.root
	mt := m.matcher
	return func() tea.Msg {
		files, err := indexFiles(root, mt)
		return IndexedMsg{Files: files, Err: err}
	}
}

func indexFiles(root string, mt matcher) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if p == root {
			return nil
		}
		name := d.Name()
		if name == ".git" && d.IsDir() {
			return fs.SkipDir
		}
		isDir := d.IsDir()
		hidden := strings.HasPrefix(name, ".") || winutil.HasHiddenAttr(d)
		if hidden {
			if isDir {
				return fs.SkipDir
			}
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if mt != nil && mt.Match(rel, isDir) {
			if isDir {
				return fs.SkipDir
			}
			return nil
		}
		if !isDir {
			files = append(files, rel)
		}
		return nil
	})
	return files, err
}
