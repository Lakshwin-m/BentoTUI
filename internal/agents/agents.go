// Package agents implements the agent launcher pane. It detects installed
// coding-agent CLIs and asks the root model to open one in a console tab.
package agents

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lakshwin/bento/internal/theme"
	"github.com/lakshwin/bento/internal/ui"
	"github.com/lakshwin/bento/internal/winutil"
)

const agentRowH = 2

// Agent is one catalog entry.
type Agent struct {
	Name    string
	Bin     string
	Path    string
	Install string
}

// Installed reports whether the CLI was found on PATH.
func (a Agent) Installed() bool { return a.Path != "" }

// LaunchMsg asks the root model to start a console tab for an agent.
type LaunchMsg struct {
	Name string
	Path string
}

// InstallMsg asks the root model to confirm and run a vendor install command.
type InstallMsg struct {
	Name    string
	Command string
}

type candidate struct {
	name    string
	bin     string
	install string
}

var known = []candidate{
	{name: "Claude Code", bin: "claude", install: "npm install -g @anthropic-ai/claude-code"},
	{name: "OpenCode", bin: "opencode", install: "npm install -g opencode-ai"},
	{name: "Gemini CLI", bin: "gemini", install: "npm install -g @google/gemini-cli"},
	{name: "Codex", bin: "codex", install: "npm install -g @openai/codex"},
	{name: "GitHub Copilot", bin: "copilot", install: "npm install -g @github/copilot"},
	{name: "Crush", bin: "crush", install: "go install github.com/charmbracelet/crush@latest"},
	{name: "Aider", bin: "aider", install: "pip install aider-chat"},
	{name: "Cursor Agent", bin: "cursor-agent", install: "Install Cursor, then ensure cursor-agent is on PATH"},
}

// Model is the agent launcher pane.
type Model struct {
	agents []Agent
	cursor int

	width  int
	height int
	scroll int
}

// New detects installed agents.
func New() *Model {
	return &Model{agents: detect(winutil.LookCLI)}
}

func detect(look func(string) (string, error)) []Agent {
	out := make([]Agent, 0, len(known))
	for _, c := range known {
		a := Agent{Name: c.name, Bin: c.bin, Install: c.install}
		if path, err := look(c.bin); err == nil && path != "" {
			a.Path = winutil.PreferWindowsLauncher(path)
		}
		out = append(out, a)
	}
	return out
}

// Init implements ui.Pane.
func (m *Model) Init() tea.Cmd { return nil }

// Update implements ui.Pane.
func (m *Model) Update(msg tea.Msg) (ui.Pane, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.PaneSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clamp()
		return m, nil
	case ui.ClickMsg:
		return m, m.handleClick(msg)
	case tea.KeyMsg:
		return m, m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleClick(msg ui.ClickMsg) tea.Cmd {
	innerY := msg.Y - 1
	if innerY < 0 {
		return nil
	}
	idx := m.scroll + innerY/agentRowH
	if idx < 0 || idx >= len(m.agents) {
		return nil
	}
	m.cursor = idx
	m.clamp()
	return m.activate()
}

func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.agents)-1 {
			m.cursor++
		}
	case "g", "home":
		m.cursor = 0
	case "G", "end":
		if len(m.agents) > 0 {
			m.cursor = len(m.agents) - 1
		}
	case "enter", "o":
		return m.activate()
	}
	m.clamp()
	return nil
}

func (m *Model) activate() tea.Cmd {
	if m.cursor < 0 || m.cursor >= len(m.agents) {
		return nil
	}
	a := m.agents[m.cursor]
	if a.Installed() {
		return func() tea.Msg { return LaunchMsg{Name: a.Name, Path: a.Path} }
	}
	if a.Install == "" {
		return nil
	}
	return func() tea.Msg { return InstallMsg{Name: a.Name, Command: a.Install} }
}

func (m *Model) clamp() {
	innerH := max(0, m.height-ui.ChromeRows)
	rows := innerH / agentRowH
	if rows < 1 {
		rows = 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.agents) && len(m.agents) > 0 {
		m.cursor = len(m.agents) - 1
	}
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+rows {
		m.scroll = m.cursor - rows + 1
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
}

// View implements ui.Pane.
func (m *Model) View(width, height int, focused bool) string {
	innerW, innerH := ui.BodySize(width, height)
	spec := ui.Spec(m.Title(), m.KeyHints())
	if len(m.agents) == 0 {
		body := ui.CenteredCard(innerW, innerH, "No agents in catalog", "")
		return ui.Panel(spec, width, height, focused, body)
	}
	lines := make([]string, 0, innerH)
	end := m.scroll + (innerH+agentRowH-1)/agentRowH
	if end > len(m.agents) {
		end = len(m.agents)
	}
	for i := m.scroll; i < end; i++ {
		a := m.agents[i]
		marker := " "
		if i == m.cursor {
			marker = theme.AccentBar
		}
		label := " " + marker + " " + a.Name
		detail := a.Install
		if a.Installed() {
			detail = shortPath(a.Path)
		} else {
			detail = "not installed  " + a.Install
		}
		detail = "   " + detail
		switch {
		case i == m.cursor && focused:
			lines = append(lines, theme.SelectedAccentStyle().Render(ui.Fit(label, innerW)))
			lines = append(lines, theme.SelectedMutedStyle().Render(ui.Fit(detail, innerW)))
		case i == m.cursor:
			lines = append(lines, theme.AccentStyle().Render(ui.Fit(label, innerW)))
			lines = append(lines, theme.MutedStyle().Render(ui.Fit(detail, innerW)))
		default:
			lines = append(lines, theme.TextStyle().Render(ui.Fit(label, innerW)))
			lines = append(lines, theme.MutedStyle().Render(ui.Fit(detail, innerW)))
		}
	}
	for len(lines) < innerH {
		lines = append(lines, ui.Fit("", innerW))
	}
	if len(lines) > innerH {
		lines = lines[:innerH]
	}
	return ui.Panel(spec, width, height, focused, strings.Join(lines, "\n"))
}

// Title implements ui.Pane.
func (m *Model) Title() string {
	if len(m.agents) == 0 {
		return "Agents"
	}
	installed := 0
	for _, a := range m.agents {
		if a.Installed() {
			installed++
		}
	}
	return fmt.Sprintf("Agents %d/%d", installed, len(m.agents))
}

// KeyHints implements ui.Pane.
func (m *Model) KeyHints() []ui.KeyHint {
	return []ui.KeyHint{
		{Key: "↑↓", Desc: "select"},
		{Key: "enter", Desc: "launch"},
		{Key: "click", Desc: "open"},
	}
}

func shortPath(p string) string {
	p = strings.ReplaceAll(p, `\`, "/")
	if len(p) <= 40 {
		return p
	}
	return theme.Ellipsis + p[len(p)-39:]
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
