package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestStatusBarExactWidthAndRightMode(t *testing.T) {
	for _, width := range []int{40, 60, 100, 140} {
		out := StatusBar(width, StatusData{
			Project: "demo",
			Branch:  "main",
			Dirty:   true,
			Hints:   HintLine([]KeyHint{{Key: "tab", Desc: "focus"}}),
			Message: "Saved",
			Mode:    "EDIT",
		})
		if got := lipgloss.Width(out); got != width {
			t.Fatalf("width=%d: got %d", width, got)
		}
		plain := ansi.Strip(out)
		if !strings.HasSuffix(plain, " EDIT ") {
			t.Fatalf("width=%d: mode not right aligned: %q", width, plain)
		}
	}
}

func TestStatusBarTightDropsMessageKeepsMode(t *testing.T) {
	out := ansi.Strip(StatusBar(48, StatusData{
		Project: "a-rather-long-project-name",
		Branch:  "feature/long-branch",
		Message: "Launch failed: something went very wrong indeed",
		Mode:    "SHELL",
	}))
	if !strings.Contains(out, "SHELL") {
		t.Fatalf("mode missing: %q", out)
	}
	if lipgloss.Width(StatusBar(48, StatusData{Project: "demo", Mode: "EDIT"})) != 48 {
		t.Fatal("width changed")
	}
}

func TestCenteredCardDimensions(t *testing.T) {
	for _, sz := range [][2]int{{20, 6}, {40, 12}, {80, 24}} {
		out := CenteredCard(sz[0], sz[1], "Nothing here", "Install a tool or open a file.")
		if got := lipgloss.Width(out); got != sz[0] {
			t.Errorf("%dx%d width = %d", sz[0], sz[1], got)
		}
		if got := lipgloss.Height(out); got != sz[1] {
			t.Errorf("%dx%d height = %d", sz[0], sz[1], got)
		}
	}
}
