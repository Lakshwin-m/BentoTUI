package git

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/lakshwin/bento/internal/ui"
)

func TestParsePorcelainBranchAndEntries(t *testing.T) {
	out := strings.Join([]string{
		"## main...origin/main [ahead 2, behind 1]",
		" M file.txt",
		"?? new.go",
		"R  new name.go",
		"old name.go",
		"",
	}, "\x00")
	st := parsePorcelain(out)
	if !st.IsRepo {
		t.Fatal("expected repo")
	}
	if st.Branch != "main" || st.Ahead != 2 || st.Behind != 1 {
		t.Fatalf("header = %+v", st)
	}
	if len(st.Entries) != 3 {
		t.Fatalf("entries = %+v", st.Entries)
	}
	var renamed *Entry
	for i := range st.Entries {
		if st.Entries[i].Path == "new name.go" {
			renamed = &st.Entries[i]
		}
	}
	if renamed == nil || renamed.Orig != "old name.go" {
		t.Fatalf("rename entry = %+v", renamed)
	}
	if renamed.Index != 'R' {
		t.Fatalf("rename status = %q", string(renamed.Index))
	}
}

func TestParsePorcelainDetachedAndNoCommits(t *testing.T) {
	st := parsePorcelain("## HEAD (no branch)\x00")
	if st.Branch != "detached" {
		t.Fatalf("branch = %q", st.Branch)
	}
	st = parsePorcelain("## No commits yet on trunk\x00")
	if st.Branch != "trunk" {
		t.Fatalf("branch = %q", st.Branch)
	}
}

func TestModelSelectionAndOpen(t *testing.T) {
	m := New(`C:\proj`)
	m.Update(ui.PaneSizeMsg{Width: 40, Height: 20})
	m.Update(LoadedMsg{Dir: `C:\proj`, Status: Status{
		IsRepo: true, Branch: "feat",
		Entries: []Entry{{Index: ' ', Worktree: 'M', Path: "a/b.go"}},
	}})
	plain := ansi.Strip(m.View(40, 20, true))
	if !strings.Contains(plain, "feat") || !strings.Contains(plain, "a/b.go") {
		t.Fatalf("view:\n%s", plain)
	}
	cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter should open")
	}
	msg := cmd().(OpenMsg)
	if msg.Path != filepath.Join(`C:\proj`, "a", "b.go") {
		t.Fatalf("path = %q", msg.Path)
	}
}

func TestModelNotRepo(t *testing.T) {
	m := New(`C:\proj`)
	m.Update(ui.PaneSizeMsg{Width: 40, Height: 20})
	m.Update(LoadedMsg{Dir: `C:\proj`, Status: Status{}})
	plain := ansi.Strip(m.View(40, 20, true))
	if !strings.Contains(plain, "Not a git repository") {
		t.Fatalf("view:\n%s", plain)
	}
}

func TestRefreshCommand(t *testing.T) {
	m := New(`C:\proj`)
	m.Update(LoadedMsg{Dir: `C:\proj`, Status: Status{IsRepo: true, Branch: "main"}})
	if cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")}); cmd == nil {
		t.Fatal("r should refresh")
	}
	if m.KeyHints() == nil {
		t.Fatal("missing hints")
	}
	if m.Title() != "Changes" {
		t.Fatalf("title = %q", m.Title())
	}
}
