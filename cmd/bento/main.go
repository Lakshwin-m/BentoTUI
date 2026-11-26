// Command bento opens one project directory in a multi-pane terminal workspace.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"

	"github.com/lakshwin/bento/internal/app"
	"github.com/lakshwin/bento/internal/config"
	"github.com/lakshwin/bento/internal/home"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

const usage = `Bento - one project. Multi-pane. Keyboard-first.

Usage:
  bento [dir]        open a project directory (default: current directory)
  bento --version    print the version
  bento --help       print this help

Inside Bento, press ? for the full keyboard reference.`

func main() {
	os.Exit(run(os.Args[1:]))
}

type options struct {
	dir         string
	showVersion bool
	showHelp    bool
	err         error
}

type sessionModel struct {
	model   tea.Model
	version string
	width   int
	height  int
}

func (m *sessionModel) Init() tea.Cmd { return m.model.Init() }

func (m *sessionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = size.Width, size.Height
	}
	if opened, ok := msg.(home.OpenMsg); ok {
		project := app.New(opened.Dir, m.version)
		m.model = project
		cmds := make([]tea.Cmd, 0, 2)
		// The startup WindowSizeMsg was delivered to the home model, so the
		// project would otherwise render blank until the next resize.
		if m.width > 0 && m.height > 0 {
			_, sizeCmd := project.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
			cmds = append(cmds, sizeCmd)
		}
		cmds = append(cmds, project.Init())
		return m, tea.Batch(cmds...)
	}
	next, cmd := m.model.Update(msg)
	m.model = next
	return m, cmd
}

func (m *sessionModel) View() string { return m.model.View() }

func parseArgs(args []string) options {
	var opt options
	for _, a := range args {
		switch {
		case a == "--version" || a == "-v":
			opt.showVersion = true
		case a == "--help" || a == "-h":
			opt.showHelp = true
		case strings.HasPrefix(a, "-"):
			if opt.err == nil {
				opt.err = fmt.Errorf("unknown flag %q", a)
			}
		default:
			if opt.dir != "" {
				if opt.err == nil {
					opt.err = fmt.Errorf("unexpected argument %q (only one directory is supported)", a)
				}
				continue
			}
			opt.dir = a
		}
	}
	return opt
}

func run(args []string) (code int) {
	defer func() {
		if r := recover(); r != nil {
			restoreConsole()
			fmt.Fprintf(os.Stderr, "bento: fatal error: %v\n\n%s\n", r, debug.Stack())
			code = 1
		}
	}()

	opt := parseArgs(args)
	if opt.err != nil {
		fmt.Fprintf(os.Stderr, "bento: %v\n\n%s\n", opt.err, usage)
		return 2
	}
	if opt.showVersion {
		fmt.Printf("bento %s\n", version)
		return 0
	}
	if opt.showHelp {
		fmt.Println(usage)
		return 0
	}

	cfg, _ := config.Load()
	if opt.dir == "" {
		if !stdoutIsTTY() {
			fmt.Fprintln(os.Stderr, "bento: stdout is not a terminal; run Bento inside Windows Terminal")
			return 1
		}
		recent := append([]string{}, cfg.Recents...)
		if cfg.LastDir != "" {
			recent = append([]string{cfg.LastDir}, recent...)
		}
		session := &sessionModel{model: home.New(version, recent), version: version}
		p := tea.NewProgram(session, tea.WithAltScreen(), tea.WithMouseCellMotion())
		final, err := p.Run()
		if fm, ok := final.(*sessionModel); ok {
			if project, ok := fm.model.(*app.Model); ok {
				snap := project.Snapshot()
				cfg.LastDir, cfg.LastFile, cfg.Focus = snap.Dir, snap.File, snap.Focus
				cfg.ShowTree, cfg.ShowAgent, cfg.Line, cfg.Col = snap.ShowTree, snap.ShowAgent, snap.Line, snap.Col
				cfg.Recents = addRecent(cfg.Recents, snap.Dir)
				_ = config.Save(cfg)
			}
		}
		if err != nil {
			restoreConsole()
			fmt.Fprintf(os.Stderr, "bento: %v\n", err)
			return 1
		}
		return 0
	}
	requested := opt.dir
	dir, err := resolveDir(requested)
	if err != nil && opt.dir == "" && requested != "" {
		// The remembered project moved or disappeared: fall back to cwd.
		dir, err = resolveDir("")
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "bento: %v\n", err)
		return 1
	}

	if !stdoutIsTTY() {
		fmt.Fprintln(os.Stderr, "bento: stdout is not a terminal; run Bento inside Windows Terminal")
		return 1
	}

	// TODO(user): personal builds deliberately do not check for updates.
	// Updating means rebuilding locally. If an update check is ever wanted,
	// implement it here and call it before starting the program.
	checkForUpdates()

	model := app.New(dir, version)
	if cfg.Valid && strings.EqualFold(dir, cfg.LastDir) {
		model.Restore(app.Snapshot{
			Dir:       cfg.LastDir,
			File:      cfg.LastFile,
			Focus:     cfg.Focus,
			ShowTree:  cfg.ShowTree,
			ShowAgent: cfg.ShowAgent,
			Line:      cfg.Line,
			Col:       cfg.Col,
		})
	}

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	final, err := p.Run()
	if fm, ok := final.(*app.Model); ok {
		snap := fm.Snapshot()
		_ = config.Save(config.State{
			LastDir:   snap.Dir,
			LastFile:  snap.File,
			Focus:     snap.Focus,
			ShowTree:  snap.ShowTree,
			ShowAgent: snap.ShowAgent,
			Line:      snap.Line,
			Col:       snap.Col,
			Recents:   addRecent(cfg.Recents, snap.Dir),
		})
	}
	if err != nil {
		restoreConsole()
		fmt.Fprintf(os.Stderr, "bento: %v\n", err)
		return 1
	}
	return 0
}

func resolveDir(dir string) (string, error) {
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("cannot determine current directory: %w", err)
		}
		dir = wd
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("cannot resolve %q: %w", dir, err)
	}
	abs = filepath.Clean(abs)
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("directory does not exist: %s", abs)
		}
		return "", fmt.Errorf("cannot access %s: %w", abs, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", abs)
	}
	return abs, nil
}

func addRecent(recent []string, dir string) []string {
	if dir == "" {
		return recent
	}
	out := []string{dir}
	for _, path := range recent {
		if !strings.EqualFold(filepath.Clean(path), filepath.Clean(dir)) {
			out = append(out, path)
		}
		if len(out) == 20 {
			break
		}
	}
	return out
}

func stdoutIsTTY() bool {
	fd := os.Stdout.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

// restoreConsole undoes the escape sequences Bento enables, so a crash never
// leaves the terminal in the alternate screen with a hidden cursor.
func restoreConsole() {
	if !stdoutIsTTY() {
		return
	}
	_, _ = os.Stdout.WriteString("\x1b[?1049l\x1b[?25h\x1b[0m")
}

// checkForUpdates is an intentionally unimplemented hook for personal builds.
func checkForUpdates() {}
