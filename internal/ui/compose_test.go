package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestComposeBasic(t *testing.T) {
	a := Island("A", 10, 4, false, "")
	b := Island("B", 10, 4, false, "")
	out := Compose(21, 4, []Placed{
		{X: 11, Y: 0, View: b},
		{X: 0, Y: 0, View: a},
	})
	lines := strings.Split(out, "\n")
	if len(lines) != 4 {
		t.Fatalf("lines = %d", len(lines))
	}
	for i, line := range lines {
		if got := lipgloss.Width(line); got != 21 {
			t.Errorf("line %d width %d: %q", i, got, ansi.Strip(line))
		}
	}
	plain := ansi.Strip(out)
	if !strings.Contains(plain, "A") || !strings.Contains(plain, "B") {
		t.Fatalf("missing panes: %q", plain)
	}
	for _, line := range strings.Split(plain, "\n") {
		runes := []rune(line)
		if len(runes) > 10 && runes[10] != ' ' {
			t.Errorf("gutter column not blank: %q", line)
		}
	}
}

func TestComposeClipsAndPads(t *testing.T) {
	view := "0123456789"
	out := Compose(5, 3, []Placed{{X: 0, Y: 1, View: view}})
	lines := strings.Split(out, "\n")
	if len(lines) != 3 {
		t.Fatalf("lines = %d", len(lines))
	}
	for i, line := range lines {
		if got := lipgloss.Width(line); got != 5 {
			t.Errorf("line %d width %d", i, got)
		}
	}
	if lines[0] != "     " {
		t.Errorf("row 0 = %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "0123") {
		t.Errorf("row 1 = %q", lines[1])
	}
}

func TestComposeEmpty(t *testing.T) {
	if got := Compose(0, 10, nil); got != "" {
		t.Errorf("zero width = %q", got)
	}
	out := Compose(5, 2, nil)
	lines := strings.Split(out, "\n")
	if len(lines) != 2 || lines[0] != "     " {
		t.Fatalf("blank canvas = %q", out)
	}
}

func TestTabStripExactWidth(t *testing.T) {
	labels := []string{"1 Tree", "2 Preview", "3 Terminal", "4 Changes"}
	for _, w := range []int{0, 1, 10, 40, 80} {
		out := TabStrip(w, labels, 1)
		if w == 0 {
			if out != "" {
				t.Errorf("w=0: %q", out)
			}
			continue
		}
		if got := lipgloss.Width(out); got != w {
			t.Errorf("w=%d: width %d", w, got)
		}
	}
}
