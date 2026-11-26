package console

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"

	"github.com/lakshwin/bento/internal/ui"
)

func TestEncodeKey(t *testing.T) {
	cases := []struct {
		msg  tea.KeyMsg
		want string
	}{
		{tea.KeyMsg{Type: tea.KeyCtrlC}, "\x03"},
		{tea.KeyMsg{Type: tea.KeyEnter}, "\r"},
		{tea.KeyMsg{Type: tea.KeyUp}, "\x1b[A"},
		{tea.KeyMsg{Type: tea.KeyBackspace}, "\x7f"},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}, "a"},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x"), Alt: true}, "\x1bx"},
		{tea.KeyMsg{Type: tea.KeySpace}, " "},
	}
	for _, c := range cases {
		if got := string(encodeKey(c.msg)); got != c.want {
			t.Errorf("%s: got %q want %q", c.msg.String(), got, c.want)
		}
	}
}

func TestConsoleTabsAndShellRender(t *testing.T) {
	m := New(t.TempDir())
	m.Update(ui.PaneSizeMsg{Width: 60, Height: 20})
	s := &shell{id: 1, title: "shell 1", emu: vt.NewSafeEmulator(58, 17), ch: make(chan []byte, 1)}
	s.emu.Write([]byte("hello\r\n"))
	m.shells = append(m.shells, s)
	m.active = 1
	plain := ansi.Strip(m.View(60, 20, true))
	if !strings.Contains(plain, "shell 1") || !strings.Contains(plain, "hello") {
		t.Fatalf("view:\n%s", plain)
	}
	if m.Title() != "shell 1" {
		t.Fatalf("title = %q", m.Title())
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}, Alt: true})
	if m.active != 1 {
		t.Fatalf("alt+x changed tabs: %d", m.active)
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}, Alt: true})
	if m.active != 0 {
		t.Fatalf("alt+] active = %d", m.active)
	}
	if m.Title() != "Build" {
		t.Fatalf("title = %q", m.Title())
	}
}

func TestCloseActiveShell(t *testing.T) {
	m := New(t.TempDir())
	s := &shell{id: 1, title: "shell 1", emu: vt.NewSafeEmulator(10, 5), ch: make(chan []byte, 1)}
	m.shells = append(m.shells, s)
	m.active = 1
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}, Alt: true})
	if len(m.shells) != 0 || m.active != 0 {
		t.Fatalf("shells=%d active=%d", len(m.shells), m.active)
	}
}

func TestShellRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("pty integration")
	}
	s, err := startShell(99, t.TempDir(), 80, 24)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer closeShell(s)
	if _, err := s.pty.Write([]byte("echo BENTO_OK\r")); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(15 * time.Second)
	var seen strings.Builder
	for {
		select {
		case data, ok := <-s.ch:
			if !ok {
				t.Fatalf("stream closed early: %q", seen.String())
			}
			seen.Write(data)
			if strings.Contains(seen.String(), "BENTO_OK") {
				return
			}
		case <-deadline:
			t.Fatalf("timeout; output so far: %q", seen.String())
		}
	}
}
