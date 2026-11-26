package winutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// LookCLI finds a CLI on PATH, preferring Windows launchers (.exe, then .cmd,
// then .bat) over extensionless npm shims that CreateProcess cannot run.
func LookCLI(name string) (string, error) {
	if name == "" {
		return "", exec.ErrNotFound
	}
	if filepath.Ext(name) != "" {
		return exec.LookPath(name)
	}
	exts := []string{".exe", ".cmd", ".bat", ".com"}
	if filepath.IsAbs(name) {
		for _, ext := range exts {
			p := name + ext
			if fileExists(p) {
				return p, nil
			}
		}
		if fileExists(name) {
			return PreferWindowsLauncher(name), nil
		}
		return "", exec.ErrNotFound
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		for _, ext := range exts {
			p := filepath.Join(dir, name+ext)
			if fileExists(p) {
				return p, nil
			}
		}
		if p := filepath.Join(dir, name); fileExists(p) {
			return PreferWindowsLauncher(p), nil
		}
	}
	return "", exec.ErrNotFound
}

// PreferWindowsLauncher swaps an extensionless or .ps1 npm shim for a sibling
// .cmd when one exists.
func PreferWindowsLauncher(path string) string {
	if path == "" {
		return path
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".exe", ".com", ".cmd", ".bat":
		return path
	}
	base := path
	if ext != "" {
		base = strings.TrimSuffix(path, filepath.Ext(path))
	}
	if fileExists(base + ".cmd") {
		return base + ".cmd"
	}
	if fileExists(base + ".bat") {
		return base + ".bat"
	}
	if fileExists(base + ".exe") {
		return base + ".exe"
	}
	return path
}

// WrapPTY returns an argv that ConPTY/CreateProcess can actually start.
// .cmd/.bat and extensionless shims are launched through %COMSPEC%.
func WrapPTY(path string, args []string) (string, []string) {
	path = PreferWindowsLauncher(path)
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".cmd", ".bat":
		return comspec(), []string{"/d", "/s", "/c", cmdLine(path, args)}
	case ".ps1":
		ps := "powershell.exe"
		if p, err := exec.LookPath("pwsh.exe"); err == nil {
			ps = p
		} else if p, err := exec.LookPath("powershell.exe"); err == nil {
			ps = p
		}
		return ps, append([]string{"-NoLogo", "-File", path}, args...)
	case "":
		return comspec(), []string{"/d", "/s", "/c", cmdLine(path, args)}
	default:
		if args == nil {
			return path, []string{}
		}
		return path, args
	}
}

func comspec() string {
	if v := os.Getenv("COMSPEC"); v != "" {
		return v
	}
	if p, err := exec.LookPath("cmd.exe"); err == nil {
		return p
	}
	return "cmd.exe"
}

func cmdLine(path string, args []string) string {
	parts := make([]string, 0, 1+len(args))
	parts = append(parts, quoteArg(path))
	for _, a := range args {
		parts = append(parts, quoteArg(a))
	}
	return strings.Join(parts, " ")
}

func quoteArg(s string) string {
	if s == "" {
		return `""`
	}
	if !strings.ContainsAny(s, " \t\"") {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// HideWindow prevents a subprocess from flashing a console window.
func HideWindow(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
}
