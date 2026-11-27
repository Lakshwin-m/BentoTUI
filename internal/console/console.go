// Package console is a real ConPTY shell pane with tab support. Shell I/O stays
// off the UI goroutine; the emulator keeps each tab's screen state.
package console

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/aymanbagabas/go-pty"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"

	"github.com/lakshwin/bento/internal/theme"
	"github.com/lakshwin/bento/internal/ui"
	"github.com/lakshwin/bento/internal/winutil"
)

// ShellStartedMsg announces a ready shell session.
type ShellStartedMsg struct {
	Model int
	shell *shell
}

// ShellOutputMsg carries a chunk of terminal output.
type ShellOutputMsg struct {
	Model int
	ID    int
	Data  []byte
}

// ShellExitedMsg reports that a shell's output stream closed. Title is kept in
// the message so the root model can explain an immediate agent exit in the
// status bar instead of making it look like the launch was ignored.
type ShellExitedMsg struct {
	Model int
	ID    int
	Title string
}

// ShellErrMsg reports a failure to start a shell.
type ShellErrMsg struct {
	Model int
	Err   error
}

var modelSeq int32

type shell struct {
	id     int
	title  string
	pty    pty.Pty
	cmd    *pty.Cmd
	emu    *vt.SafeEmulator
	ch     chan []byte
	exited bool
}

// Model is a shell console pane.
type Model struct {
	id        int
	dir       string
	paneTitle string

	shells []*shell
	active int // 1..len(shells); 0 means no shell yet
	nextID int

	width  int
	height int

	errMsg string
}

// New creates the console for a project directory.
func New(dir string) *Model {
	return &Model{id: int(atomic.AddInt32(&modelSeq, 1)), dir: dir}
}

// ID identifies this console in shell messages, letting the root model route
// output when several consoles are on screen at once.
func (m *Model) ID() int { return m.id }

// SetTitle overrides the pane and island title. Tabs keep their shell titles.
func (m *Model) SetTitle(title string) { m.paneTitle = title }

// Init implements ui.Pane. Start the default PowerShell tab immediately so
// the bottom-center pane is a usable Terminal on first paint.
func (m *Model) Init() tea.Cmd {
	if len(m.shells) == 0 {
		return m.newShell()
	}
	return nil
}

// Update implements ui.Pane.
func (m *Model) Update(msg tea.Msg) (ui.Pane, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.PaneSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeShells()
		return m, nil

	case ui.ClickMsg:
		if msg.Y == 1 {
			m.clickTab(msg.X)
		}
		return m, nil

	case tea.KeyMsg:
		return m, m.handleKey(msg)

	case ShellStartedMsg:
		m.errMsg = ""
		m.shells = append(m.shells, msg.shell)
		m.active = len(m.shells)
		m.resizeShells()
		return m, readCmd(m.id, msg.shell)

	case ShellOutputMsg:
		s := m.find(msg.ID)
		if s == nil || s.emu == nil {
			return m, nil
		}
		s.emu.Write(msg.Data)
		return m, readCmd(m.id, s)

	case ShellExitedMsg:
		if s := m.find(msg.ID); s != nil {
			s.exited = true
		}
		return m, nil

	case ShellErrMsg:
		if msg.Err != nil {
			m.errMsg = msg.Err.Error()
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "ctrl+t":
		return m.newShell()
	case "alt+]":
		m.cycle(1)
		return nil
	case "alt+[":
		m.cycle(-1)
		return nil
	case "alt+w":
		if m.active > 0 {
			m.closeActiveShell()
		}
		return nil
	}
	if m.active < 1 || m.active > len(m.shells) {
		return nil
	}
	s := m.shells[m.active-1]
	if s.exited {
		return nil
	}
	if data := encodeKey(msg); len(data) > 0 && s.pty != nil {
		_, _ = s.pty.Write(data)
	}
	return nil
}

func (m *Model) activeShellID() int {
	if m.active < 1 || m.active > len(m.shells) {
		return -1
	}
	return m.shells[m.active-1].id
}

func (m *Model) find(id int) *shell {
	for _, s := range m.shells {
		if s.id == id {
			return s
		}
	}
	return nil
}

func (m *Model) cycle(delta int) {
	n := len(m.shells)
	if n <= 1 {
		return
	}
	m.active = ((m.active-1+delta)%n+n)%n + 1
}

func (m *Model) closeActiveShell() {
	if m.active < 1 || m.active > len(m.shells) {
		return
	}
	s := m.shells[m.active-1]
	closeShell(s)
	m.shells = append(m.shells[:m.active-1], m.shells[m.active:]...)
	if len(m.shells) == 0 {
		m.active = 0
	} else if m.active > len(m.shells) {
		m.active = len(m.shells)
	}
}

func closeShell(s *shell) {
	if s == nil {
		return
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	if s.pty != nil {
		_ = s.pty.Close()
	}
}

func readCmd(modelID int, s *shell) tea.Cmd {
	ch := s.ch
	id := s.id
	return func() tea.Msg {
		data, ok := <-ch
		if !ok {
			return ShellExitedMsg{Model: modelID, ID: id, Title: s.title}
		}
		return ShellOutputMsg{Model: modelID, ID: id, Data: data}
	}
}

func (m *Model) shellSize() (int, int) {
	cols := m.width - 2
	rows := m.height - ui.ChromeRows
	if cols < 20 {
		cols = 80
	}
	if rows < 5 {
		rows = 24
	}
	return cols, rows
}

func (m *Model) resizeShells() {
	cols, rows := m.shellSize()
	for _, s := range m.shells {
		if s.pty != nil {
			_ = s.pty.Resize(cols, rows)
		}
		if s.emu != nil {
			s.emu.Resize(cols, rows)
		}
	}
}

func (m *Model) clickTab(x int) {
	n := len(m.shells)
	if n == 0 || m.width <= 0 {
		return
	}
	cell := m.width / n
	if cell < 1 {
		cell = 1
	}
	idx := x / cell
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	m.active = idx + 1
}

// NewShell opens a new ConPTY tab. The root model calls this for ctrl+t.
func (m *Model) NewShell() tea.Cmd {
	return m.newShell()
}

func (m *Model) newShell() tea.Cmd {
	m.nextID++
	id := m.nextID
	modelID := m.id
	dir := m.dir
	m.errMsg = ""
	cols, rows := m.shellSize()
	return func() tea.Msg {
		s, err := startShell(id, dir, cols, rows)
		if err != nil {
			return ShellErrMsg{Model: modelID, Err: err}
		}
		return ShellStartedMsg{Model: modelID, shell: s}
	}
}

func startShell(id int, dir string, cols, rows int) (*shell, error) {
	name, args := shellCommand()
	return startSession(id, "PowerShell", dir, cols, rows, name, args)
}

func startSession(id int, title, dir string, cols, rows int, name string, args []string) (*shell, error) {
	name, args = winutil.WrapPTY(name, args)
	p, err := pty.New()
	if err != nil {
		return nil, err
	}
	cmd := p.Command(name, args...)
	cmd.Dir = dir
	if strings.EqualFold(title, "OpenCode") {
		cmd.Env = openCodeEnv()
	}
	if err := cmd.Start(); err != nil {
		_ = p.Close()
		return nil, err
	}
	_ = p.Resize(cols, rows)
	s := &shell{
		id:    id,
		title: title,
		pty:   p,
		cmd:   cmd,
		emu:   vt.NewSafeEmulator(cols, rows),
		ch:    make(chan []byte, 256),
	}
	go s.readLoop(p, cols, rows)
	go func() { _ = cmd.Wait() }()
	return s, nil
}

// openCodeEnv works around a Windows regression in OpenCode's Bun runtime:
// some versions fail with EEXIST when the normal ~/.config/opencode directory
// already exists. Keep the user's config file when possible, but give
// OpenCode a separate config root so it can boot reliably from Bento.
func openCodeEnv() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	original := filepath.Join(home, ".config", "opencode")
	if _, err := os.Stat(original); err != nil {
		return nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return nil
	}
	isolated := filepath.Join(base, "bento", "opencode-config")
	if err := os.MkdirAll(isolated, 0o755); err != nil {
		return nil
	}
	env := append([]string(nil), os.Environ()...)
	env = append(env, "XDG_CONFIG_HOME="+isolated)
	// OpenCode's TUI uses terminfo and true-color capabilities even inside
	// ConPTY. Make those capabilities explicit because Bento is not itself a
	// Windows Terminal tab and may inherit no TERM metadata.
	env = append(env, "TERM=xterm-256color", "COLORTERM=truecolor")
	for _, name := range []string{"opencode.json", "opencode.jsonc", "config.json", "config.jsonc"} {
		candidate := filepath.Join(original, name)
		if _, err := os.Stat(candidate); err == nil {
			env = append(env, "OPENCODE_CONFIG="+candidate)
			break
		}
	}
	return env
}

// StartCommand opens a new console tab running an external command, used by
// the agent launcher. The process starts off the UI goroutine.
func (m *Model) StartCommand(title, path string, args ...string) tea.Cmd {
	m.nextID++
	id := m.nextID
	modelID := m.id
	dir := m.dir
	cols, rows := m.shellSize()
	if title == "" {
		title = "agent"
	}
	m.errMsg = ""
	argv := append([]string(nil), args...)
	return func() tea.Msg {
		s, err := startSession(id, title, dir, cols, rows, path, argv)
		if err != nil {
			return ShellErrMsg{Model: modelID, Err: err}
		}
		return ShellStartedMsg{Model: modelID, shell: s}
	}
}

func shellCommand() (string, []string) {
	if p, err := exec.LookPath("pwsh.exe"); err == nil {
		return p, []string{"-NoLogo"}
	}
	if p, err := exec.LookPath("powershell.exe"); err == nil {
		return p, []string{"-NoLogo"}
	}
	if p, err := exec.LookPath("cmd.exe"); err == nil {
		return p, nil
	}
	return "cmd.exe", nil
}

func (s *shell) readLoop(p pty.Pty, cols, rows int) {
	buf := make([]byte, 32*1024)
	for {
		n, err := p.Read(buf)
		if n > 0 {
			respondToTerminalQueries(p, buf[:n], cols, rows)
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			s.ch <- chunk
		}
		if err != nil {
			close(s.ch)
			return
		}
	}
}

// respondToTerminalQueries answers the small set of terminal capability
// queries used by full-screen TUIs. A real Windows Terminal answers these
// while hosting ConPTY; Bento is the host, so it must answer them itself.
func respondToTerminalQueries(p pty.Pty, data []byte, cols, rows int) {
	if bytes.Contains(data, []byte("\x1b[14t")) {
		// CSI 4 ; height ; width t reports text-area pixel dimensions.
		_, _ = p.Write([]byte("\x1b[4;480;640t"))
	}
	if bytes.Contains(data, []byte("\x1b[18t")) {
		// CSI 8 ; rows ; cols t reports the character-cell dimensions.
		_, _ = p.Write([]byte("\x1b[8;" + itoa(rows) + ";" + itoa(cols) + "t"))
	}
}

// View implements ui.Pane.
func (m *Model) View(width, height int, focused bool) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	labels := make([]string, 0, len(m.shells))
	for _, s := range m.shells {
		label := s.title
		if s.exited {
			label += " (exited)"
		}
		labels = append(labels, label)
	}
	if len(labels) == 0 {
		labels = append(labels, m.displayTitle())
	}
	active := 0
	if len(m.shells) > 0 {
		active = m.active - 1
	}
	if active < 0 {
		active = 0
	}
	if active >= len(labels) {
		active = len(labels) - 1
	}
	tabs := make([]ui.Tab, 0, len(labels))
	for i, l := range labels {
		tabs = append(tabs, ui.Tab{Label: l, Active: i == active})
	}
	spec := ui.PanelSpec{Tabs: tabs, Hints: m.KeyHints()}
	if m.paneTitle != "" {
		spec.Right = theme.MutedStyle().Render(m.paneTitle)
	}

	innerW, innerH := ui.BodySize(width, height)
	var body string
	switch {
	case len(m.shells) == 0 && m.errMsg != "":
		body = ui.CenteredCard(innerW, innerH, "Could not start process", m.errMsg)
	case len(m.shells) == 0:
		body = ui.CenteredCard(innerW, innerH, "Starting shell", "The PowerShell session is warming up.")
	default:
		body = m.shellBody(m.shells[active], innerW, innerH)
	}
	return ui.Panel(spec, width, height, focused, body)
}

// shellBody renders one shell's terminal grid into an innerW x innerH body.
func (m *Model) shellBody(s *shell, innerW, innerH int) string {
	if s == nil || s.emu == nil {
		return ui.CenteredCard(innerW, innerH, "Shell unavailable", m.errMsg)
	}
	rendered := s.emu.Render()
	lines := strings.Split(rendered, "\n")
	if len(lines) > innerH {
		lines = lines[:innerH]
	}
	for i := range lines {
		lines[i] = ui.Fit(lines[i], innerW)
	}
	for len(lines) < innerH {
		lines = append(lines, ui.Fit("", innerW))
	}
	if !s.exited && !s.emu.CursorHidden() {
		pos := s.emu.CursorPosition()
		overlayCursor(lines, pos.X, pos.Y, innerW)
	}
	return strings.Join(lines, "\n")
}

func (m *Model) displayTitle() string {
	if m.paneTitle != "" {
		return m.paneTitle
	}
	return "Terminal"
}

func overlayCursor(lines []string, x, y, width int) {
	if y < 0 || y >= len(lines) || x < 0 || x >= width {
		return
	}
	left := ansi.Cut(lines[y], 0, x)
	cell := ansi.Cut(lines[y], x, x+1)
	right := ansi.Cut(lines[y], x+1, width)
	if cell == "" {
		cell = " "
	}
	lines[y] = left + theme.CursorStyle().Render(cell) + right
}

// Title implements ui.Pane.
func (m *Model) Title() string {
	if m.paneTitle != "" {
		return m.paneTitle
	}
	if m.active < 1 || m.active > len(m.shells) {
		return "Terminal"
	}
	return m.shells[m.active-1].title
}

// KeyHints implements ui.Pane.
func (m *Model) KeyHints() []ui.KeyHint {
	hints := make([]ui.KeyHint, 0, 4)
	if len(m.shells) > 0 {
		hints = append(hints,
			ui.KeyHint{Key: "type", Desc: "send keys"},
			ui.KeyHint{Key: "alt+w", Desc: "close"},
		)
	}
	hints = append(hints,
		ui.KeyHint{Key: "ctrl+t", Desc: "new shell"},
		ui.KeyHint{Key: "alt+[/]", Desc: "tabs"},
	)
	return hints
}

// Close releases every shell. The root model calls this when quitting.
func (m *Model) Close() {
	for _, s := range m.shells {
		closeShell(s)
	}
	m.shells = nil
	m.active = 0
}

func encodeKey(msg tea.KeyMsg) []byte {
	switch msg.String() {
	case "enter":
		return []byte("\r")
	case "tab":
		return []byte("\t")
	case "shift+tab":
		return []byte("\x1b[Z")
	case "backspace":
		return []byte("\x7f")
	case "esc":
		return []byte("\x1b")
	case " ":
		return []byte(" ")
	case "up":
		return []byte("\x1b[A")
	case "down":
		return []byte("\x1b[B")
	case "right":
		return []byte("\x1b[C")
	case "left":
		return []byte("\x1b[D")
	case "home":
		return []byte("\x1b[H")
	case "end":
		return []byte("\x1b[F")
	case "pgup":
		return []byte("\x1b[5~")
	case "pgdown":
		return []byte("\x1b[6~")
	case "insert":
		return []byte("\x1b[2~")
	case "delete":
		return []byte("\x1b[3~")
	case "f1":
		return []byte("\x1bOP")
	case "f2":
		return []byte("\x1bOQ")
	case "f3":
		return []byte("\x1bOR")
	case "f4":
		return []byte("\x1bOS")
	case "f5":
		return []byte("\x1b[15~")
	case "f6":
		return []byte("\x1b[17~")
	case "f7":
		return []byte("\x1b[18~")
	case "f8":
		return []byte("\x1b[19~")
	case "f9":
		return []byte("\x1b[20~")
	case "f10":
		return []byte("\x1b[21~")
	case "f11":
		return []byte("\x1b[23~")
	case "f12":
		return []byte("\x1b[24~")
	}
	t := msg.Type
	if t >= tea.KeyCtrlA && t <= tea.KeyCtrlZ {
		return []byte{byte(t-tea.KeyCtrlA) + 1}
	}
	switch msg.String() {
	case "ctrl+@", "ctrl+space":
		return []byte{0}
	case "ctrl+\\":
		return []byte{0x1c}
	case "ctrl+]":
		return []byte{0x1d}
	case "ctrl+^":
		return []byte{0x1e}
	case "ctrl+_":
		return []byte{0x1f}
	}
	if t == tea.KeyRunes {
		if msg.Alt {
			return append([]byte{0x1b}, []byte(string(msg.Runes))...)
		}
		return []byte(string(msg.Runes))
	}
	if t == tea.KeySpace {
		if msg.Alt {
			return []byte{0x1b, ' '}
		}
		return []byte{' '}
	}
	return nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
