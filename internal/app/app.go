// Package app owns the root Bubble Tea model: window size, focus, overlays and
// one sub-model per pane. Nothing here performs blocking I/O inside Update.
package app

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lakshwin/bento/internal/agents"
	buildpane "github.com/lakshwin/bento/internal/build"
	"github.com/lakshwin/bento/internal/console"
	"github.com/lakshwin/bento/internal/finder"
	gitpane "github.com/lakshwin/bento/internal/git"
	"github.com/lakshwin/bento/internal/layout"
	"github.com/lakshwin/bento/internal/theme"
	"github.com/lakshwin/bento/internal/tree"
	"github.com/lakshwin/bento/internal/ui"
	"github.com/lakshwin/bento/internal/viewer"
)

// Minimum console size. Below this we show a calm "too small" state.
const (
	MinWidth  = layout.MinWidth
	MinHeight = layout.MinHeight
)

// Model is the root application model.
type Model struct {
	Dir     string
	Version string

	width  int
	height int

	panes        map[ui.PaneID]ui.Pane
	tree         *tree.Model
	viewer       *viewer.Model
	finder       *finder.Model
	changes      *gitpane.Model
	console      *console.Model
	agents       *agents.Model
	build        *buildpane.Model
	agentConsole *console.Model
	agentRunning bool

	finderOpen bool

	showTree  bool
	showAgent bool
	focused   ui.PaneID

	showHelp bool
	quitting bool

	confirm     string
	pendingPath string
	pendingCmd  string

	message      string
	messageError bool
	mode         string

	startup tea.Cmd
}

// Snapshot is the slice of session state persisted between launches.
type Snapshot struct {
	Dir       string
	File      string
	Focus     string
	ShowTree  bool
	ShowAgent bool
	Line      int
	Col       int
}

// New builds the root model for a project directory.
func New(dir, version string) *Model {
	t := tree.New(dir)
	v := viewer.New()
	f := finder.New(dir)
	ch := gitpane.New(dir)
	con := console.New(dir)
	ag := agents.New()
	bld := buildpane.New(dir)
	m := &Model{
		Dir:       dir,
		Version:   version,
		panes:     map[ui.PaneID]ui.Pane{},
		tree:      t,
		viewer:    v,
		finder:    f,
		changes:   ch,
		console:   con,
		agents:    ag,
		build:     bld,
		showTree:  true,
		showAgent: true,
		focused:   ui.PaneViewer,
		mode:      "READY",
	}
	m.panes[ui.PaneTree] = t
	m.panes[ui.PaneViewer] = v
	m.panes[ui.PaneTerminal] = con
	m.panes[ui.PaneChanges] = ch
	m.panes[ui.PaneAgent] = ag
	m.panes[ui.PaneBuild] = bld
	return m
}

// Focused reports the currently focused pane (useful for tests).
func (m *Model) Focused() ui.PaneID { return m.focused }

// Snapshot captures session state for persistence.
func (m *Model) Snapshot() Snapshot {
	line, col := m.viewer.Cursor()
	return Snapshot{
		Dir:       m.Dir,
		File:      m.viewer.Path(),
		Focus:     m.focused.String(),
		ShowTree:  m.showTree,
		ShowAgent: m.showAgent,
		Line:      line,
		Col:       col,
	}
}

// Restore applies saved session state. File loading is deferred to Init so it
// still happens off the UI goroutine.
func (m *Model) Restore(s Snapshot) {
	m.showTree = s.ShowTree
	m.showAgent = s.ShowAgent
	if id, ok := paneFromString(s.Focus); ok {
		m.focused = id
	}
	if s.File != "" && withinDir(m.Dir, s.File) {
		if info, err := os.Stat(s.File); err == nil && !info.IsDir() {
			m.startup = m.viewer.Open(s.File)
			m.viewer.RestoreCursor(s.Line, s.Col)
			m.mode = "PREVIEW"
		}
	}
	m.ensureFocusVisible()
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.panes)+1)
	if m.startup != nil {
		cmds = append(cmds, m.startup)
		m.startup = nil
	}
	for _, p := range m.panes {
		if cmd := p.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ensureFocusVisible()
		var finderCmd tea.Cmd
		if m.finderOpen {
			w, h := m.finderBox()
			finderCmd = m.finder.Update(ui.PaneSizeMsg{Width: w, Height: h})
		}
		return m, tea.Batch(finderCmd, m.resizePanes())

	case tea.MouseMsg:
		return m, m.handleMouse(msg)

	case tree.DirLoadedMsg, viewer.FileLoadedMsg, finder.IndexedMsg,
		viewer.SavedMsg, viewer.HighlightedMsg, gitpane.LoadedMsg,
		buildpane.RunStartedMsg, buildpane.RunEventMsg,
		console.ShellStartedMsg, console.ShellOutputMsg,
		console.ShellExitedMsg, console.ShellErrMsg:
		return m, m.routeToPane(msg)

	case tree.OpenFileMsg:
		return m, m.requestOpen(msg.Path)

	case finder.OpenMsg:
		m.finderOpen = false
		return m, m.requestOpen(msg.Path)

	case gitpane.OpenMsg:
		return m, m.requestOpen(msg.Path)

	case agents.LaunchMsg:
		m.agentConsole = console.New(m.Dir)
		m.panes[ui.PaneAgent] = m.agentConsole
		m.agentRunning = true
		m.focused = ui.PaneAgent
		m.mode = "SHELL"
		m.setMessage("Launching "+msg.Name+"…", false)
		cmd := m.agentConsole.StartCommand(msg.Name, msg.Path)
		return m, tea.Batch(cmd, m.resizePanes())

	case agents.InstallMsg:
		m.confirm = "install"
		m.pendingCmd = msg.Command
		m.setMessage("Install "+msg.Name+"? y run, n stay", false)
		return m, nil

	case tea.KeyMsg:
		if m.confirm != "" {
			switch msg.String() {
			case "y", "Y":
				action := m.confirm
				path := m.pendingPath
				install := m.pendingCmd
				m.confirm = ""
				m.pendingPath = ""
				m.pendingCmd = ""
				if action == "quit" {
					m.quitting = true
					m.console.Close()
					return m, tea.Quit
				}
				if action == "install" {
					m.setMessage("Installing…", false)
					return m, m.runInstall(install)
				}
				m.setMessage("", false)
				return m, m.openPath(path)
			case "n", "N", "esc":
				m.confirm = ""
				m.pendingPath = ""
				m.pendingCmd = ""
				m.setMessage("", false)
			}
			return m, nil
		}
		if m.showHelp {
			switch msg.String() {
			case "esc", "?", "q", "ctrl+q":
				m.showHelp = false
			}
			return m, nil
		}
		if m.finderOpen {
			if msg.String() == "ctrl+q" {
				m.quitting = true
				return m, tea.Quit
			}
			cmd := m.finder.Update(msg)
			if !m.finder.Opened() {
				m.finderOpen = false
			}
			return m, cmd
		}
		if m.viewer.Searching() {
			switch msg.String() {
			case "ctrl+q", "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			}
			next, cmd := m.viewer.Update(msg)
			m.panes[ui.PaneViewer] = next
			return m, cmd
		}
		if handled, cmd := m.handleGlobal(msg); handled {
			return m, cmd
		}
		p := m.panes[m.focused]
		if p == nil {
			return m, nil
		}
		next, cmd := p.Update(msg)
		m.panes[m.focused] = next
		return m, cmd
	}

	if p := m.panes[m.focused]; p != nil {
		next, cmd := p.Update(msg)
		m.panes[m.focused] = next
		return m, cmd
	}
	return m, nil
}

func (m *Model) requestOpen(path string) tea.Cmd {
	if m.viewer.Dirty() {
		m.confirm = "open"
		m.pendingPath = path
		m.setMessage("Unsaved changes: y discard and open, n stay", false)
		return nil
	}
	return m.openPath(path)
}

func (m *Model) openPath(path string) tea.Cmd {
	m.confirm = ""
	m.pendingPath = ""
	m.focused = ui.PaneViewer
	m.mode = "PREVIEW"
	m.setMessage("", false)
	cmd := m.viewer.Open(path)
	return tea.Batch(cmd, m.resizePanes())
}

func (m *Model) handleGlobal(msg tea.KeyMsg) (bool, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		if m.focused == ui.PaneTerminal {
			return false, nil
		}
		return true, m.requestQuit()
	case "ctrl+q":
		return true, m.requestQuit()
	case "ctrl+s":
		if cmd := m.viewer.Save(); cmd != nil {
			m.setMessage("Saving…", false)
			return true, cmd
		}
		m.setMessage("No changes to save", false)
		return true, nil
	case "?":
		m.showHelp = true
		return true, m.resizePanes()
	case "ctrl+t":
		m.focused = ui.PaneTerminal
		m.mode = "SHELL"
		return true, tea.Batch(m.console.NewShell(), m.resizePanes())
	case "ctrl+a":
		m.showAgent = true
		m.ensureFocusVisible()
		m.focusPane(ui.PaneAgent)
		return true, m.resizePanes()
	case "ctrl+p":
		m.finderOpen = true
		w, h := m.finderBox()
		m.finder.Update(ui.PaneSizeMsg{Width: w, Height: h})
		return true, m.finder.Open()
	case "ctrl+f":
		m.focusPane(ui.PaneViewer)
		if m.viewer.Editing() {
			m.setMessage("Press esc to leave edit mode before searching", false)
			return true, nil
		}
		if m.viewer.StartSearch() {
			return true, m.resizePanes()
		}
		m.setMessage("Open a file first (select it in Tree and press enter)", false)
		return true, nil
	case "tab":
		m.cycleFocus(1)
		return true, m.resizePanes()
	case "shift+tab":
		m.cycleFocus(-1)
		return true, m.resizePanes()
	case "alt+t":
		m.toggleTree()
		return true, m.resizePanes()
	case "alt+a":
		m.toggleAgent()
		return true, m.resizePanes()
	case "alt+1":
		m.focusPane(ui.PaneTree)
		return true, m.resizePanes()
	case "alt+2":
		m.focusPane(ui.PaneViewer)
		return true, m.resizePanes()
	case "alt+3":
		m.focusPane(ui.PaneTerminal)
		return true, m.resizePanes()
	case "alt+4":
		m.focusPane(ui.PaneChanges)
		return true, m.resizePanes()
	case "alt+5":
		m.focusPane(ui.PaneAgent)
		return true, m.resizePanes()
	}
	return false, nil
}

// routeToPane delivers an async result to the pane that produced it, even if
// focus has moved elsewhere in the meantime.
func (m *Model) routeToPane(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case tree.DirLoadedMsg:
		_, cmd := m.tree.Update(msg)
		return cmd
	case viewer.FileLoadedMsg:
		_, cmd := m.viewer.Update(msg)
		return cmd
	case finder.IndexedMsg:
		return m.finder.Update(msg)
	case viewer.SavedMsg:
		saved := msg.(viewer.SavedMsg)
		_, cmd := m.viewer.Update(saved)
		if saved.Err != nil {
			m.setMessage("Save failed: "+saved.Err.Error(), true)
		} else {
			m.setMessage("Saved "+filepath.Base(saved.Path), false)
		}
		return cmd
	case viewer.HighlightedMsg:
		m.viewer.Update(msg)
		return nil
	case gitpane.LoadedMsg:
		_, cmd := m.changes.Update(msg)
		return cmd
	case buildpane.RunStartedMsg, buildpane.RunEventMsg:
		_, cmd := m.build.Update(msg)
		return cmd
	case console.ShellStartedMsg, console.ShellOutputMsg,
		console.ShellExitedMsg:
		target := m.console
		if m.agentRunning && m.agentConsole != nil {
			target = m.agentConsole
		}
		_, cmd := target.Update(msg)
		if _, started := msg.(console.ShellStartedMsg); started {
			m.setMessage("", false)
		}
		if exited, ok := msg.(console.ShellExitedMsg); ok && exited.Title != "" {
			m.setMessage(exited.Title+" exited; see the terminal output", true)
		}
		return cmd
	case console.ShellErrMsg:
		err := msg.(console.ShellErrMsg)
		target := m.console
		if m.agentRunning && m.agentConsole != nil {
			target = m.agentConsole
		}
		_, cmd := target.Update(err)
		if err.Err != nil {
			m.setMessage("Launch failed: "+err.Err.Error(), true)
		}
		return cmd
	}
	return nil
}

// resizePanes tells every visible pane its rectangle so paging and wrapping
// have correct dimensions without doing layout work in View.
func (m *Model) resizePanes() tea.Cmd {
	if m.width == 0 || m.height == 0 {
		return nil
	}
	grid := layout.Solve(m.width, m.height-1, m.layoutState())
	cmds := make([]tea.Cmd, 0, len(grid.Rects))
	for id, r := range grid.Rects {
		p := m.panes[id]
		if p == nil {
			continue
		}
		if _, cmd := p.Update(ui.PaneSizeMsg{ID: id, Width: r.W, Height: r.H}); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) layoutState() layout.State {
	return layout.State{ShowTree: m.showTree, ShowAgent: m.showAgent, Active: m.focused}
}

func (m *Model) visible() []ui.PaneID {
	ids := layout.VisiblePanes(m.layoutState())
	if m.width == 0 || m.height == 0 {
		return ids
	}
	grid := layout.Solve(m.width, m.height-1, m.layoutState())
	if grid.Mode == layout.Narrow {
		return ids
	}
	out := make([]ui.PaneID, 0, len(ids))
	for _, id := range ids {
		if _, ok := grid.Rects[id]; ok {
			out = append(out, id)
		}
	}
	return out
}

func (m *Model) cycleFocus(delta int) {
	ids := m.visible()
	if len(ids) == 0 {
		return
	}
	idx := -1
	for i, id := range ids {
		if id == m.focused {
			idx = i
			break
		}
	}
	next := 0
	if idx >= 0 {
		next = (idx + delta + len(ids)) % len(ids)
	}
	m.focused = ids[next]
}

func (m *Model) focusPane(id ui.PaneID) {
	for _, vis := range m.visible() {
		if vis == id {
			m.focused = id
			return
		}
	}
	m.setMessage(id.String()+" pane is hidden", false)
}

func (m *Model) toggleTree() {
	m.showTree = !m.showTree
	m.ensureFocusVisible()
}

func (m *Model) toggleAgent() {
	m.showAgent = !m.showAgent
	m.ensureFocusVisible()
}

func paneFromString(s string) (ui.PaneID, bool) {
	switch strings.ToLower(s) {
	case "tree":
		return ui.PaneTree, true
	case "viewer":
		return ui.PaneViewer, true
	case "terminal":
		return ui.PaneTerminal, true
	case "changes":
		return ui.PaneChanges, true
	case "agent":
		return ui.PaneAgent, true
	}
	return 0, false
}

func withinDir(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil || rel == ".." || filepath.IsAbs(rel) {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func (m *Model) ensureFocusVisible() {
	for _, id := range m.visible() {
		if id == m.focused {
			return
		}
	}
	if ids := m.visible(); len(ids) > 0 {
		m.focused = ids[0]
	}
}

func (m *Model) requestQuit() tea.Cmd {
	if m.viewer.Dirty() {
		m.confirm = "quit"
		m.setMessage("Unsaved changes: y discard and quit, n stay", false)
		return nil
	}
	m.quitting = true
	m.console.Close()
	return tea.Quit
}

func (m *Model) runInstall(line string) tea.Cmd {
	if strings.TrimSpace(line) == "" {
		return nil
	}
	m.focused = ui.PaneTerminal
	m.mode = "SHELL"
	comspec := os.Getenv("COMSPEC")
	if comspec == "" {
		comspec = "cmd.exe"
	}
	return tea.Batch(m.console.StartCommand("install", comspec, "/c", line), m.resizePanes())
}

func (m *Model) handleMouse(msg tea.MouseMsg) tea.Cmd {
	id, r, ok := m.paneAt(msg.X, msg.Y)
	if !ok {
		return nil
	}
	p := m.panes[id]
	if p == nil {
		return nil
	}
	switch {
	case msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown:
		key := tea.KeyMsg{Type: tea.KeyUp}
		if msg.Button == tea.MouseButtonWheelDown {
			key.Type = tea.KeyDown
		}
		next, cmd := p.Update(key)
		m.panes[id] = next
		return cmd
	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft:
		m.focused = id
		next, cmd := p.Update(ui.ClickMsg{X: msg.X - r.X, Y: msg.Y - r.Y})
		m.panes[id] = next
		return tea.Batch(cmd, m.resizePanes())
	}
	return nil
}

func (m *Model) paneAt(x, y int) (ui.PaneID, layout.Rect, bool) {
	if m.width == 0 || m.height == 0 {
		return 0, layout.Rect{}, false
	}
	grid := layout.Solve(m.width, m.height-1, m.layoutState())
	for id, r := range grid.Rects {
		if x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H {
			return id, r, true
		}
	}
	return 0, layout.Rect{}, false
}

func (m *Model) focusAt(x, y int) {
	if id, _, ok := m.paneAt(x, y); ok {
		m.focused = id
	}
}

func (m *Model) setMessage(text string, isErr bool) {
	m.message = text
	m.messageError = isErr
}

// View implements tea.Model.
func (m *Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	if m.width < MinWidth || m.height < MinHeight {
		return tooSmallView(m.width, m.height)
	}
	if m.showHelp {
		return m.helpView()
	}
	content := m.gridView()
	hints := m.statusHints()
	mode := m.modeLabel()
	if m.confirm != "" {
		hints = "y discard   n stay"
	}
	if m.finderOpen {
		bw, bh := m.finderBox()
		box := m.finder.View(bw, bh)
		x := (m.width - bw) / 2
		y := (m.height - 1 - bh) / 2
		content = ui.Overlay(content, m.width, m.height-1, box, x, y)
		hints = "type filter   ↑↓ select   enter open   esc close"
		mode = "FIND"
	}
	status := ui.StatusBar(m.width, ui.StatusData{
		Project:        projectName(m.Dir),
		Branch:         m.changes.Branch(),
		Dirty:          m.changes.Dirty(),
		Hints:          hints,
		Message:        m.message,
		MessageIsError: m.messageError,
		Mode:           mode,
	})
	return strings.Join([]string{content, status}, "\n")
}

func (m *Model) modeLabel() string {
	if m.confirm != "" {
		return "UNSAVED"
	}
	if m.finderOpen {
		return "FIND"
	}
	if m.viewer.Editing() {
		return "EDIT"
	}
	if m.viewer.Searching() {
		return "SEARCH"
	}
	return m.mode
}

func (m *Model) finderBox() (int, int) {
	w := clampInt(m.width-10, 40, 90)
	h := clampInt(m.height-8, 10, 22)
	if w > m.width-2 {
		w = m.width - 2
	}
	if h > m.height-3 {
		h = m.height - 3
	}
	if w < 10 {
		w = 10
	}
	if h < 5 {
		h = 5
	}
	return w, h
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

func (m *Model) gridView() string {
	contentH := m.height - 1
	grid := layout.Solve(m.width, contentH, m.layoutState())
	if grid.TooSmall {
		return tooSmallView(m.width, contentH)
	}

	items := make([]ui.Placed, 0, len(grid.Rects))
	for id, r := range grid.Rects {
		p := m.panes[id]
		if p == nil {
			continue
		}
		items = append(items, ui.Placed{X: r.X, Y: r.Y, View: p.View(r.W, r.H, id == m.focused)})
	}
	content := ui.Compose(m.width, contentH, items)

	if grid.Mode == layout.Narrow {
		labels := make([]string, 0, len(grid.Visible))
		active := 0
		for i, id := range grid.Visible {
			label := id.String()
			if p := m.panes[id]; p != nil {
				label = p.Title()
			}
			labels = append(labels, strconv.Itoa(int(id)+1)+" "+label)
			if id == m.focused {
				active = i
			}
		}
		line := ui.TabStrip(m.width, labels, active)
		rows := strings.Split(content, "\n")
		if len(rows) > 0 {
			rows[0] = line
		}
		content = strings.Join(rows, "\n")
	}
	return content
}

func (m *Model) statusHints() string {
	var hints []string
	if p := m.panes[m.focused]; p != nil {
		for _, h := range p.KeyHints() {
			hints = append(hints, h.Key+" "+h.Desc)
		}
	}
	hints = append(hints, "tab focus", "? help")
	return strings.Join(hints, "  ")
}

func (m *Model) helpView() string {
	var b strings.Builder
	b.WriteString(theme.TitleStyle(true).Render("Bento — keyboard reference"))
	b.WriteString("\n\n")
	for _, g := range GlobalBindings {
		keys := strings.Join(g.Keys, " / ")
		b.WriteString(theme.AccentStyle().Render(padRight(keys, 18)))
		b.WriteString("  ")
		b.WriteString(theme.TextStyle().Render(g.Help))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(theme.MutedStyle().Render("press esc or ? to close"))
	body := lipgloss.Place(m.width-2, m.height-3, lipgloss.Center, lipgloss.Center, b.String())
	panel := ui.Island("Help", m.width, m.height-1, true, body)
	status := ui.StatusBar(m.width, ui.StatusData{
		Project: projectName(m.Dir),
		Hints:   "esc close",
		Mode:    "HELP",
	})
	return strings.Join([]string{panel, status}, "\n")
}

func tooSmallView(w, h int) string {
	msg := "Terminal too small"
	sub := "Bento needs at least 60 x 15 cells."
	if w < 4 || h < 2 {
		return ui.Fit(msg, max(w, 0))
	}
	block := theme.ErrorStyle().Render(msg) + "\n" + theme.MutedStyle().Render(sub)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, block)
}

func projectName(dir string) string {
	dir = strings.TrimRight(dir, `\/`)
	if i := strings.LastIndexAny(dir, `\/`); i >= 0 {
		return dir[i+1:]
	}
	return dir
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
