package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestPanelExactSize(t *testing.T) {
	hints := []KeyHint{{Key: "↑↓", Desc: "move"}, {Key: "enter", Desc: "open"}}
	for _, sz := range [][2]int{{30, 8}, {60, 20}, {12, 5}, {8, 5}} {
		for _, focused := range []bool{true, false} {
			out := Panel(Spec("Tree", hints), sz[0], sz[1], focused, "a\nb")
			lines := strings.Split(out, "\n")
			if len(lines) != sz[1] {
				t.Fatalf("%v: got %d lines, want %d", sz, len(lines), sz[1])
			}
			for i, l := range lines {
				if w := lipgloss.Width(l); w != sz[0] {
					t.Fatalf("%v line %d: width %d, want %d", sz, i, w, sz[0])
				}
			}
		}
	}
}

func TestBodySize(t *testing.T) {
	if w, h := BodySize(30, 10); w != 28 || h != 6 {
		t.Fatalf("BodySize(30,10) = %d,%d", w, h)
	}
	if w, h := BodySize(1, 1); w != 0 || h != 0 {
		t.Fatalf("BodySize(1,1) = %d,%d", w, h)
	}
}
