package winutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLookCLIPrefersCmdOverShim(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "opencode"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "opencode.cmd"), []byte("@echo off\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	got, err := LookCLI("opencode")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(filepath.Base(got), "opencode.cmd") {
		t.Fatalf("got %q, want opencode.cmd", got)
	}
}

func TestPreferWindowsLauncherFromPS1(t *testing.T) {
	dir := t.TempDir()
	ps1 := filepath.Join(dir, "opencode.ps1")
	cmd := filepath.Join(dir, "opencode.cmd")
	os.WriteFile(ps1, []byte("#!/usr/bin/env pwsh\n"), 0o644)
	os.WriteFile(cmd, []byte("@echo off\n"), 0o644)
	if got := PreferWindowsLauncher(ps1); got != cmd {
		t.Fatalf("got %q want %q", got, cmd)
	}
}

func TestWrapPTYCmdGoesThroughComspec(t *testing.T) {
	t.Setenv("COMSPEC", `C:\Windows\System32\cmd.exe`)
	exe, args := WrapPTY(`C:\Users\me\AppData\Roaming\npm\opencode.cmd`, nil)
	if !strings.Contains(strings.ToLower(exe), "cmd.exe") {
		t.Fatalf("exe = %q", exe)
	}
	if len(args) < 3 || args[0] != "/d" || args[1] != "/s" || args[2] != "/c" {
		t.Fatalf("args = %#v", args)
	}
	if !strings.Contains(args[len(args)-1], "opencode.cmd") {
		t.Fatalf("command line = %q", args[len(args)-1])
	}
}

func TestWrapPTYExePassthrough(t *testing.T) {
	exe, args := WrapPTY(`C:\tools\claude.exe`, []string{"--help"})
	if exe != `C:\tools\claude.exe` {
		t.Fatalf("exe = %q", exe)
	}
	if len(args) != 1 || args[0] != "--help" {
		t.Fatalf("args = %#v", args)
	}
}
