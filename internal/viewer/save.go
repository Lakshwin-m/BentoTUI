package viewer

import (
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// SavedMsg reports the result of an asynchronous write.
type SavedMsg struct {
	Path string
	Err  error
}

// SaveCmd snapshots the buffer and writes it off the UI goroutine, preserving
// the original EOL style, BOM and file permissions.
func SaveCmd(path string, lines []string, eol string, bom bool) tea.Cmd {
	snapshot := append([]string(nil), lines...)
	return func() tea.Msg {
		if eol == "" {
			eol = "\n"
		}
		text := strings.Join(snapshot, eol)
		if bom {
			text = "\uFEFF" + text
		}
		mode := os.FileMode(0o644)
		if info, err := os.Stat(path); err == nil {
			mode = info.Mode().Perm()
		}
		err := os.WriteFile(path, []byte(text), mode)
		return SavedMsg{Path: path, Err: err}
	}
}
