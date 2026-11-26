package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestIslandExactDimensions(t *testing.T) {
	sizes := [][2]int{{20, 5}, {60, 15}, {100, 30}, {4, 3}}
	for _, sz := range sizes {
		w, h := sz[0], sz[1]
		out := Island("Tree", w, h, true, "one\ntwo")
		lines := strings.Split(out, "\n")
		if len(lines) != h {
			t.Errorf("%dx%d: got %d lines, want %d", w, h, len(lines), h)
		}
		for i, line := range lines {
			if got := lipgloss.Width(line); got != w {
				t.Errorf("%dx%d line %d: width %d, want %d (%q)", w, h, i, got, w, ansi.Strip(line))
			}
		}
	}
}

func TestIslandTitle(t *testing.T) {
	out := ansi.Strip(Island("Preview", 40, 8, false, ""))
	if !strings.Contains(out, "Preview") {
		t.Fatalf("title missing: %q", out)
	}
}

func TestFit(t *testing.T) {
	if got := Fit("abc", 5); got != "abc  " {
		t.Errorf("Fit pad = %q", got)
	}
	if got := ansi.Strip(Fit("abcdef", 4)); lipgloss.Width(got) != 4 {
		t.Errorf("Fit truncate width = %d", lipgloss.Width(got))
	}
}
