// Package app owns the root Bubble Tea model: window size, focus, overlays and
// one sub-model per pane. Nothing here performs blocking I/O inside Update.
package app

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lakshwin/bento/internal/agents"
	"github.com/lakshwin/bento/internal/console"
	"github.com/lakshwin/bento/internal/finder"
	gitpane "github.com/lakshwin/bento/internal/git"
	"github.com/lakshwin/bento/internal/layout"
	"github.com/lakshwin/bento/internal/palette"
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
	palette      *palette.Model
	changes      *gitpane.Model
	console      *console.Model
	console2     *console.Model
	agents       *agents.Model
	agentConsole *console.Model
	agentRunning bool

	finderOpen  bool
	paletteOpen bool
	zoomed      bool

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
	messageAt    time.Time
	mode         string

	startup tea.Cmd
}

// toastTickMsg drives automatic dismissal of informational status messages.
type toastTickMsg struct{}

func toastTick() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return toastTickMsg{} })
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
	pal := palette.New()
	ch := gitpane.New(dir)
	con := console.New(dir)
	con2 := console.New(dir)
	con2.SetTitle("Terminal 2")
	ag := agents.New()
	m := &Model{
		Dir:       dir,
		Version:   version,
		panes:     map[ui.PaneID]ui.Pane{},
		tree:      t,
		viewer:    v,
		finder:    f,
		palette:   pal,
		changes:   ch,
		console:   con,
		console2:  con2,
		agents:    ag,
		showTree:  true,
		showAgent: true,
		focused:   ui.PaneViewer,
		mode:      "READY",
	}
	m.panes[ui.PaneTree] = t
	m.panes[ui.PaneViewer] = v
	m.panes[ui.PaneTerminal] = con
	m.panes[ui.PaneTerminal2] = con2
	m.panes[ui.PaneChanges] = ch
	m.panes[ui.PaneAgent] = ag
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
	cmds = append(cmds, toastTick())
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
		var paletteCmd tea.Cmd
		if m.paletteOpen {
			w, h := m.paletteBox()
			paletteCmd = m.palette.Update(ui.PaneSizeMsg{Width: w, Height: h})
		}
		return m, tea.Batch(finderCmd, paletteCmd, m.resizePanes())

	case toastTickMsg:
		if m.message != "" && !m.messageError && m.confirm == "" && time.Since(m.messageAt) >= 4*time.Second {
			m.message = ""
		}
		return m, toastTick()

	case palette.RunMsg:
		m.paletteOpen = false
		return m, m.runPaletteCommand(msg.ID)

	case tea.MouseMsg:
		return m, m.handleMouse(msg)

	case tree.DirLoadedMsg, viewer.FileLoadedMsg, finder.IndexedMsg,
		viewer.SavedMsg, viewer.HighlightedMsg, gitpane.LoadedMsg,
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
					m.closeConsoles()
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
		if m.paletteOpen {
			switch msg.String() {
			case "ctrl+q", "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			}
			cmd := m.palette.Update(msg)
			if !m.palette.Opened() {
				m.paletteOpen = false
			}
			return m, cmd
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
		if m.focused == ui.PaneTerminal || m.focused == ui.PaneTerminal2 {
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
	case "ctrl+k":
		return true, m.openPalette()
	case "alt+z":
		m.zoomed = !m.zoomed
		return true, m.resizePanes()
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
	case console.ShellStartedMsg, console.ShellOutputMsg,
		console.ShellExitedMsg:
		target := m.consoleFor(msg)
		if target == nil {
			return nil
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
		target := m.consoleFor(err)
		if target == nil {
			return nil
		}
		_, cmd := target.Update(err)
		if err.Err != nil {
			m.setMessage("Launch failed: "+err.Err.Error(), true)
		}
		return cmd
	}
	return nil
}

func (m *Model) consoleFor(msg any) *console.Model {
	var id int
	switch event := msg.(type) {
	case console.ShellStartedMsg:
		id = event.Model
	case console.ShellOutputMsg:
		id = event.Model
	case console.ShellExitedMsg:
		id = event.Model
	case console.ShellErrMsg:
		id = event.Model
	default:
		return nil
	}
	if m.agentConsole != nil && m.agentConsole.ID() == id {
		return m.agentConsole
	}
	if m.console != nil && m.console.ID() == id {
		return m.console
	}
	if m.console2 != nil && m.console2.ID() == id {
		return m.console2
	}
	return nil
}

func (m *Model) closeConsoles() {
	if m.console != nil {
		m.console.Close()
	}
	if m.console2 != nil {
		m.console2.Close()
	}
	if m.agentConsole != nil {
		m.agentConsole.Close()
	}
}

// resizePanes tells every visible pane its rectangle so paging and wrapping
// have correct dimensions without doing layout work in View.
func (m *Model) resizePanes() tea.Cmd {
	if m.width == 0 || m.height == 0 {
		return nil
	}
	if m.zoomed {
		if p := m.panes[m.focused]; p != nil {
			if _, cmd := p.Update(ui.PaneSizeMsg{ID: m.focused, Width: m.width, Height: m.height - 1}); cmd != nil {
				return cmd
			}
		}
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
	case "terminal 2", "terminal2":
		return ui.PaneTerminal2, true
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
	m.closeConsoles()
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
	if m.showHelp || m.finderOpen || m.paletteOpen {
		return nil
	}
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
	if m.zoomed {
		return m.focused, layout.Rect{X: 0, Y: 0, W: m.width, H: m.height - 1}, true
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
	m.messageAt = time.Now()
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
		hints = "type filter   up/down select   enter open   esc close"
		mode = "FIND"
	}
	if m.paletteOpen {
		bw, bh := m.paletteBox()
		box := m.palette.View(bw, bh)
		x := (m.width - bw) / 2
		y := (m.height - 1 - bh) / 2
		content = ui.Overlay(content, m.width, m.height-1, box, x, y)
		hints = "type filter   up/down move   enter run   esc close"
		mode = "COMMAND"
	}
	position := ""
	if m.viewer.Path() != "" {
		line, col := m.viewer.Cursor()
		position = "Ln " + strconv.Itoa(line+1) + ":" + strconv.Itoa(col+1)
	}
	status := ui.StatusBar(m.width, ui.StatusData{
		Project:        projectName(m.Dir),
		Branch:         m.changes.Branch(),
		Dirty:          m.changes.Dirty(),
		Hints:          hints,
		Message:        m.message,
		MessageIsError: m.messageError,
		Position:       position,
		Mode:           mode,
		Zoom:           m.zoomed,
	})
	return strings.Join([]string{content, status}, "\n")
}

func (m *Model) modeLabel() string {
	if m.confirm != "" {
		return "UNSAVED"
	}
	if m.paletteOpen {
		return "COMMAND"
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

func (m *Model) paletteBox() (int, int) {
	w := clampInt(m.width-16, 44, 84)
	h := clampInt(m.height-8, 9, 20)
	if w > m.width-2 {
		w = m.width - 2
	}
	if h > m.height-3 {
		h = m.height - 3
	}
	if w < 16 {
		w = 16
	}
	if h < 5 {
		h = 5
	}
	return w, h
}

func (m *Model) openPalette() tea.Cmd {
	m.paletteOpen = true
	w, h := m.paletteBox()
	m.palette.Update(ui.PaneSizeMsg{Width: w, Height: h})
	return m.palette.Open(m.paletteCommands())
}

func (m *Model) paletteCommands() []palette.Command {
	return []palette.Command{
		{ID: "pane.tree", Title: "Focus Tree", Description: "File tree pane", Keys: "alt+1"},
		{ID: "pane.viewer", Title: "Focus Preview", Description: "Editor and preview pane", Keys: "alt+2"},
		{ID: "pane.terminal", Title: "Focus Terminal", Description: "Shell tab strip", Keys: "alt+3"},
		{ID: "pane.changes", Title: "Focus Changes", Description: "Git working tree", Keys: "alt+4"},
		{ID: "pane.agent", Title: "Focus Agents", Description: "AI agent launcher", Keys: "alt+5"},
		{ID: "view.toggleTree", Title: "Toggle file tree", Description: "Show or hide the tree column", Keys: "alt+t"},
		{ID: "view.toggleAgent", Title: "Toggle agent column", Description: "Show or hide the agents pane", Keys: "alt+a"},
		{ID: "view.zoom", Title: "Toggle pane zoom", Description: "Maximize the focused pane", Keys: "alt+z"},
		{ID: "file.find", Title: "Find file", Description: "Fuzzy file finder", Keys: "ctrl+p"},
		{ID: "file.search", Title: "Search in file", Description: "Find text in the open file", Keys: "ctrl+f"},
		{ID: "file.save", Title: "Save file", Description: "Write editor changes", Keys: "ctrl+s"},
		{ID: "shell.new", Title: "New shell tab", Description: "Open a fresh terminal tab", Keys: "ctrl+t"},
		{ID: "app.help", Title: "Keyboard reference", Description: "Show every shortcut", Keys: "?"},
		{ID: "app.quit", Title: "Quit Bento", Description: "Close the workspace", Keys: "ctrl+q"},
	}
}

func (m *Model) runPaletteCommand(id string) tea.Cmd {
	switch id {
	case "pane.tree":
		m.focusPane(ui.PaneTree)
		return m.resizePanes()
	case "pane.viewer":
		m.focusPane(ui.PaneViewer)
		return m.resizePanes()
	case "pane.terminal":
		m.focusPane(ui.PaneTerminal)
		return m.resizePanes()
	case "pane.changes":
		m.focusPane(ui.PaneChanges)
		return m.resizePanes()
	case "pane.agent":
		m.showAgent = true
		m.ensureFocusVisible()
		m.focusPane(ui.PaneAgent)
		return m.resizePanes()
	case "view.toggleTree":
		m.toggleTree()
		return m.resizePanes()
	case "view.toggleAgent":
		m.toggleAgent()
		return m.resizePanes()
	case "view.zoom":
		m.zoomed = !m.zoomed
		return m.resizePanes()
	case "file.find":
		_, cmd := m.handleGlobal(tea.KeyMsg{Type: tea.KeyCtrlP})
		return cmd
	case "file.search":
		_, cmd := m.handleGlobal(tea.KeyMsg{Type: tea.KeyCtrlF})
		return cmd
	case "file.save":
		_, cmd := m.handleGlobal(tea.KeyMsg{Type: tea.KeyCtrlS})
		return cmd
	case "shell.new":
		_, cmd := m.handleGlobal(tea.KeyMsg{Type: tea.KeyCtrlT})
		return cmd
	case "app.help":
		m.showHelp = true
		return m.resizePanes()
	case "app.quit":
		return m.requestQuit()
	}
	return nil
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
	if m.zoomed {
		if p := m.panes[m.focused]; p != nil {
			return p.View(m.width, contentH, true)
		}
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
		// Each pane now owns its title row. Keeping a second global strip here
		// would reintroduce the duplicate chrome the panel restyle removed.
	}
	return content
}

func (m *Model) statusHints() string {
	var hints []ui.KeyHint // per-pane hints now live in each pane's footer
	hints = append(hints, ui.KeyHint{Key: "tab", Desc: "focus"}, ui.KeyHint{Key: "ctrl+k", Desc: "commands"}, ui.KeyHint{Key: "?", Desc: "help"})
	if m.zoomed {
		hints = append(hints, ui.KeyHint{Key: "alt+z", Desc: "unzoom"})
	}
	return ui.HintLine(hints)
}

func (m *Model) helpView() string {
	scopes := make([]string, 0, 3)
	seen := map[string]bool{}
	for _, g := range GlobalBindings {
		if !seen[g.Scope] {
			seen[g.Scope] = true
			scopes = append(scopes, g.Scope)
		}
	}

	var b strings.Builder
	b.WriteString(theme.TitleStyle(true).Render("Bento — keyboard reference"))
	b.WriteString("\n")
	b.WriteString(theme.MutedStyle().Render("Every shortcut Bento understands."))
	for _, scope := range scopes {
		b.WriteString("\n\n")
		b.WriteString(theme.GroupHeaderStyle().Render(strings.ToUpper(scope)))
		for _, g := range GlobalBindings {
			if g.Scope != scope {
				continue
			}
			keys := strings.Join(g.Keys, " / ")
			b.WriteString("\n")
			b.WriteString(" " + theme.KeycapStyle().Render(" "+keys+" "))
			b.WriteString(strings.Repeat(" ", max(0, 24-lipgloss.Width(keys)-2)))
			b.WriteString(theme.TextStyle().Render(g.Help))
		}
	}
	b.WriteString("\n\n")
	b.WriteString(theme.MutedStyle().Render("press "))
	b.WriteString(theme.KeycapStyle().Render(" esc "))
	b.WriteString(theme.MutedStyle().Render(" or "))
	b.WriteString(theme.KeycapStyle().Render(" ? "))
	b.WriteString(theme.MutedStyle().Render(" to close"))

	blockW := 0
	for _, line := range strings.Split(b.String(), "\n") {
		if w := lipgloss.Width(line); w > blockW {
			blockW = w
		}
	}
	block := lipgloss.NewStyle().Width(blockW).Align(lipgloss.Left).Render(b.String())
	panelH := m.height - 1
	innerW, innerH := ui.BodySize(m.width, panelH)
	body := lipgloss.Place(innerW, innerH, lipgloss.Center, lipgloss.Center, block)
	panel := ui.Panel(ui.PanelSpec{
		Tabs:  []ui.Tab{{Label: "Keyboard reference", Active: true}},
		Hints: []ui.KeyHint{{Key: "esc", Desc: "close"}},
	}, m.width, panelH, true, body)
	status := ui.StatusBar(m.width, ui.StatusData{
		Project: projectName(m.Dir),
		Hints:   ui.HintLine([]ui.KeyHint{{Key: "esc", Desc: "close"}}),
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
