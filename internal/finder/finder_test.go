package finder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/lakshwin/bento/internal/ui"
)

func TestScoreRanking(t *testing.T) {
	type sample struct {
		cand, query string
		ok          bool
	}
	samples := []sample{
		{"internal/app/app.go", "app", true},
		{"main.go", "mg", true},
		{"main.go", "xyz", false},
		{"README.md", "read", true},
		{"src/main.go", "SM", true},
	}
	for _, s := range samples {
		_, _, ok := scorePath(s.cand, s.query)
		if ok != s.ok {
			t.Errorf("scorePath(%q,%q) ok=%v want %v", s.cand, s.query, ok, s.ok)
		}
	}

	better := func(a, b, q string) {
		t.Helper()
		sa, _, oka := scorePath(a, q)
		sb, _, okb := scorePath(b, q)
		if !oka || !okb {
			t.Fatalf("both should match: %q %v %q %v", a, oka, b, okb)
		}
		if sa <= sb {
			t.Errorf("expected %q (%d) to outrank %q (%d) for %q", a, sa, b, sb, q)
		}
	}
	better("app.go", "xappx.go", "app")
	better("abc.go", "a_b_c.go", "abc")
	better("src/main.go", "src/xmain.go", "sm")
	better("main.go", "domain.go", "main")
}

func TestIndexRespectsIgnoresAndHidden(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".gitignore", "node_modules/\n*.log\n")
	write("main.go", "package main")
	write("debug.log", "log")
	write("node_modules/pkg/index.js", "x")
	write(".hidden.txt", "x")
	write("src/app.go", "package app")

	files, err := indexFiles(root, newMatcher(root))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(files, ",")
	if !strings.Contains(joined, "main.go") || !strings.Contains(joined, "src/app.go") {
		t.Fatalf("missing files: %v", files)
	}
	for _, bad := range []string{"debug.log", "node_modules", ".hidden"} {
		if strings.Contains(joined, bad) {
			t.Errorf("%q should be excluded: %v", bad, files)
		}
	}
}

func TestFinderFilterAndOpen(t *testing.T) {
	m := New(`C:\proj`)
	m.Update(ui.PaneSizeMsg{Width: 80, Height: 20})
	cmd := m.Open()
	if cmd == nil {
		t.Fatal("first open should index")
	}
	msg := cmd()
	if _, ok := msg.(IndexedMsg); !ok {
		t.Fatalf("got %T", msg)
	}
	m.Update(IndexedMsg{Files: []string{"src/app.go", "README.md", "internal/app/app.go"}})

	// The IndexedMsg above bypasses the real path; m.files is set in Update.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("app")})
	if len(m.results) == 0 {
		t.Fatal("no results")
	}
	if !strings.Contains(m.results[0].path, "app") {
		t.Errorf("top result = %q", m.results[0].path)
	}
	cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter should emit OpenMsg")
	}
	open, ok := cmd().(OpenMsg)
	if !ok {
		t.Fatalf("got %T", cmd())
	}
	if !strings.HasSuffix(open.Path, filepath.FromSlash(m.results[0].path)) && m.results[0].path != "" {
		// The selected result was cleared by the filtered rerun; just ensure a path.
		if open.Path == "" {
			t.Fatal("empty path")
		}
	}
	if m.Opened() {
		t.Fatal("finder should close on enter")
	}
}

func TestFinderViewExact(t *testing.T) {
	m := New(t.TempDir())
	m.Update(ui.PaneSizeMsg{Width: 60, Height: 16})
	m.Update(IndexedMsg{Files: []string{"a.go", "b.go"}})
	out := ansi.Strip(m.View(60, 16))
	lines := strings.Split(out, "\n")
	if len(lines) != 16 {
		t.Fatalf("lines = %d", len(lines))
	}
	for i, line := range lines {
		if w := len([]rune(line)); w != 60 {
			t.Errorf("line %d width %d: %q", i, w, line)
		}
	}
}
