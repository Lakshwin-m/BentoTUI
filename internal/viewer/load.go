package viewer

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// MaxFileSize is the size above which the viewer asks before loading.
const MaxFileSize int64 = 5 << 20

// FileLoadedMsg carries the result of an asynchronous file read.
type FileLoadedMsg struct {
	Path     string
	Lines    []string
	Styled   []string
	Binary   bool
	Size     int64
	TooLarge bool
	Regions  []FoldRegion
	EOL      string
	BOM      bool
	Err      error
}

// LoadCmd reads and highlights a file off the UI goroutine.
func LoadCmd(path string, force bool) tea.Cmd {
	return func() tea.Msg {
		return loadFile(path, force, MaxFileSize)
	}
}

func loadFile(path string, force bool, limit int64) FileLoadedMsg {
	info, err := os.Stat(path)
	if err != nil {
		return FileLoadedMsg{Path: path, Err: err}
	}
	if info.IsDir() {
		return FileLoadedMsg{Path: path, Err: errors.New("is a directory")}
	}
	size := info.Size()
	if size > limit && !force {
		return FileLoadedMsg{Path: path, Size: size, TooLarge: true}
	}

	f, err := os.Open(path)
	if err != nil {
		return FileLoadedMsg{Path: path, Err: err}
	}
	defer f.Close()

	head := make([]byte, 8192)
	n, err := io.ReadFull(f, head)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return FileLoadedMsg{Path: path, Err: err}
	}
	head = head[:n]
	if bytes.IndexByte(head, 0) >= 0 {
		return FileLoadedMsg{Path: path, Size: size, Binary: true}
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return FileLoadedMsg{Path: path, Err: err}
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return FileLoadedMsg{Path: path, Err: err}
	}

	raw := string(data)
	bom := strings.HasPrefix(raw, "\uFEFF")
	raw = strings.TrimPrefix(raw, "\uFEFF")
	// Normalize for display; saving restores the original EOL convention.
	eol := "\n"
	if strings.Contains(raw, "\r\n") {
		eol = "\r\n"
	}
	text := strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	highlighted := HighlightLines(text, path)
	if len(highlighted) < len(lines) {
		for len(highlighted) < len(lines) {
			highlighted = append(highlighted, "")
		}
	}
	highlighted = highlighted[:len(lines)]

	return FileLoadedMsg{
		Path:    path,
		Lines:   lines,
		Styled:  highlighted,
		Size:    size,
		Regions: computeFoldRegions(lines, path),
		EOL:     eol,
		BOM:     bom,
	}
}
