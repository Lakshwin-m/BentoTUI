// Package build implements the build/test runner pane. Detection happens once
// at construction; commands stream output through a channel so Update never
// blocks on the child process.
package build

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lakshwin/bento/internal/theme"
	"github.com/lakshwin/bento/internal/ui"
	"github.com/lakshwin/bento/internal/winutil"
)

// Task is one detected build/test command.
type Task struct {
	Name    string
	CmdLine string
}

// Event is one streamed line or the completion of a task.
type Event struct {
	Line string
	Done bool
	Err  error
}

// RunStartedMsg is returned when a task is launched; the pane then waits on
// the event channel.
type RunStartedMsg struct{}

// RunEventMsg carries one streamed line or completion.
type RunEventMsg struct {
	Line string
	Done bool
	Err  error
}

type Model struct {
	dir   string
	tasks []Task
	sel   int

	running bool
	output  []string
	offset  int
	follow  bool
	cancel  context.CancelFunc
	ch      chan Event

	width  int
	height int
}

const maxOutputLines = 5000

// New creates the build pane and detects tasks for dir.
func New(dir string) *Model {
	return &Model{dir: dir, tasks: Detect(dir), follow: true}
}

// Init implements ui.Pane.
func (m *Model) Init() tea.Cmd { return nil }

// Detect inspects the project and returns build/test tasks.
func Detect(dir string) []Task {
	var tasks []Task
	has := func(name string) bool {
		_, err := os.Stat(filepath.Join(dir, name))
		return err == nil
	}
	if has("go.mod") {
		tasks = append(tasks,
			Task{Name: "build", CmdLine: "go build ./..."},
			Task{Name: "test", CmdLine: "go test ./..."},
			Task{Name: "vet", CmdLine: "go vet ./..."},
		)
	}
	if has("package.json") {
		scripts := npmScripts(filepath.Join(dir, "package.json"))
		order := []string{"build", "test", "lint", "typecheck", "check"}
		added := map[string]bool{}
		for _, name := range order {
			if _, ok := scripts[name]; ok && !added[name] {
				label := name
				tasks = append(tasks, Task{Name: label, CmdLine: "npm run " + name})
				added[name] = true
			}
		}
		if len(added) == 0 {
			tasks = append(tasks, Task{Name: "test", CmdLine: "npm test"})
		}
	}
	if has("Cargo.toml") {
		tasks = append(tasks,
			Task{Name: "build", CmdLine: "cargo build"},
			Task{Name: "test", CmdLine: "cargo test"},
		)
	}
	if len(tasks) == 0 {
		python := has("pyproject.toml") || has("pytest.ini") || has("requirements.txt")
		if python {
			tasks = append(tasks, Task{Name: "test", CmdLine: "python -m pytest"})
		}
	}
	if has("Makefile") {
		tasks = append(tasks, Task{Name: "make", CmdLine: "make"})
		if makeHasTarget(filepath.Join(dir, "Makefile"), "test") {
			tasks = append(tasks, Task{Name: "make test", CmdLine: "make test"})
		}
	}
	return dedupe(tasks)
}

func dedupe(tasks []Task) []Task {
	seen := map[string]bool{}
	out := tasks[:0]
	for _, t := range tasks {
		if seen[t.CmdLine] {
			continue
		}
		seen[t.CmdLine] = true
		out = append(out, t)
	}
	return out
}

func npmScripts(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}
	return pkg.Scripts
}

func makeHasTarget(path, name string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), name+":") {
			return true
		}
	}
	return false
}

// Update implements ui.Pane.
func (m *Model) Update(msg tea.Msg) (ui.Pane, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.PaneSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clampOffset()
		return m, nil
	case RunStartedMsg:
		return m, m.waitEvent()
	case RunEventMsg:
		if msg.Done {
			m.running = false
			m.cancel = nil
			if msg.Err != nil {
				m.appendLine(theme.Cross + " " + msg.Err.Error())
			} else {
				m.appendLine(theme.Check + " done")
			}
			return m, nil
		}
		m.appendLine(msg.Line)
		if m.running {
			return m, m.waitEvent()
		}
		return m, nil
	case tea.KeyMsg:
		return m, m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) waitEvent() tea.Cmd {
	ch := m.ch
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return RunEventMsg{Done: true}
		}
		return RunEventMsg{Line: ev.Line, Done: ev.Done, Err: ev.Err}
	}
}

func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "s", "ctrl+c":
		if m.running {
			if m.cancel != nil {
				m.cancel()
			}
			m.appendLine(theme.MutedStyle().Render("stopping…"))
			return nil
		}
	case "enter":
		if !m.running {
			return m.start()
		}
	case "left", "h", "shift+tab":
		if !m.running && len(m.tasks) > 0 {
			m.sel = (m.sel - 1 + len(m.tasks)) % len(m.tasks)
		}
	case "right", "l", "tab":
		if !m.running && len(m.tasks) > 0 {
			m.sel = (m.sel + 1) % len(m.tasks)
		}
	case "up", "k":
		m.follow = false
		m.offset--
	case "down", "j":
		m.offset++
	case "pgup":
		m.follow = false
		m.offset -= m.bodyHeight()
	case "pgdown":
		m.offset += m.bodyHeight()
	case "g", "home":
		m.follow = false
		m.offset = 0
	case "G", "end":
		m.follow = true
	}
	m.clampOffset()
	return nil
}

func (m *Model) start() tea.Cmd {
	if m.running || len(m.tasks) == 0 {
		return nil
	}
	task := m.tasks[m.sel]
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan Event, 512)
	m.running = true
	m.cancel = cancel
	m.ch = ch
	m.output = nil
	m.offset = 0
	m.follow = true
	m.appendLine(theme.AccentStyle().Render("$ " + task.CmdLine))

	dir := m.dir
	go stream(ctx, dir, task.CmdLine, ch)
	return func() tea.Msg { return RunStartedMsg{} }
}

func stream(ctx context.Context, dir, line string, ch chan<- Event) {
	defer close(ch)
	cmd := exec.CommandContext(ctx, "cmd", "/c", line)
	cmd.Dir = dir
	winutil.HideWindow(cmd)
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		pw.Close()
		ch <- Event{Done: true, Err: err}
		return
	}
	scanDone := make(chan struct{})
	go func() {
		defer close(scanDone)
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 64*1024), 2*1024*1024)
		for sc.Scan() {
			ch <- Event{Line: sc.Text()}
		}
		pr.CloseWithError(sc.Err())
	}()
	err := cmd.Wait()
	pw.Close()
	<-scanDone
	ch <- Event{Done: true, Err: err}
}

func (m *Model) appendLine(s string) {
	m.output = append(m.output, s)
	if len(m.output) > maxOutputLines {
		m.output = m.output[len(m.output)-maxOutputLines:]
	}
	m.clampOffset()
}

func (m *Model) bodyHeight() int {
	h := m.height - ui.ChromeRows - 1
	if h < 1 {
		return 1
	}
	return h
}

func (m *Model) clampOffset() {
	h := m.bodyHeight()
	maxOffset := len(m.output) - h
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.follow {
		m.offset = maxOffset
	}
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

// View implements ui.Pane.
func (m *Model) View(width, height int, focused bool) string {
	innerW, innerH := ui.BodySize(width, height)

	var header string
	if len(m.tasks) == 0 {
		header = theme.MutedStyle().Render("no build system detected")
	} else {
		parts := make([]string, 0, len(m.tasks))
		for i, t := range m.tasks {
			if i == m.sel {
				parts = append(parts, theme.SelectedStyle().Render(" "+t.Name+" "))
			} else {
				parts = append(parts, theme.MutedStyle().Render(" "+t.Name+" "))
			}
		}
		header = strings.Join(parts, " ")
		if m.running {
			header = theme.AccentStyle().Render(theme.Bullet+" running ") + header
		}
	}

	var body string
	switch {
	case len(m.tasks) == 0:
		body = ui.CenteredCard(innerW, innerH-1, "Nothing to build", "Drop a go.mod, package.json, Cargo.toml or Makefile here.")
	case len(m.output) == 0 && !m.running:
		task := m.tasks[m.sel]
		body = ui.CenteredCard(innerW, innerH-1, "Run "+task.Name, task.CmdLine)
	default:
		visible := innerH - 1
		if visible < 1 {
			visible = 1
		}
		m.clampOffset()
		end := m.offset + visible
		if end > len(m.output) {
			end = len(m.output)
		}
		lines := make([]string, 0, visible)
		for i := m.offset; i < end; i++ {
			lines = append(lines, ui.Fit(m.styleOutput(m.output[i]), innerW))
		}
		for len(lines) < visible {
			lines = append(lines, ui.Fit("", innerW))
		}
		body = strings.Join(lines, "\n")
	}

	content := ui.Fit(header, innerW) + "\n" + body
	return ui.Panel(ui.Spec(m.Title(), m.KeyHints()), width, height, focused, content)
}

func (m *Model) styleOutput(line string) string {
	lower := strings.ToLower(line)
	switch {
	case strings.HasPrefix(line, theme.Check):
		return theme.SuccessStyle().Render(line)
	case strings.HasPrefix(line, theme.Cross):
		return theme.ErrorStyle().Render(line)
	case strings.HasPrefix(line, "$ "):
		return line
	case strings.Contains(lower, "error") || strings.Contains(lower, "fail"):
		return theme.ErrorStyle().Render(line)
	case strings.HasPrefix(strings.TrimSpace(lower), "ok"):
		return theme.SuccessStyle().Render(line)
	default:
		return theme.TextStyle().Render(line)
	}
}

// Title implements ui.Pane.
func (m *Model) Title() string {
	if m.running && len(m.tasks) > 0 {
		return "Build: " + m.tasks[m.sel].Name
	}
	return "Build"
}

// KeyHints implements ui.Pane.
func (m *Model) KeyHints() []ui.KeyHint {
	if len(m.tasks) == 0 {
		return nil
	}
	if m.running {
		return []ui.KeyHint{
			{Key: "ctrl+c", Desc: "cancel"},
			{Key: "↑↓", Desc: "output"},
		}
	}
	return []ui.KeyHint{
		{Key: "←→", Desc: "task"},
		{Key: "enter", Desc: "run"},
		{Key: "↑↓", Desc: "output"},
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
