# Bento

One project. Multi-pane. Keyboard-first.

Bento is a personal, Windows-only terminal workspace (Bubble Tea + Lip Gloss)
that opens a single project directory as a bento grid of panes: file tree,
preview/editor, git changes, builds, embedded shell tabs and AI agent launcher.

## Build and run

```powershell
go test ./...
go build -o bento.exe ./cmd/bento
.\bento.exe
```

One-time install to your user PATH:

```powershell
.\scripts\build.ps1
```

Then `bento [dir]` works from any new Windows Terminal tab.

## Project picker

On launch, Bento shows recent projects. Use `↑`/`↓` and `enter`, or press
`1`–`9` to open a recent project directly. `o` opens a folder, `c` clones a
repository, and `q` quits.

## Keys

| Key | Action |
| --- | --- |
| `tab` / `shift+tab` | cycle pane focus |
| `alt+1..5` | jump to Tree, Preview, Build, Changes, Agent |
| `alt+t` / `alt+a` | toggle tree / agent columns |
| `ctrl+p` | fuzzy file finder |
| `ctrl+k` | command palette |
| `alt+z` | zoom the focused pane |
| `ctrl+f` | search in the open file |
| `ctrl+s` | save |
| `ctrl+a` | focus agent launcher |
| `ctrl+t` | new shell tab |
| `click` | focus a pane; in Agents, launch the clicked tool |
| `?` | help overlay |
| `ctrl+q` | quit (`ctrl+c` also quits except in the shell) |

The in-app help (`?`) is generated from the same keymap the app uses.

## Agents

The Agents pane lists the catalog (Claude Code, OpenCode, Gemini CLI, Codex,
Copilot, Crush, Aider, Cursor Agent). Installed CLIs launch in a ConPTY tab
beside the preview. npm `.cmd` shims (OpenCode and friends) are started through
`cmd.exe` so Windows can actually run them. Missing tools show the vendor
install command; press enter, then `y` to run it.

## Not implemented yet

- Git commits tab, patch viewer, and `fsnotify` auto-refresh
- Viewer folding polish, disk-change warning while editing, atomic save edge cases around locked files
- Build process-tree kill via job object (`taskkill` fallback is not wired)
- Full VT alt-screen apps inside the shell, config.toml (session state is JSON)
- Update check (intentionally omitted for personal builds)
- macOS/Linux support
