package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/lakshwin/bento/internal/theme"
)

func TestPaintBackgroundAfterResets(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(old)

	seq := backgroundSeq(theme.P.Panel)
	if seq == "" {
		t.Fatal("expected a background sequence in truecolor")
	}
	line := theme.SelectedStyle().Render("x") + "   "
	painted := PaintBackground(line, theme.P.Panel)
	if !strings.Contains(painted, "\x1b[0m"+seq) {
		t.Fatalf("reset not repainted: %q", painted)
	}
	if !strings.HasSuffix(painted, "\x1b[0m") {
		t.Fatalf("missing final reset: %q", painted)
	}
}

func TestPaintBackgroundCoversLineStarts(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(old)

	seq := backgroundSeq(theme.P.Panel)
	painted := PaintBackground("first\nsecond", theme.P.Panel)
	if !strings.HasPrefix(painted, seq) {
		t.Fatalf("first line is not painted: %q", painted)
	}
	if !strings.Contains(painted, "\n"+seq) {
		t.Fatalf("following line is not painted: %q", painted)
	}
	if !strings.HasSuffix(painted, "\x1b[0m") {
		t.Fatalf("missing final reset: %q", painted)
	}
}

func TestIslandRepaintsBodyGaps(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(old)

	seq := backgroundSeq(theme.P.Panel)
	body := theme.SelectedStyle().Render("short")
	out := Island("Pane", 30, 6, true, body)
	if !strings.Contains(out, "\x1b[0m"+seq) {
		t.Fatalf("island body gaps are not filled:\n%q", out)
	}
}
