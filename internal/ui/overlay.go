package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Overlay replaces a rectangle of base with box, preserving the cells outside
// the box. base must be a rendered canvas of width x height; box lines are
// clipped or padded to the box width. Coordinates outside the canvas clip.
func Overlay(base string, width, height int, box string, x, y int) string {
	if width <= 0 || height <= 0 {
		return base
	}
	boxLines := strings.Split(box, "\n")
	boxW := 0
	for _, l := range boxLines {
		if w := lipgloss.Width(l); w > boxW {
			boxW = w
		}
	}
	if boxW == 0 || len(boxLines) == 0 {
		return base
	}
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x+boxW > width {
		x = width - boxW
		if x < 0 {
			x = 0
		}
	}
	if y+len(boxLines) > height {
		y = height - len(boxLines)
		if y < 0 {
			y = 0
		}
	}

	rows := strings.Split(base, "\n")
	for len(rows) < height {
		rows = append(rows, Fit("", width))
	}
	rows = rows[:height]

	for i, bl := range boxLines {
		row := y + i
		if row < 0 || row >= height {
			continue
		}
		bl = Fit(bl, boxW)
		line := rows[row]
		left := ansi.Cut(line, 0, x)
		right := ansi.Cut(line, x+boxW, width)
		rows[row] = Fit(left+bl+right, width)
	}
	return strings.Join(rows[:height], "\n")
}
