package viewer

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/lakshwin/bento/internal/theme"
)

// FoldRegion is a foldable span of buffer lines: [Start, End).
type FoldRegion struct {
	Start int
	End   int
}

type displayRow struct {
	line   int
	hidden int // lines hidden by a collapsed fold at this row
}

var braceLangs = map[string]bool{
	".go": true, ".js": true, ".jsx": true, ".ts": true, ".tsx": true,
	".json": true, ".java": true, ".c": true, ".cc": true, ".cpp": true,
	".h": true, ".hpp": true, ".cs": true, ".rs": true, ".css": true,
	".scss": true, ".php": true, ".swift": true, ".kt": true, ".kts": true,
	".m": true, ".mm": true,
}

// computeFoldRegions returns foldable regions for a file. Brace languages use
// matching brackets (ignoring strings and comments); everything else falls back
// to indentation.
func computeFoldRegions(lines []string, filename string) []FoldRegion {
	ext := strings.ToLower(filepath.Ext(filename))
	var regions []FoldRegion
	if braceLangs[ext] {
		regions = braceRegions(lines)
	} else {
		regions = indentRegions(lines)
	}
	sort.SliceStable(regions, func(i, j int) bool {
		if regions[i].Start != regions[j].Start {
			return regions[i].Start < regions[j].Start
		}
		return regions[i].End > regions[j].End
	})
	out := regions[:0]
	lastStart := -1
	for _, r := range regions {
		if r.Start == lastStart {
			continue
		}
		if r.End-r.Start <= 1 {
			continue
		}
		out = append(out, r)
		lastStart = r.Start
	}
	return out
}

func braceRegions(lines []string) []FoldRegion {
	var regions []FoldRegion
	var stack []int
	blockComment := false
	var quote byte

	for li, line := range lines {
		escaped := false
		for k := 0; k < len(line); k++ {
			c := line[k]
			if blockComment {
				if c == '*' && k+1 < len(line) && line[k+1] == '/' {
					blockComment = false
					k++
				}
				continue
			}
			if quote != 0 {
				if escaped {
					escaped = false
					continue
				}
				if c == '\\' && quote != '`' {
					escaped = true
					continue
				}
				if c == quote {
					quote = 0
				}
				continue
			}
			switch {
			case c == '/' && k+1 < len(line) && line[k+1] == '/':
				k = len(line)
			case c == '/' && k+1 < len(line) && line[k+1] == '*':
				blockComment = true
				k++
			case c == '"' || c == '\'' || c == '`':
				quote = c
			case c == '{':
				stack = append(stack, li)
			case c == '}':
				if len(stack) > 0 {
					start := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					if li > start {
						regions = append(regions, FoldRegion{Start: start, End: li + 1})
					}
				}
			}
		}
		if quote != '`' {
			quote = 0
		}
	}
	return regions
}

func indentRegions(lines []string) []FoldRegion {
	indent := func(s string) int {
		n := 0
		for _, r := range s {
			switch r {
			case ' ':
				n++
			case '\t':
				n += 4
			default:
				return n
			}
		}
		return n
	}
	blank := func(s string) bool { return strings.TrimSpace(s) == "" }

	var regions []FoldRegion
	for i := range lines {
		if blank(lines[i]) {
			continue
		}
		base := indent(lines[i])
		j := i + 1
		for j < len(lines) && blank(lines[j]) {
			j++
		}
		if j >= len(lines) {
			continue
		}
		if indent(lines[j]) <= base {
			if j-i > 1 {
				regions = append(regions, FoldRegion{Start: i, End: j})
			}
			continue
		}
		// Find the first subsequent non-blank line at or below base indentation.
		end := len(lines)
		for k := j; k < len(lines); k++ {
			if blank(lines[k]) {
				continue
			}
			if indent(lines[k]) <= base {
				end = k
				break
			}
		}
		if end-i > 1 {
			regions = append(regions, FoldRegion{Start: i, End: end})
		}
	}
	return regions
}

// rebuildDisplay computes visible rows from buffer lines and fold state.
func (m *Model) rebuildDisplay() {
	m.display = m.display[:0]
	if len(m.collapsed) == 0 {
		m.display = make([]displayRow, len(m.lines))
		for i := range m.lines {
			m.display[i] = displayRow{line: i}
		}
		return
	}
	spans := make(map[int]int, len(m.regions))
	for _, r := range m.regions {
		if m.collapsed[r.Start] {
			spans[r.Start] = r.End
		}
	}
	for i := 0; i < len(m.lines); {
		if end, ok := spans[i]; ok && end > i {
			m.display = append(m.display, displayRow{line: i, hidden: end - i - 1})
			i = end
			continue
		}
		m.display = append(m.display, displayRow{line: i})
		i++
	}
}

func (m *Model) displayIndex(line int) int {
	for i, row := range m.display {
		if row.line == line {
			return i
		}
	}
	return -1
}

func (m *Model) moveCursor(delta int) {
	if len(m.display) == 0 {
		return
	}
	idx := m.displayIndex(m.cursor)
	if idx < 0 {
		idx = 0
	}
	idx += delta
	if idx < 0 {
		idx = 0
	}
	if idx >= len(m.display) {
		idx = len(m.display) - 1
	}
	m.cursor = m.display[idx].line
}

func (m *Model) toggleFold() {
	if len(m.regions) == 0 {
		return
	}
	target := -1
	for _, r := range m.regions {
		if r.Start == m.cursor {
			target = r.Start
			break
		}
	}
	if target < 0 {
		for _, r := range m.regions {
			if r.Start < m.cursor && m.cursor < r.End {
				if target < 0 || r.Start > target {
					target = r.Start
				}
			}
		}
	}
	if target < 0 {
		return
	}
	if m.collapsed[target] {
		delete(m.collapsed, target)
	} else {
		m.collapsed[target] = true
	}
	m.afterFoldChange()
}

func (m *Model) toggleAllFolds() {
	if len(m.regions) == 0 {
		return
	}
	if len(m.collapsed) > 0 {
		m.collapsed = map[int]bool{}
	} else {
		for _, r := range m.regions {
			m.collapsed[r.Start] = true
		}
	}
	m.afterFoldChange()
}

func (m *Model) afterFoldChange() {
	m.rebuildDisplay()
	m.clamp()
}

// revealLine unfolds every region containing line.
func (m *Model) revealLine(line int) {
	changed := false
	for _, r := range m.regions {
		if r.Start < line && line < r.End {
			if m.collapsed[r.Start] {
				delete(m.collapsed, r.Start)
				changed = true
			}
		}
	}
	if changed {
		m.rebuildDisplay()
	}
}

// foldMarker reports the gutter marker for a folded or foldable line.
func (m *Model) foldMarker(line int) (string, bool) {
	for _, r := range m.regions {
		if r.Start == line {
			if m.collapsed[line] {
				return theme.TreeCollapsed, true
			}
			return theme.TreeExpanded, true
		}
	}
	return "", false
}
