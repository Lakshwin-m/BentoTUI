package viewer

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	githubansi "github.com/charmbracelet/x/ansi"

	"github.com/lakshwin/bento/internal/ui"
)

func TestIndentRegions(t *testing.T) {
	lines := []string{
		"def f():",
		"    a = 1",
		"    if a:",
		"        b = 2",
		"    return a",
		"x = 1",
	}
	regions := computeFoldRegions(lines, "script.py")
	if len(regions) != 2 {
		t.Fatalf("regions = %+v", regions)
	}
	if regions[0] != (FoldRegion{Start: 0, End: 5}) {
		t.Errorf("outer = %+v", regions[0])
	}
	if regions[1] != (FoldRegion{Start: 2, End: 4}) {
		t.Errorf("inner = %+v", regions[1])
	}
}

func TestBraceRegionsIgnoreStringsAndComments(t *testing.T) {
	lines := []string{
		"func f() {",
		`	s := "{"`,
		"	// }",
		"	if true {",
		"		println(s)",
		"	}",
		"}",
	}
	regions := computeFoldRegions(lines, "a.go")
	if len(regions) != 2 {
		t.Fatalf("regions = %+v", regions)
	}
	if regions[0] != (FoldRegion{Start: 0, End: 7}) {
		t.Errorf("outer = %+v", regions[0])
	}
	if regions[1] != (FoldRegion{Start: 3, End: 6}) {
		t.Errorf("inner = %+v", regions[1])
	}
}

func TestFoldedOnlyOpensStringsDoNotFold(t *testing.T) {
	lines := []string{`s := "{"`, `println(s)`}
	if regions := computeFoldRegions(lines, "a.go"); len(regions) != 0 {
		t.Fatalf("regions = %+v", regions)
	}
}

func TestToggleFoldAffectsDisplayAndNavigation(t *testing.T) {
	m := New()
	m.Update(ui.PaneSizeMsg{Width: 60, Height: 20})
	path := writeTemp(t, "f.go", "func a() {\n\tb()\n\tc()\n}\nfunc d() {}")
	m.Update(m.Open(path)())
	if len(m.display) != 5 {
		t.Fatalf("display = %d", len(m.display))
	}
	m.toggleFold()
	if len(m.display) != 2 {
		t.Fatalf("folded display = %d", len(m.display))
	}
	plain := githubansi.Strip(m.View(60, 20, true))
	if !strings.Contains(plain, "3 lines") {
		t.Errorf("missing fold summary:\n%s", plain)
	}
	m.moveCursor(1)
	if m.cursor != 4 {
		t.Errorf("down from fold: cursor = %d", m.cursor)
	}
	m.moveCursor(-1)
	if m.cursor != 0 {
		t.Errorf("up: cursor = %d", m.cursor)
	}
	m.toggleFold()
	if len(m.display) != 5 {
		t.Fatalf("unfolded display = %d", len(m.display))
	}
}

func TestFoldAllAndReveal(t *testing.T) {
	m := New()
	m.Update(ui.PaneSizeMsg{Width: 60, Height: 20})
	path := writeTemp(t, "f.go", "func a() {\n\tmatch()\n}")
	m.Update(m.Open(path)())
	m.toggleAllFolds()
	if len(m.display) != 1 {
		t.Fatalf("fold all display = %d", len(m.display))
	}
	m.toggleAllFolds()
	if len(m.display) != 3 {
		t.Fatalf("unfold all display = %d", len(m.display))
	}

	m.toggleAllFolds()
	m.StartSearch()
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("match")})
	if len(m.matches) != 1 {
		t.Fatalf("matches = %d", len(m.matches))
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.display) != 3 {
		t.Fatalf("search jump must reveal folds, display = %d", len(m.display))
	}
	if m.cursor != 1 {
		t.Errorf("cursor = %d", m.cursor)
	}
}
