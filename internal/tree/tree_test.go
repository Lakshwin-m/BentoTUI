package tree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/lakshwin/bento/internal/ui"
)

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mkdir := func(rel string) {
		if err := os.MkdirAll(filepath.Join(root, rel), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(rel, body string) {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".gitignore", "ignored.txt\nbuild/\n")
	write("b.txt", "b")
	write("Zebra.txt", "z")
	write("apple.txt", "a")
	write("ignored.txt", "ignored")
	write(".hidden", "h")
	mkdir("a")
	write("a/z.go", "package z")
	mkdir("build")
	write("build/out.bin", "x")
	mkdir(".git")
	write(".git/HEAD", "ref")
	return root
}

func loadRoot(t *testing.T, m *Model) {
	t.Helper()
	cmd := m.Init()
	msg := runCmd(t, cmd)
	if _, ok := msg.(DirLoadedMsg); !ok {
		t.Fatalf("expected DirLoadedMsg, got %T", msg)
	}
	m.Update(msg)
}

func runCmd(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	return cmd()
}

func flatNames(m *Model) []string {
	out := make([]string, len(m.flat))
	for i, r := range m.flat {
		out[i] = r.node.name
	}
	return out
}

func TestTreeInitialLoad(t *testing.T) {
	m := New(fixture(t))
	m.Update(ui.PaneSizeMsg{Width: 40, Height: 20})
	loadRoot(t, m)

	got := flatNames(m)
	want := []string{"a", "apple.txt", "b.txt", "Zebra.txt"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("flat = %v, want %v", got, want)
	}
	for _, bad := range []string{".git", ".gitignore", ".hidden", "ignored.txt", "build"} {
		for _, n := range got {
			if n == bad {
				t.Errorf("%q should be hidden/ignored", bad)
			}
		}
	}
}

func TestTreeLazyExpandCollapse(t *testing.T) {
	m := New(fixture(t))
	m.Update(ui.PaneSizeMsg{Width: 40, Height: 20})
	loadRoot(t, m)

	m.sel = 0 // "a"
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	msg := runCmd(t, cmd)
	if msg == nil {
		t.Fatal("expanding an unloaded dir should return a load command")
	}
	m.Update(msg)
	if len(m.flat) != 5 || m.flat[1].node.name != "z.go" {
		t.Fatalf("expanded flat = %v", flatNames(m))
	}
	if m.flat[1].depth != 1 {
		t.Errorf("child depth = %d, want 1", m.flat[1].depth)
	}

	m.sel = 0
	m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if len(m.flat) != 4 {
		t.Fatalf("collapsed flat = %v", flatNames(m))
	}
}

func TestTreeEnterEmitsOpenFile(t *testing.T) {
	m := New(fixture(t))
	m.Update(ui.PaneSizeMsg{Width: 40, Height: 20})
	loadRoot(t, m)

	for i, r := range m.flat {
		if r.node.name == "b.txt" {
			m.sel = i
		}
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := runCmd(t, cmd)
	open, ok := msg.(OpenFileMsg)
	if !ok {
		t.Fatalf("expected OpenFileMsg, got %T", msg)
	}
	if filepath.Base(open.Path) != "b.txt" {
		t.Errorf("opened %q", open.Path)
	}
}

func TestTreeHiddenToggleShowsDotfiles(t *testing.T) {
	m := New(fixture(t))
	m.Update(ui.PaneSizeMsg{Width: 40, Height: 20})
	loadRoot(t, m)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'.'}})
	if cmd == nil {
		t.Fatal("hidden toggle should reload")
	}
	msg := runCmd(t, cmd)
	if d, ok := msg.(DirLoadedMsg); !ok || d.Err != nil {
		t.Fatalf("reload failed: %#v", msg)
	}
	m.Update(msg)
	names := flatNames(m)
	found := false
	for _, n := range names {
		if n == ".hidden" {
			found = true
		}
	}
	if !found {
		t.Fatalf("hidden file missing after toggle: %v", names)
	}
}

func TestTreeViewDimensions(t *testing.T) {
	m := New(fixture(t))
	m.Update(ui.PaneSizeMsg{Width: 40, Height: 20})
	loadRoot(t, m)
	out := ansi.Strip(m.View(40, 20, true))
	lines := strings.Split(out, "\n")
	if len(lines) != 20 {
		t.Fatalf("lines = %d", len(lines))
	}
	for i, line := range lines {
		if lipgloss.Width(line) != 40 {
			t.Errorf("line %d width %d", i, lipgloss.Width(line))
		}
	}
}
