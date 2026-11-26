package build

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/lakshwin/bento/internal/ui"
)

func key(s string) tea.KeyMsg {
	switch s {
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectGoProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module x\n")
	tasks := Detect(dir)
	if len(tasks) != 3 {
		t.Fatalf("tasks = %+v", tasks)
	}
	want := []string{"go build ./...", "go test ./...", "go vet ./..."}
	for i, w := range want {
		if tasks[i].CmdLine != w {
			t.Errorf("task %d = %q, want %q", i, tasks[i].CmdLine, w)
		}
	}
}

func TestDetectNPMOrderedScripts(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"scripts":{"test":"vitest","build":"vite build","lint":"eslint ."}}`)
	tasks := Detect(dir)
	got := []string{}
	for _, task := range tasks {
		got = append(got, task.CmdLine)
	}
	want := []string{"npm run build", "npm run test", "npm run lint"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("tasks = %v", got)
	}
}

func TestDetectNothing(t *testing.T) {
	if tasks := Detect(t.TempDir()); len(tasks) != 0 {
		t.Fatalf("tasks = %+v", tasks)
	}
}

func TestViewShowsTasksAndOutput(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module x\n")
	m := New(dir)
	m.Update(ui.PaneSizeMsg{Width: 50, Height: 18})
	plain := ansi.Strip(m.View(50, 18, true))
	if !strings.Contains(plain, "build") || !strings.Contains(plain, "go build ./...") {
		t.Fatalf("view:\n%s", plain)
	}
	m.running = true
	m.Update(RunEventMsg{Line: "hello output"})
	m.Update(RunEventMsg{Done: true})
	plain = ansi.Strip(m.View(50, 18, true))
	if !strings.Contains(plain, "hello output") {
		t.Fatalf("output missing:\n%s", plain)
	}
	if m.running {
		t.Fatal("done event should stop the task")
	}
}

func TestTaskSelectionCycles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module x\n")
	m := New(dir)
	if m.sel != 0 {
		t.Fatal("initial selection")
	}
	m.handleKey(key("right"))
	if m.sel != 1 {
		t.Fatalf("sel = %d", m.sel)
	}
	m.handleKey(key("left"))
	if m.sel != 0 {
		t.Fatalf("sel = %d", m.sel)
	}
}

func TestStreamEcho(t *testing.T) {
	ch := make(chan Event, 8)
	go stream(context.Background(), t.TempDir(), "echo hello", ch)
	var lines []string
	var done bool
	for ev := range ch {
		if ev.Done {
			done = true
			if ev.Err != nil {
				t.Fatalf("err = %v", ev.Err)
			}
			continue
		}
		lines = append(lines, ev.Line)
	}
	if !done {
		t.Fatal("missing done event")
	}
	if len(lines) == 0 || strings.TrimSpace(lines[len(lines)-1]) != "hello" {
		t.Fatalf("lines = %v", lines)
	}
}
