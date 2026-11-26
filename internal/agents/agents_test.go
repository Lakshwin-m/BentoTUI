package agents

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/lakshwin/bento/internal/ui"
)

func TestDetectKeepsFullCatalog(t *testing.T) {
	look := func(bin string) (string, error) {
		switch bin {
		case "claude":
			return `C:\tools\claude.exe`, nil
		case "opencode":
			return `C:\Users\me\AppData\Roaming\npm\opencode.cmd`, nil
		default:
			return "", errors.New("missing")
		}
	}
	got := detect(look)
	if len(got) != len(known) {
		t.Fatalf("agents = %d, want %d", len(got), len(known))
	}
	if got[0].Name != "Claude Code" || !got[0].Installed() {
		t.Fatalf("claude = %+v", got[0])
	}
	if got[1].Name != "OpenCode" || !strings.HasSuffix(got[1].Path, "opencode.cmd") {
		t.Fatalf("opencode = %+v", got[1])
	}
	if got[2].Installed() {
		t.Fatalf("gemini should be missing: %+v", got[2])
	}
}

func TestViewAndLaunch(t *testing.T) {
	m := &Model{agents: []Agent{
		{Name: "Claude Code", Path: `C:\tools\claude.exe`},
		{Name: "OpenCode", Path: `C:\npm\opencode.cmd`},
	}}
	m.Update(ui.PaneSizeMsg{Width: 40, Height: 20})
	plain := ansi.Strip(m.View(40, 20, true))
	if !strings.Contains(plain, "Claude Code") || !strings.Contains(plain, "OpenCode") {
		t.Fatalf("view:\n%s", plain)
	}
	cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	if cmd != nil {
		t.Fatal("down should not launch")
	}
	cmd = m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter should launch")
	}
	msg := cmd().(LaunchMsg)
	if msg.Name != "OpenCode" || msg.Path != `C:\npm\opencode.cmd` {
		t.Fatalf("launch = %+v", msg)
	}
}

func TestClickLaunches(t *testing.T) {
	m := &Model{agents: []Agent{
		{Name: "Claude Code", Path: `C:\tools\claude.exe`},
		{Name: "OpenCode", Path: `C:\npm\opencode.cmd`},
	}}
	m.Update(ui.PaneSizeMsg{Width: 40, Height: 20})
	_, cmd := m.Update(ui.ClickMsg{X: 2, Y: 1 + agentRowH})
	if cmd == nil {
		t.Fatal("click should launch")
	}
	msg := cmd().(LaunchMsg)
	if msg.Name != "OpenCode" {
		t.Fatalf("launch = %+v", msg)
	}
}

func TestMissingAgentOffersInstall(t *testing.T) {
	m := &Model{agents: []Agent{
		{Name: "OpenCode", Bin: "opencode", Install: "npm install -g opencode-ai"},
	}}
	m.Update(ui.PaneSizeMsg{Width: 48, Height: 16})
	plain := ansi.Strip(m.View(48, 16, true))
	if !strings.Contains(plain, "not installed") {
		t.Fatalf("view:\n%s", plain)
	}
	cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter should offer install")
	}
	msg := cmd().(InstallMsg)
	if msg.Command != "npm install -g opencode-ai" {
		t.Fatalf("install = %+v", msg)
	}
}
