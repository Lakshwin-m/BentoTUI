package palette

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lakshwin/bento/internal/ui"
)

func typeRunes(m *Model, s string) {
	for _, r := range s {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

func TestPaletteFiltersAndRuns(t *testing.T) {
	m := New()
	m.Update(ui.PaneSizeMsg{Width: 60, Height: 12})
	m.Open([]Command{
		{ID: "tree", Title: "Focus Tree", Description: "file tree", Keys: "alt+1"},
		{ID: "quit", Title: "Quit Bento", Description: "close workspace", Keys: "ctrl+q"},
	})
	if !m.Opened() {
		t.Fatal("palette should be open")
	}
	typeRunes(m, "tree")
	if len(m.results) != 1 || m.results[0].ID != "tree" {
		t.Fatalf("results = %+v", m.results)
	}
	cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter should emit a command")
	}
	msg, ok := cmd().(RunMsg)
	if !ok || msg.ID != "tree" {
		t.Fatalf("msg = %#v", msg)
	}
	if m.Opened() {
		t.Fatal("palette should close after running")
	}
}

func TestPaletteEscapeCloses(t *testing.T) {
	m := New()
	m.Update(ui.PaneSizeMsg{Width: 60, Height: 12})
	m.Open([]Command{{ID: "a", Title: "Alpha"}})
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.Opened() {
		t.Fatal("esc should close the palette")
	}
}

func TestPaletteViewExactSize(t *testing.T) {
	m := New()
	m.Update(ui.PaneSizeMsg{Width: 50, Height: 12})
	m.Open([]Command{{ID: "a", Title: "Alpha", Description: "first", Keys: "x"}})
	out := m.View(50, 12)
	lines := strings.Split(out, "\n")
	if len(lines) != 12 {
		t.Fatalf("lines = %d", len(lines))
	}
	for i, line := range lines {
		if got := lipgloss.Width(line); got != 50 {
			t.Errorf("line %d width %d", i, got)
		}
	}
}
