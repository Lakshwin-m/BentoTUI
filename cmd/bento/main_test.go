package main

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lakshwin/bento/internal/home"
)

func TestParseArgs(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		wantDir   string
		wantVer   bool
		wantHelp  bool
		wantError bool
	}{
		{name: "empty", args: nil},
		{name: "dir", args: []string{"proj"}, wantDir: "proj"},
		{name: "dir with spaces", args: []string{"my project"}, wantDir: "my project"},
		{name: "version", args: []string{"--version"}, wantVer: true},
		{name: "version short", args: []string{"-v"}, wantVer: true},
		{name: "help", args: []string{"--help"}, wantHelp: true},
		{name: "help short", args: []string{"-h"}, wantHelp: true},
		{name: "unknown flag", args: []string{"--nope"}, wantError: true},
		{name: "two dirs", args: []string{"a", "b"}, wantDir: "a", wantError: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseArgs(tc.args)
			if got.dir != tc.wantDir {
				t.Errorf("dir = %q, want %q", got.dir, tc.wantDir)
			}
			if got.showVersion != tc.wantVer {
				t.Errorf("showVersion = %v, want %v", got.showVersion, tc.wantVer)
			}
			if got.showHelp != tc.wantHelp {
				t.Errorf("showHelp = %v, want %v", got.showHelp, tc.wantHelp)
			}
			if (got.err != nil) != tc.wantError {
				t.Errorf("err = %v, wantError %v", got.err, tc.wantError)
			}
		})
	}
}

func TestSessionForwardsWindowSizeToProject(t *testing.T) {
	s := &sessionModel{model: home.New("test", nil), version: "test"}
	s.Update(tea.WindowSizeMsg{Width: 140, Height: 45})
	if _, cmd := s.Update(home.OpenMsg{Dir: t.TempDir()}); cmd == nil {
		t.Fatal("opening a project should return commands")
	}
	if got := s.View(); got == "" {
		t.Fatal("project view is blank after opening; window size was not forwarded")
	}
}

func TestResolveDir(t *testing.T) {
	base := t.TempDir()
	sub := filepath.Join(base, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(base, "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveDir(sub)
	if err != nil {
		t.Fatalf("resolveDir(sub): %v", err)
	}
	if want, _ := filepath.Abs(sub); got != want {
		t.Errorf("resolveDir = %q, want %q", got, want)
	}

	if _, err := resolveDir(file); err == nil {
		t.Error("resolveDir(file) should fail")
	}
	if _, err := resolveDir(filepath.Join(base, "missing")); err == nil {
		t.Error("resolveDir(missing) should fail")
	}
}
