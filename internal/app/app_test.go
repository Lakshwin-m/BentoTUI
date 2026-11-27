package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/lakshwin/bento/internal/agents"
	"github.com/lakshwin/bento/internal/finder"
	"github.com/lakshwin/bento/internal/layout"
	"github.com/lakshwin/bento/internal/tree"
	"github.com/lakshwin/bento/internal/ui"
	"github.com/lakshwin/bento/internal/viewer"
)

func resize(m *Model, w, h int) {
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
}

func TestViewExactSize(t *testing.T) {
	for _, sz := range [][2]int{{60, 15}, {100, 30}, {140, 45}} {
		m := New(`C:\temp\demo`, "test")
		resize(m, sz[0], sz[1])
		out := m.View()
		lines := strings.Split(ansi.Strip(out), "\n")
		if len(lines) != sz[1] {
			t.Fatalf("%dx%d: got %d lines, want %d", sz[0], sz[1], len(lines), sz[1])
		}
		for i, line := range lines {
			if got := lipgloss.Width(line); got != sz[0] {
				t.Errorf("%dx%d line %d: width %d, want %d", sz[0], sz[1], i, got, sz[0])
			}
		}
	}
}

func TestTooSmall(t *testing.T) {
	m := New(`C:\temp\demo`, "test")
	resize(m, 40, 10)
	if !strings.Contains(ansi.Strip(m.View()), "Terminal too small") {
		t.Fatal("expected too-small message")
	}
}

func TestHelpOverlayToggles(t *testing.T) {
	m := New(`C:\temp\demo`, "test")
	resize(m, 100, 30)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !strings.Contains(ansi.Strip(m.View()), "keyboard reference") {
		t.Fatal("help did not open")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if strings.Contains(ansi.Strip(m.View()), "keyboard reference") {
		t.Fatal("help did not close")
	}
}

func TestCommandPaletteOpensAndRuns(t *testing.T) {
	m := New(`C:\temp\demo`, "test")
	resize(m, 140, 45)
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	plain := ansi.Strip(m.View())
	if !strings.Contains(plain, "Commands") || !strings.Contains(plain, "COMMAND") {
		t.Fatalf("palette did not open:\n%s", plain)
	}
	for _, r := range "tree" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter should return the palette command")
	}
	_, _ = m.Update(cmd())
	if m.paletteOpen {
		t.Fatal("palette should close after running a command")
	}
	if m.Focused() != ui.PaneTree {
		t.Fatalf("focus = %v, want Tree", m.Focused())
	}
}

func TestZoomShowsBadge(t *testing.T) {
	m := New(`C:\temp\demo`, "test")
	resize(m, 140, 45)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}, Alt: true})
	if !m.zoomed {
		t.Fatal("alt+z should enable zoom")
	}
	if !strings.Contains(ansi.Strip(m.View()), "ZOOM") {
		t.Fatal("status bar should show the zoom badge")
	}
}

func TestInfoToastAutoDismisses(t *testing.T) {
	m := New(`C:\temp\demo`, "test")
	resize(m, 100, 30)
	m.setMessage("Saved demo.go", false)
	m.messageAt = time.Now().Add(-5 * time.Second)
	m.Update(toastTickMsg{})
	if m.message != "" {
		t.Fatalf("info toast should clear, got %q", m.message)
	}
	m.setMessage("Save failed", true)
	m.messageAt = time.Now().Add(-5 * time.Second)
	m.Update(toastTickMsg{})
	if m.message == "" {
		t.Fatal("error messages should persist")
	}
}

func TestQuitKeys(t *testing.T) {
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyCtrlQ},
		{Type: tea.KeyCtrlC},
	} {
		m := New(`C:\temp\demo`, "test")
		resize(m, 100, 30)
		_, cmd := m.Update(key)
		if cmd == nil {
			t.Errorf("%s: expected quit command", key.String())
		}
	}
}

func TestMinimumSizeIsNotTooSmall(t *testing.T) {
	m := New(`C:\temp\demo`, "test")
	resize(m, 60, 15)
	plain := ansi.Strip(m.View())
	if strings.Contains(plain, "Terminal too small") {
		t.Fatal("60x15 must be usable")
	}
	lines := strings.Split(plain, "\n")
	if len(lines) != 15 {
		t.Fatalf("lines = %d, want 15", len(lines))
	}
}

func TestFocusCycling(t *testing.T) {
	m := New(`C:\temp\demo`, "test")
	resize(m, 140, 45)
	if m.Focused() != ui.PaneViewer {
		t.Fatalf("initial focus = %v", m.Focused())
	}
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.Focused() != ui.PaneTerminal {
		t.Fatalf("tab focus = %v", m.Focused())
	}
	m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.Focused() != ui.PaneViewer {
		t.Fatalf("shift+tab focus = %v", m.Focused())
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}, Alt: true})
	if m.Focused() != ui.PaneTree {
		t.Fatalf("alt+1 focus = %v", m.Focused())
	}
}

func TestTogglesAndHiddenFocus(t *testing.T) {
	m := New(`C:\temp\demo`, "test")
	resize(m, 140, 45)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}, Alt: true})
	if m.showTree {
		t.Fatal("tree should be hidden")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}, Alt: true})
	if m.Focused() == ui.PaneTree {
		t.Fatal("hidden tree must not receive focus")
	}
	if m.message == "" {
		t.Fatal("hidden pane jump should explain itself")
	}
}

func TestMouseFocus(t *testing.T) {
	m := New(`C:\temp\demo`, "test")
	resize(m, 140, 45)
	grid := layout.Solve(140, 44, layout.State{ShowTree: true, ShowAgent: true, Active: ui.PaneViewer})
	tree := grid.Rects[ui.PaneTree]
	m.Update(tea.MouseMsg{
		X:      tree.X + 1,
		Y:      tree.Y + 1,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	if m.Focused() != ui.PaneTree {
		t.Fatalf("click focus = %v", m.Focused())
	}
}

func TestOpenFileMessageFocusesViewer(t *testing.T) {
	m := New(`C:\temp\demo`, "test")
	resize(m, 140, 45)
	_, cmd := m.Update(tree.OpenFileMsg{Path: `C:\temp\demo\main.go`})
	if m.Focused() != ui.PaneViewer {
		t.Fatalf("focus = %v", m.Focused())
	}
	if m.mode != "PREVIEW" {
		t.Fatalf("mode = %q", m.mode)
	}
	if cmd == nil {
		t.Fatal("opening a file should start a load command")
	}
}

func TestFinderOverlayOpenClose(t *testing.T) {
	m := New(`C:\temp\demo`, "test")
	resize(m, 100, 30)
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	if !m.finderOpen {
		t.Fatal("ctrl+p should open the finder")
	}
	m.Update(finder.IndexedMsg{Files: []string{"a.go", "b.go"}})
	plain := ansi.Strip(m.View())
	if !strings.Contains(plain, "Find file") {
		t.Fatal("finder overlay not rendered")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.finderOpen {
		t.Fatal("esc should close the finder")
	}
}

func TestCtrlFStartsViewerSearch(t *testing.T) {
	m := New(`C:\temp\demo`, "test")
	resize(m, 100, 30)
	m.Update(tree.OpenFileMsg{Path: `C:\temp\demo\main.go`})
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	if !m.viewer.Searching() {
		t.Fatal("ctrl+f should start search")
	}
	if m.modeLabel() != "SEARCH" {
		t.Fatalf("mode = %q", m.modeLabel())
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.viewer.Searching() {
		t.Fatal("esc should leave search")
	}
}

func TestEditorSaveThenQuitConfirm(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(dir, "test")
	resize(m, 100, 30)
	m.Update(tree.OpenFileMsg{Path: path})
	m.Update(viewer.LoadCmd(path, false)())
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if !m.viewer.Editing() {
		t.Fatal("e should start editing")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd == nil {
		t.Fatal("ctrl+s should save")
	}
	m.Update(cmd())
	if m.viewer.Dirty() {
		t.Fatal("dirty after save")
	}
	if !strings.Contains(m.message, "Saved") {
		t.Fatalf("message = %q", m.message)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "xhello" {
		t.Fatalf("saved = %q", string(data))
	}

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlQ})
	if cmd != nil || m.quitting {
		t.Fatal("dirty quit must ask first")
	}
	if m.modeLabel() != "UNSAVED" {
		t.Fatalf("mode = %q", m.modeLabel())
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if m.quitting || m.confirm != "" {
		t.Fatal("n should stay")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlQ})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if !m.quitting {
		t.Fatal("y should discard and quit")
	}
}

func TestUnsavedOpenConfirm(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "a.txt")
	second := filepath.Join(dir, "b.txt")
	os.WriteFile(first, []byte("a"), 0o644)
	os.WriteFile(second, []byte("b"), 0o644)
	m := New(dir, "test")
	resize(m, 100, 30)
	m.Update(tree.OpenFileMsg{Path: first})
	m.Update(viewer.LoadCmd(first, false)())
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("z")})
	m.Update(tree.OpenFileMsg{Path: second})
	if m.confirm != "open" || m.pendingPath != second {
		t.Fatalf("confirm = %q pending = %q", m.confirm, m.pendingPath)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if m.confirm != "" {
		t.Fatal("n should cancel")
	}
	m.Update(tree.OpenFileMsg{Path: second})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if m.confirm != "" {
		t.Fatalf("confirm = %q", m.confirm)
	}
	if m.focused != ui.PaneViewer {
		t.Fatalf("focus = %v", m.focused)
	}
}

func TestCtrlAFocusesAgent(t *testing.T) {
	m := New(`C:\temp\demo`, "test")
	resize(m, 140, 45)
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	if m.Focused() != ui.PaneAgent {
		t.Fatalf("focus = %v", m.Focused())
	}
}

func TestMouseClickOnAgentLaunches(t *testing.T) {
	m := New(`C:\temp\demo`, "test")
	resize(m, 140, 45)
	grid := layout.Solve(140, 44, layout.State{ShowTree: true, ShowAgent: true, Active: ui.PaneViewer})
	agent := grid.Rects[ui.PaneAgent]
	_, cmd := m.Update(tea.MouseMsg{
		X:      agent.X + 2,
		Y:      agent.Y + 1,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	if m.Focused() != ui.PaneAgent {
		t.Fatalf("click focus = %v", m.Focused())
	}
	if cmd == nil {
		t.Fatal("clicking an installed agent should produce a launch command")
	}
}

func TestAgentLaunchFocusesAgentPane(t *testing.T) {
	m := New(`C:\temp\demo`, "test")
	resize(m, 100, 30)
	m.Update(agents.LaunchMsg{Name: "Test Agent", Path: `C:\tools\test-agent.exe`})
	if m.Focused() != ui.PaneAgent {
		t.Fatalf("focus = %v", m.Focused())
	}
	if !strings.Contains(m.message, "Launching") {
		t.Fatalf("message = %q", m.message)
	}
}

func TestSnapshotRestore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\nfour\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(dir, "test")
	resize(m, 140, 40)
	m.viewer.Open(path)
	m.Update(viewer.FileLoadedMsg{Path: path, Lines: []string{"one", "two", "three", "four"}})
	m.viewer.RestoreCursor(2, 1)
	m.showAgent = false
	m.focused = ui.PaneChanges

	snap := m.Snapshot()
	if snap.File != path || snap.Focus != "Changes" || snap.ShowAgent || !snap.ShowTree || snap.Line != 2 || snap.Col != 1 {
		t.Fatalf("snapshot = %+v", snap)
	}

	m2 := New(dir, "test")
	m2.Restore(snap)
	resize(m2, 140, 40)
	if m2.viewer.Path() != path {
		t.Fatalf("path = %q", m2.viewer.Path())
	}
	if m2.showAgent || !m2.showTree || m2.Focused() != ui.PaneChanges {
		t.Fatalf("restored layout focus=%v tree=%v agent=%v", m2.Focused(), m2.showTree, m2.showAgent)
	}
	cmd := m2.startup
	m2.startup = nil
	if cmd == nil {
		t.Fatal("expected deferred file load")
	}
	m2.Update(cmd())
	if line, col := m2.viewer.Cursor(); line != 2 || col != 1 {
		t.Fatalf("cursor = %d,%d", line, col)
	}
}

func TestRestoreRejectsOutsideFile(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(dir, "test")
	m.Restore(Snapshot{Dir: dir, File: outside, ShowTree: true, ShowAgent: true, Focus: "Viewer"})
	if m.viewer.Path() != "" {
		t.Fatalf("opened outside file: %q", m.viewer.Path())
	}
}

func TestProjectName(t *testing.T) {
	cases := map[string]string{
		`C:\Users\me\proj`:  "proj",
		`C:\Users\me\proj\`: "proj",
		`proj`:              "proj",
	}
	for in, want := range cases {
		if got := projectName(in); got != want {
			t.Errorf("projectName(%q) = %q, want %q", in, got, want)
		}
	}
}
