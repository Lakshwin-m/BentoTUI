package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	in := State{
		LastDir:   `C:\work\demo`,
		LastFile:  `C:\work\demo\main.go`,
		Focus:     "Viewer",
		ShowTree:  true,
		ShowAgent: false,
		Line:      12,
		Col:       3,
	}
	if err := saveTo(path, in); err != nil {
		t.Fatal(err)
	}
	out, err := loadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Valid {
		t.Fatal("expected valid state")
	}
	if out.Version != schemaVersion {
		t.Fatalf("version = %d", out.Version)
	}
	in.Version = schemaVersion
	if out.LastDir != in.LastDir || out.LastFile != in.LastFile ||
		out.Focus != in.Focus || out.ShowTree != in.ShowTree ||
		out.ShowAgent != in.ShowAgent || out.Line != in.Line || out.Col != in.Col {
		t.Fatalf("round trip = %+v, want %+v", out, in)
	}
}

func TestLoadMissingAndCorrupt(t *testing.T) {
	st, err := loadFrom(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil || st.Valid {
		t.Fatalf("missing: %+v %v", st, err)
	}
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadFrom(path); err == nil {
		t.Fatal("expected parse error")
	}
}
