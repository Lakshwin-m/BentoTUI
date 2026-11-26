package ignore

import "testing"

func TestPatterns(t *testing.T) {
	ig := Parse([]byte(`
# comment
node_modules/
*.log
!keep.log
/build
dist/**
**/generated
*.tmp
`))
	cases := []struct {
		rel   string
		isDir bool
		want  bool
	}{
		{"node_modules", true, true},
		{"src/node_modules", true, true},
		{"node_modules", false, false},
		{"a.log", false, true},
		{"src/deep/a.log", false, true},
		{"keep.log", false, false},
		{"build", true, true},
		{"src/build", true, false},
		{"dist/out.js", false, true},
		{"dist", false, false},
		{"src/generated", true, true},
		{"generated", true, true},
		{"notes.tmp", false, true},
		{"notes.txt", false, false},
	}
	for _, c := range cases {
		if got := ig.Match(c.rel, c.isDir); got != c.want {
			t.Errorf("Match(%q, dir=%v) = %v, want %v", c.rel, c.isDir, got, c.want)
		}
	}
}

func TestNegationOrder(t *testing.T) {
	ig := Parse([]byte("*.log\n!important.log\n"))
	if !ig.Match("x.log", false) {
		t.Error("x.log should be ignored")
	}
	if ig.Match("important.log", false) {
		t.Error("important.log should be re-included")
	}
}

func TestBackslashPaths(t *testing.T) {
	ig := Parse([]byte("secret/\n"))
	if !ig.Match(`src\secret`, true) {
		t.Error("backslash paths should normalize")
	}
}

func TestDoubleStar(t *testing.T) {
	re, err := globToRegex("**/cache/**")
	if err != nil {
		t.Fatal(err)
	}
	// gitignore's trailing /** matches everything inside, not the directory.
	for _, s := range []string{"cache/x", "a/cache/x", "a/b/cache/x/y"} {
		if !re.MatchString(s) {
			t.Errorf("%q should match", s)
		}
	}
	for _, s := range []string{"cache", "a/cache", "a/cached"} {
		if re.MatchString(s) {
			t.Errorf("%q should not match", s)
		}
	}
}

func TestEmptyMatcher(t *testing.T) {
	var m *Matcher
	if m.Match("x", false) {
		t.Error("nil matcher must never match")
	}
}
