package viewer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	githubansi "github.com/charmbracelet/x/ansi"

	"github.com/lakshwin/bento/internal/ui"
)

func keyMsg(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func writeTemp(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadTextNormalizesCRLF(t *testing.T) {
	path := writeTemp(t, "main.go", "package main\r\n\r\nfunc main() {}\r\n")
	msg := loadFile(path, false, MaxFileSize)
	if msg.Err != nil {
		t.Fatal(msg.Err)
	}
	if msg.Binary || msg.TooLarge {
		t.Fatalf("unexpected flags: %+v", msg)
	}
	want := []string{"package main", "", "func main() {}", ""}
	if strings.Join(msg.Lines, "|") != strings.Join(want, "|") {
		t.Fatalf("lines = %#v", msg.Lines)
	}
	if len(msg.Styled) != len(msg.Lines) {
		t.Fatalf("styled %d != lines %d", len(msg.Styled), len(msg.Lines))
	}
	if !strings.Contains(githubansi.Strip(msg.Styled[0]), "package") {
		t.Errorf("highlight lost text: %q", msg.Styled[0])
	}
}

func TestLoadBinaryDetected(t *testing.T) {
	path := writeTemp(t, "blob.bin", "abc\x00def")
	msg := loadFile(path, false, MaxFileSize)
	if msg.Err != nil {
		t.Fatal(msg.Err)
	}
	if !msg.Binary {
		t.Fatal("expected binary detection")
	}
	if msg.Size != 7 {
		t.Errorf("size = %d", msg.Size)
	}
}

func TestLoadTooLargeAndForce(t *testing.T) {
	path := writeTemp(t, "big.txt", "hello world")
	msg := loadFile(path, false, 5)
	if !msg.TooLarge {
		t.Fatalf("expected TooLarge, got %+v", msg)
	}
	forced := loadFile(path, true, 5)
	if forced.TooLarge || forced.Err != nil {
		t.Fatalf("forced load failed: %+v", forced)
	}
	if len(forced.Lines) == 0 {
		t.Fatal("forced load returned no lines")
	}
}

func TestLoadMissingAndDirectory(t *testing.T) {
	msg := loadFile(filepath.Join(t.TempDir(), "nope"), false, MaxFileSize)
	if msg.Err == nil {
		t.Fatal("missing file should error")
	}
	msg = loadFile(t.TempDir(), false, MaxFileSize)
	if msg.Err == nil {
		t.Fatal("directory should error")
	}
}

func TestHighlightLineCount(t *testing.T) {
	src := "package main\n\nfunc main() {}\n"
	lines := HighlightLines(src, "x.go")
	if len(lines) != 4 {
		t.Fatalf("highlighted lines = %d, want 4", len(lines))
	}
	for i, line := range lines {
		if got := strings.Count(line, "\n"); got != 0 {
			t.Errorf("line %d contains newline", i)
		}
	}
}

func TestViewerEmptyAndDimensions(t *testing.T) {
	m := New()
	m.Update(ui.PaneSizeMsg{Width: 60, Height: 20})
	empty := githubansi.Strip(m.View(60, 20, true))
	if !strings.Contains(empty, "No file open") {
		t.Fatalf("empty state = %q", empty)
	}
	if len(strings.Split(empty, "\n")) != 20 {
		t.Fatal("empty state wrong height")
	}

	path := writeTemp(t, "many.txt", strings.Repeat("line\n", 100))
	cmd := m.Open(path)
	m.Update(cmd())
	out := githubansi.Strip(m.View(60, 20, true))
	lines := strings.Split(out, "\n")
	if len(lines) != 20 {
		t.Fatalf("lines = %d", len(lines))
	}
	for i, line := range lines {
		if lipgloss.Width(line) != 60 {
			t.Errorf("line %d width %d", i, lipgloss.Width(line))
		}
	}
}

func TestViewerBinaryAndTooLargeStates(t *testing.T) {
	m := New()
	m.Update(ui.PaneSizeMsg{Width: 60, Height: 20})
	m.Open("x")
	m.Update(FileLoadedMsg{Path: "x", Binary: true, Size: 2048})
	if !strings.Contains(githubansi.Strip(m.View(60, 20, true)), "Binary file") {
		t.Fatal("binary state missing")
	}

	m.Open("y")
	m.Update(FileLoadedMsg{Path: "y", TooLarge: true, Size: 9 << 20})
	out := githubansi.Strip(m.View(60, 20, true))
	if !strings.Contains(out, "load anyway") {
		t.Fatalf("too-large state = %q", out)
	}
}

func TestSearchIncrementalAndNavigation(t *testing.T) {
	m := New()
	m.Update(ui.PaneSizeMsg{Width: 60, Height: 20})
	path := writeTemp(t, "s.txt", "alpha\nbeta\nalpha beta\nALPHA\n")
	m.Update(m.Open(path)())
	if !m.StartSearch() {
		t.Fatal("StartSearch failed")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("alpha")})
	if len(m.matches) != 3 {
		t.Fatalf("matches = %d, want 3", len(m.matches))
	}
	if m.matchIdx != 0 {
		t.Fatalf("matchIdx = %d", m.matchIdx)
	}
	prompt := githubansi.Strip(m.View(60, 20, true))
	if !strings.Contains(prompt, "/ alpha") || !strings.Contains(prompt, "1/3") {
		t.Errorf("prompt missing query/count: %q", prompt)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.Searching() {
		t.Fatal("enter should leave search input")
	}
	m.Update(keyMsg("n"))
	if m.matchIdx != 1 {
		t.Errorf("n: matchIdx = %d", m.matchIdx)
	}
	m.Update(keyMsg("N"))
	if m.matchIdx != 0 {
		t.Errorf("N: matchIdx = %d", m.matchIdx)
	}
	if !m.HasQuery() {
		t.Fatal("highlights should persist after enter")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.HasQuery() {
		t.Fatal("esc should clear highlights")
	}
}

func TestOpenClearsSearch(t *testing.T) {
	m := New()
	m.Update(ui.PaneSizeMsg{Width: 60, Height: 20})
	path := writeTemp(t, "a.txt", "hello\n")
	m.Update(m.Open(path)())
	m.StartSearch()
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hel")})
	if len(m.matches) == 0 {
		t.Fatal("expected matches")
	}
	m.Update(m.Open(path)())
	if m.HasQuery() || m.Searching() {
		t.Fatal("opening a file must clear search state")
	}
}

func TestViewerScrollKeepsCursorVisible(t *testing.T) {
	m := New()
	m.Update(ui.PaneSizeMsg{Width: 60, Height: 12})
	path := writeTemp(t, "many.txt", strings.Repeat("line\n", 200))
	m.Update(m.Open(path)())
	m.Update(keyMsg("G"))
	if m.cursor != len(m.lines)-1 {
		t.Fatalf("cursor = %d, want %d", m.cursor, len(m.lines)-1)
	}
	if m.scroll == 0 {
		t.Fatal("scroll did not follow cursor")
	}
	m.Update(keyMsg("g"))
	if m.cursor != 0 || m.scroll != 0 {
		t.Fatalf("g: cursor=%d scroll=%d", m.cursor, m.scroll)
	}
}
