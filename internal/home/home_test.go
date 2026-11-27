package home

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestViewExactSize(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {120, 40}, {60, 20}} {
		m := New("dev", nil)
		m.Update(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		out := m.View()
		if got := lipgloss.Width(out); got != sz[0] {
			t.Errorf("%dx%d width = %d", sz[0], sz[1], got)
		}
		if got := lipgloss.Height(out); got != sz[1] {
			t.Errorf("%dx%d height = %d", sz[0], sz[1], got)
		}
		plain := strings.ToLower(ansi.Strip(out))
		if !strings.Contains(plain, "bento") || !strings.Contains(plain, "recent projects") {
			t.Errorf("%dx%d missing chrome:\n%s", sz[0], sz[1], plain)
		}
	}
}

func TestViewWithRecents(t *testing.T) {
	m := New("1.2.3", []string{`C:\work\bento`, `C:\work\demo`})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	out := m.View()
	if got := lipgloss.Width(out); got != 80 {
		t.Errorf("width = %d", got)
	}
	if got := lipgloss.Height(out); got != 24 {
		t.Errorf("height = %d", got)
	}
	plain := strings.ToLower(ansi.Strip(out))
	for _, want := range []string{"bento", "recent projects", "1.2.3", "1-9", "open folder", "clone repo"} {
		if !strings.Contains(plain, want) {
			t.Errorf("missing %q:\n%s", want, plain)
		}
	}
}
