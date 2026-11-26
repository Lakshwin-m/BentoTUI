package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestOverlayPlacement(t *testing.T) {
	base := strings.Join([]string{
		"abcdefghij",
		"abcdefghij",
		"abcdefghij",
	}, "\n")
	out := ansi.Strip(Overlay(base, 10, 3, "XYZ", 2, 0))
	lines := strings.Split(out, "\n")
	if lines[0] != "abXYZfghij" {
		t.Errorf("row 0 = %q", lines[0])
	}
	if lines[1] != "abcdefghij" {
		t.Errorf("row 1 = %q", lines[1])
	}
	for i, line := range lines {
		if lipgloss.Width(line) != 10 {
			t.Errorf("row %d width %d", i, lipgloss.Width(line))
		}
	}
}

func TestOverlayMultilineAndClip(t *testing.T) {
	base := strings.Join([]string{
		"0123456789",
		"0123456789",
		"0123456789",
		"0123456789",
	}, "\n")
	box := "AA\nBB"
	out := ansi.Strip(Overlay(base, 10, 4, box, 12, 3))
	lines := strings.Split(out, "\n")
	if len(lines) != 4 {
		t.Fatalf("lines = %d", len(lines))
	}
	for i, line := range lines {
		if lipgloss.Width(line) != 10 {
			t.Errorf("row %d width %d", i, lipgloss.Width(line))
		}
	}
}
