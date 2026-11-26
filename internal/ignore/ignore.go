// Package ignore implements a pragmatic subset of gitignore matching used by
// the tree and the fuzzy finder. Nested .gitignore files are not interpreted
// yet; the project-root file covers common repositories.
package ignore

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// Matcher is a parsed .gitignore.
type Matcher struct {
	rules []rule
}

type rule struct {
	re       *regexp.Regexp
	negate   bool
	dirOnly  bool
	basename bool
}

// Load reads <root>/.gitignore. A missing or unreadable file yields an empty
// matcher rather than an error.
func Load(root string) *Matcher {
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		return &Matcher{}
	}
	return Parse(data)
}

// Parse parses gitignore syntax, tolerating anything malformed.
func Parse(data []byte) *Matcher {
	m := &Matcher{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		negate := false
		switch {
		case strings.HasPrefix(line, `\#`), strings.HasPrefix(line, `\!`):
			line = line[1:]
		case strings.HasPrefix(line, "!"):
			negate = true
			line = line[1:]
		}
		line = strings.TrimRight(line, " ")
		if line == "" {
			continue
		}
		dirOnly := strings.HasSuffix(line, "/")
		line = strings.TrimSuffix(line, "/")
		anchored := strings.HasPrefix(line, "/")
		line = strings.TrimPrefix(line, "/")
		if line == "" {
			continue
		}
		basename := !strings.Contains(line, "/") && !anchored
		re, err := globToRegex(line)
		if err != nil {
			continue
		}
		m.rules = append(m.rules, rule{
			re:       re,
			negate:   negate,
			dirOnly:  dirOnly,
			basename: basename,
		})
	}
	return m
}

// Match reports whether rel (relative to the project root) is ignored.
func (m *Matcher) Match(rel string, isDir bool) bool {
	if m == nil || len(m.rules) == 0 {
		return false
	}
	rel = strings.TrimPrefix(strings.ReplaceAll(rel, `\`, "/"), "./")
	rel = path.Clean(rel)
	if rel == "." || rel == "" {
		return false
	}
	base := path.Base(rel)
	ignored := false
	for _, r := range m.rules {
		if r.dirOnly && !isDir {
			continue
		}
		target := rel
		if r.basename {
			target = base
		}
		if r.re.MatchString(target) {
			ignored = !r.negate
		}
	}
	return ignored
}

// globToRegex converts a gitignore glob to an anchored regular expression.
func globToRegex(pat string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pat); i++ {
		c := pat[i]
		switch c {
		case '*':
			if i+1 < len(pat) && pat[i+1] == '*' {
				if i+2 < len(pat) && pat[i+2] == '/' {
					b.WriteString("(?:.*/)?")
					i += 2
				} else {
					b.WriteString(".*")
					i++
				}
				continue
			}
			b.WriteString("[^/]*")
		case '?':
			b.WriteString("[^/]")
		case '[':
			j := i + 1
			for j < len(pat) && pat[j] != ']' {
				j++
			}
			if j >= len(pat) {
				b.WriteString(`\[`)
				continue
			}
			class := pat[i+1 : j]
			if strings.HasPrefix(class, "!") {
				class = "^" + class[1:]
			}
			b.WriteString("[" + class + "]")
			i = j
		case '\\':
			if i+1 < len(pat) {
				b.WriteString(regexp.QuoteMeta(string(pat[i+1])))
				i++
			}
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}
