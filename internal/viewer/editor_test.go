package viewer

import (
	"os"
	"strings"
	"testing"

	githubansi "github.com/charmbracelet/x/ansi"

	"github.com/lakshwin/bento/internal/ui"
)

func TestEditingTypesSavesAndPreservesTrailingNewline(t *testing.T) {
	m := New()
	m.Update(ui.PaneSizeMsg{Width: 60, Height: 20})
	path := writeTemp(t, "e.txt", "one\ntwo\n")
	m.Update(m.Open(path)())
	if !m.StartEditing() {
		t.Fatal("StartEditing failed")
	}
	m.cursor = 1
	m.cursorCol = 3
	m.insertText("!")
	if m.lines[1] != "two!" {
		t.Fatalf("line = %q", m.lines[1])
	}
	m.insertNewline()
	if m.cursor != 2 || m.lines[2] != "" {
		t.Fatalf("newline: cursor=%d lines=%q", m.cursor, m.lines)
	}
	m.insertText("three")
	if !strings.Contains(m.Title(), "●") {
		t.Error("dirty title marker missing")
	}
	cmd := m.Save()
	if cmd == nil {
		t.Fatal("Save returned nil")
	}
	m.Update(cmd())
	if m.Dirty() {
		t.Error("still dirty after save")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "one\ntwo!\nthree\n" {
		t.Fatalf("saved = %q", string(data))
	}
	if strings.Contains(m.Title(), "●") {
		t.Error("dirty marker should clear after save")
	}
}

func TestEditingBackspaceJoinsLinesAndIndent(t *testing.T) {
	m := New()
	m.Update(ui.PaneSizeMsg{Width: 60, Height: 20})
	path := writeTemp(t, "i.py", "def f():\n    pass")
	m.Update(m.Open(path)())
	m.StartEditing()
	m.moveLine(1)
	m.cursorCol = 0
	m.backspace()
	if len(m.lines) != 1 || m.lines[0] != "def f():    pass" {
		t.Fatalf("join = %q", m.lines)
	}
	if m.cursorCol != len("def f():") {
		t.Fatalf("cursorCol = %d", m.cursorCol)
	}

	m.cursor = 0
	m.cursorCol = len("def f():")
	m.insertNewline()
	if m.lines[0] != "def f():" || m.lines[1] != "    pass" || m.cursor != 1 {
		t.Fatalf("newline = %q cursor=%d", m.lines, m.cursor)
	}

	m.lines = []string{"x"}
	m.cursor = 0
	m.cursorCol = 1
	m.insertIndent()
	m.insertText("body")
	if m.lines[0] != "x   body" {
		t.Fatalf("indent = %q", m.lines[0])
	}
}

func TestEditingEscRestoresHighlightAndFolds(t *testing.T) {
	m := New()
	m.Update(ui.PaneSizeMsg{Width: 60, Height: 20})
	path := writeTemp(t, "c.go", "func a() {\n\tx()\n}")
	m.Update(m.Open(path)())
	m.StartEditing()
	m.cursor = 1
	m.cursorCol = 1
	m.insertText("y")
	cmd := m.exitEditing()
	if cmd == nil {
		t.Fatal("expected re-highlight command")
	}
	msg := cmd().(HighlightedMsg)
	m.Update(msg)
	if len(m.styled) != len(m.lines) {
		t.Fatalf("styled = %d lines = %d", len(m.styled), len(m.lines))
	}
	if len(m.regions) != 1 {
		t.Fatalf("regions = %+v", m.regions)
	}
	plain := githubansi.Strip(m.View(60, 20, true))
	if !strings.Contains(plain, "func a()") {
		t.Fatalf("view missing content:\n%s", plain)
	}
}

func TestEditCursorRendersAtColumn(t *testing.T) {
	m := New()
	m.Update(ui.PaneSizeMsg{Width: 60, Height: 20})
	path := writeTemp(t, "x.txt", "abcdef")
	m.Update(m.Open(path)())
	m.StartEditing()
	m.cursorCol = 2
	line := m.editCursorLine(0)
	if githubansi.Strip(line) != "abcdef" {
		t.Fatalf("cursor altered text: %q", githubansi.Strip(line))
	}
	m.cursorCol = 6
	if githubansi.Strip(m.editCursorLine(0)) != "abcdef " {
		t.Fatalf("cursor at EOL: %q", githubansi.Strip(m.editCursorLine(0)))
	}
}

func TestOpenResetsEditingState(t *testing.T) {
	m := New()
	m.Update(ui.PaneSizeMsg{Width: 60, Height: 20})
	path := writeTemp(t, "a.txt", "hello")
	m.Update(m.Open(path)())
	m.StartEditing()
	m.insertText("x")
	if !m.Dirty() {
		t.Fatal("expected dirty")
	}
	m.Update(m.Open(path)())
	if m.Dirty() || m.Editing() {
		t.Fatal("open must reset dirty/editing")
	}
}

func TestSavePreservesCRLF(t *testing.T) {
	m := New()
	m.Update(ui.PaneSizeMsg{Width: 60, Height: 20})
	path := writeTemp(t, "w.txt", "a\r\nb\r\n")
	m.Update(m.Open(path)())
	m.StartEditing()
	m.cursor = 0
	m.cursorCol = 1
	m.insertText("!")
	msg := m.Save()()
	m.Update(msg)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "a!\r\nb\r\n" {
		t.Fatalf("saved = %q", string(data))
	}
}

func TestSearchRejectedWhileEditing(t *testing.T) {
	m := New()
	path := writeTemp(t, "s.txt", "x")
	m.Update(m.Open(path)())
	m.StartEditing()
	if m.StartSearch() {
		t.Fatal("search must be rejected while editing")
	}
}
