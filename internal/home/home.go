// Package home implements Bento's project picker shown before a project opens.
package home

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lakshwin/bento/internal/theme"
	"github.com/lakshwin/bento/internal/ui"
	"github.com/lakshwin/bento/internal/winutil"
)

type OpenMsg struct{ Dir string }
type CloneDoneMsg struct {
	Dir string
	Err error
}
type FolderPickedMsg struct {
	Dir string
	Err error
}

type mode int

const (
	menuMode mode = iota
	openMode
	cloneMode
)

type Model struct {
	Version string
	Recent  []string
	cursor  int
	scroll  int
	mode    mode
	input   []rune
	width   int
	height  int
	message string
}

func New(version string, recent []string) *Model {
	clean := make([]string, 0, len(recent))
	seen := map[string]bool{}
	for _, path := range recent {
		abs, err := filepath.Abs(path)
		if err == nil && !seen[strings.ToLower(abs)] {
			if info, statErr := os.Stat(abs); statErr == nil && info.IsDir() {
				clean = append(clean, abs)
				seen[strings.ToLower(abs)] = true
			}
		}
	}
	return &Model{Version: version, Recent: clean}
}

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clampRecent()
	case CloneDoneMsg:
		if msg.Err != nil {
			m.message = "Clone failed: " + msg.Err.Error()
			m.mode = menuMode
			return m, nil
		}
		return m, func() tea.Msg { return OpenMsg{Dir: msg.Dir} }
	case FolderPickedMsg:
		if msg.Err != nil {
			m.message = "Folder picker failed: " + msg.Err.Error()
			m.mode = menuMode
			return m, nil
		}
		if msg.Dir == "" {
			m.mode = menuMode
			m.message = ""
			return m, nil
		}
		return m, func() tea.Msg { return OpenMsg{Dir: msg.Dir} }
	case tea.KeyMsg:
		return m, m.key(msg)
	}
	return m, nil
}

func (m *Model) key(msg tea.KeyMsg) tea.Cmd {
	if m.mode != menuMode {
		switch msg.String() {
		case "esc":
			m.mode = menuMode
			m.input = nil
			m.message = ""
		case "enter":
			return m.submitInput()
		case "backspace":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
		default:
			if msg.Type == tea.KeyRunes {
				m.input = append(m.input, msg.Runes...)
			}
		}
		return nil
	}
	items := len(m.Recent) + 3
	switch msg.String() {
	case "q", "ctrl+q", "esc":
		return tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		m.clampRecent()
	case "down", "j":
		if m.cursor < items-1 {
			m.cursor++
		}
		m.clampRecent()
	case "enter", "o":
		if m.cursor < len(m.Recent) {
			return func() tea.Msg { return OpenMsg{Dir: m.Recent[m.cursor]} }
		}
		if m.cursor == len(m.Recent) {
			m.mode, m.input, m.message = menuMode, nil, "Choose a project folder in the Windows picker"
			return pickFolderCmd(mustGetwd())
		} else if m.cursor == len(m.Recent)+1 {
			m.mode, m.input, m.message = cloneMode, nil, "Enter a repository URL"
		} else {
			return tea.Quit
		}
	default:
		if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
			r := msg.Runes[0]
			if r >= '1' && r <= '9' {
				idx := int(r - '1')
				if idx < len(m.Recent) {
					return func() tea.Msg { return OpenMsg{Dir: m.Recent[idx]} }
				}
			}
		}
	}
	return nil
}

// recentVisible reports how many recent rows fit in the card at the current
// size. The two card modes reserve different amounts of chrome.
func (m *Model) recentVisible() int {
	if m.height <= 0 {
		return len(m.Recent)
	}
	fixed := 15
	if m.height < 30 {
		fixed = 7
	}
	avail := m.height - 4 - fixed
	if avail < 2 {
		return 0
	}
	n := avail / 2
	if n > len(m.Recent) {
		n = len(m.Recent)
	}
	return n
}

func (m *Model) clampRecent() {
	vis := m.recentVisible()
	if vis <= 0 {
		m.scroll = 0
		return
	}
	if m.cursor < len(m.Recent) {
		if m.cursor < m.scroll {
			m.scroll = m.cursor
		}
		if m.cursor >= m.scroll+vis {
			m.scroll = m.cursor - vis + 1
		}
	}
	maxScroll := len(m.Recent) - vis
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scroll > maxScroll {
		m.scroll = maxScroll
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
}

func (m *Model) submitInput() tea.Cmd {
	// Users commonly paste quoted paths or paths copied with forward slashes.
	// Normalize those before asking Windows to inspect the directory.
	value := strings.Trim(strings.TrimSpace(string(m.input)), "\"'")
	if value == "" {
		m.message = "Enter a value first"
		return nil
	}
	if m.mode == openMode {
		path := filepath.Clean(filepath.FromSlash(value))
		abs := path
		var err error
		if !filepath.IsAbs(path) {
			abs, err = filepath.Abs(path)
		}
		if err != nil {
			m.message = "Invalid folder path: " + value
			return nil
		}
		info, err := os.Stat(abs)
		if err != nil {
			if os.IsNotExist(err) {
				m.message = "Folder not found: " + abs
			} else {
				m.message = "Cannot open folder: " + err.Error()
			}
			return nil
		}
		if !info.IsDir() {
			m.message = "Not a folder: " + abs
			return nil
		}
		return func() tea.Msg { return OpenMsg{Dir: abs} }
	}
	url := value
	m.mode, m.input, m.message = menuMode, nil, "Cloning repository..."
	dest := filepath.Join(mustGetwd(), filepath.Base(strings.TrimSuffix(url, "/")))
	return func() tea.Msg {
		cmd := exec.Command("git", "clone", url, dest)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return CloneDoneMsg{Err: fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))}
		}
		return CloneDoneMsg{Dir: dest}
	}
}

// pickFolderCmd opens the modern Windows Explorer-style folder picker
// (IFileOpenDialog with FOS_PICKFOLDERS). It is the same large dialog File
// Explorer uses, so users can browse and click a folder instead of typing a
// path. The legacy System.Windows.Forms.FolderBrowserDialog is avoided because
// it renders the small, dated "Browse For Folder" tree.
func pickFolderCmd(initialDir string) tea.Cmd {
	return func() tea.Msg {
		const script = `$ErrorActionPreference = 'Stop'
Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;

[ComImport, Guid("42f85136-db7e-439c-85f1-e4075d135fc8"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
internal interface IFileDialog {
    [PreserveSig] int Show(IntPtr parent);
    void SetFileTypes(uint cFileTypes, IntPtr rgFilterSpec);
    void SetFileTypeIndex(uint iFileType);
    uint GetFileTypeIndex();
    uint Advise(IntPtr pfde);
    void Unadvise(uint dwCookie);
    void SetOptions(uint fos);
    void GetOptions(out uint pfos);
    void SetDefaultFolder(IShellItem psi);
    void SetFolder(IShellItem psi);
    IShellItem GetFolder();
    IShellItem GetCurrentSelection();
    void SetFileName([MarshalAs(UnmanagedType.LPWStr)] string pszName);
    [return: MarshalAs(UnmanagedType.LPWStr)] string GetFileName();
    void SetTitle([MarshalAs(UnmanagedType.LPWStr)] string pszTitle);
    void SetOkButtonLabel([MarshalAs(UnmanagedType.LPWStr)] string pszText);
    void SetFileNameLabel([MarshalAs(UnmanagedType.LPWStr)] string pszLabel);
    IShellItem GetResult();
    void AddPlace(IShellItem psi, int fdap);
    void SetDefaultExtension([MarshalAs(UnmanagedType.LPWStr)] string pszDefaultExtension);
    void Close(int hr);
    void SetClientGuid(ref Guid guid);
    void ClearClientData();
    void SetFilter(IntPtr pFilter);
}

[ComImport, Guid("43826d1e-e718-42ee-bc55-a1e261c37bfe"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
internal interface IShellItem {
    void BindToHandler(IntPtr pbc, ref Guid bhid, ref Guid riid, out IntPtr ppv);
    void GetParent(out IShellItem ppsi);
    void GetDisplayName(uint sigdnName, [MarshalAs(UnmanagedType.LPWStr)] out string ppszName);
    void GetAttributes(uint sfgaoMask, out uint psfgaoAttribs);
    void Compare(IShellItem psi, uint hint, out int piOrder);
}

public static class FolderPicker {
    const uint FOS_PICKFOLDERS = 0x20;
    const uint FOS_FORCEFILESYSTEM = 0x40;
    const uint FOS_NOCHANGEDIR = 0x8;
    const uint FOS_PATHMUSTEXIST = 0x800;
    const uint SIGDN_FILESYSPATH = 0x80058000;
    static readonly Guid CLSID_FileOpenDialog = new Guid("DC1C5A9C-E88A-4DDE-A5A1-60F82A20AEF7");
    static readonly Guid IID_IShellItem = new Guid("43826d1e-e718-42ee-bc55-a1e261c37bfe");

    [DllImport("user32.dll")]
    static extern IntPtr GetForegroundWindow();

    [DllImport("shell32.dll", CharSet = CharSet.Unicode, PreserveSig = false)]
    static extern void SHCreateItemFromParsingName(
        [MarshalAs(UnmanagedType.LPWStr)] string pszPath,
        IntPtr pbc,
        [MarshalAs(UnmanagedType.LPStruct)] Guid riid,
        [MarshalAs(UnmanagedType.Interface)] out IShellItem ppv);

    public static string Show(string title, string initialDir) {
        var dialog = (IFileDialog)Activator.CreateInstance(Type.GetTypeFromCLSID(CLSID_FileOpenDialog));
        try {
            uint opts;
            dialog.GetOptions(out opts);
            dialog.SetOptions(opts | FOS_PICKFOLDERS | FOS_FORCEFILESYSTEM | FOS_PATHMUSTEXIST | FOS_NOCHANGEDIR);
            if (!string.IsNullOrEmpty(title)) dialog.SetTitle(title);
            if (!string.IsNullOrEmpty(initialDir)) {
                try {
                    IShellItem folder;
                    SHCreateItemFromParsingName(initialDir, IntPtr.Zero, IID_IShellItem, out folder);
                    dialog.SetFolder(folder);
                } catch {
                }
            }
            int hr = dialog.Show(GetForegroundWindow());
            if (hr != 0) return "";
            IShellItem item = dialog.GetResult();
            string path;
            item.GetDisplayName(SIGDN_FILESYSPATH, out path);
            Marshal.ReleaseComObject(item);
            return path;
        } finally {
            Marshal.ReleaseComObject(dialog);
        }
    }
}
'@
[FolderPicker]::Show('Choose a Bento project folder', $env:BENTO_PICKER_DIR)
`
		cmd := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-STA", "-Command", script)
		cmd.Env = append(os.Environ(), "BENTO_PICKER_DIR="+initialDir)
		winutil.HideWindow(cmd)
		out, err := cmd.Output()
		if err != nil {
			return FolderPickedMsg{Err: err}
		}
		return FolderPickedMsg{Dir: strings.TrimSpace(string(out))}
	}
}

func mustGetwd() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

func (m *Model) View() string {
	w, h := m.width, m.height
	if w <= 0 || h <= 0 {
		w, h = 80, 24
	}
	cardW := min(w-6, 80)
	if cardW < 30 {
		cardW = max(w-6, 16)
	}
	innerW := max(cardW-6, 10)
	compact := h < 30

	content := make([]string, 0, 28)
	brand := theme.BrandStyle().Render("ben") + theme.BrandAltStyle().Render("to")
	if m.Version != "" && m.Version != "dev" {
		brand += "  " + theme.VersionStyle().Render(" "+m.Version+" ")
	}
	content = append(content, ui.PaintBackground(ui.Fit(brand, innerW), theme.P.Panel))

	if m.mode != menuMode {
		label := "Folder path"
		if m.mode == cloneMode {
			label = "Repository URL"
		}
		content = append(content,
			theme.GroupHeaderStyle().Render(ui.Fit(strings.ToUpper(label), innerW)))
		prompt := theme.InputAccentStyle().Render(" > ") +
			theme.InputStyle().Render(string(m.input)) +
			theme.InputAccentStyle().Render(theme.Cursor)
		prompt = theme.InputStyle().Render(ui.PaintBackground(ui.Fit(prompt, innerW), theme.P.Surface))
		content = append(content, prompt)
		if m.message != "" {
			content = append(content, theme.WarningStyle().Render(ui.Fit(m.message, innerW)))
		}
		content = append(content,
			theme.PanelStyle().Render(strings.Repeat(" ", innerW)),
			m.hintLine([]ui.KeyHint{{Key: "enter", Desc: "continue"}, {Key: "esc", Desc: "back"}}, innerW))
		return m.frame(w, h, cardW, fitContent(content, h))
	}

	if !compact {
		content = append(content,
			theme.MutedStyle().Render(ui.Fit("One project. Multi-pane. Keyboard-first.", innerW)),
			theme.RuleStyle().Render(strings.Repeat("─", innerW)),
			theme.PanelStyle().Render(strings.Repeat(" ", innerW)))
	}
	content = append(content, theme.GroupHeaderStyle().Render(
		ui.Fit(fmt.Sprintf("RECENT PROJECTS  %d", len(m.Recent)), innerW)))

	vis := m.recentVisible()
	if len(m.Recent) == 0 {
		content = append(content,
			theme.MutedStyle().Render(ui.Fit("  No recent projects yet", innerW)),
			theme.MutedStyle().Render(ui.Fit("  Open a folder or clone a repository to begin.", innerW)))
	} else {
		end := m.scroll + vis
		if end > len(m.Recent) {
			end = len(m.Recent)
		}
		for i := m.scroll; i < end; i++ {
			content = append(content, m.recentItem(i == m.cursor, i, filepath.Base(m.Recent[i]), m.Recent[i], innerW))
		}
		if end < len(m.Recent) {
			content = append(content, theme.MutedStyle().Render(
				ui.Fit(fmt.Sprintf("  … %d more", len(m.Recent)-end), innerW)))
		}
	}

	content = append(content, theme.PanelStyle().Render(strings.Repeat(" ", innerW)))
	content = append(content,
		m.menuItem(m.cursor == len(m.Recent), "Open folder", "Choose a project directory", innerW, compact),
		m.menuItem(m.cursor == len(m.Recent)+1, "Clone repo", "Clone with git", innerW, compact),
		m.menuItem(m.cursor == len(m.Recent)+2, "Quit", "Close Bento", innerW, compact))
	if !compact {
		content = append(content, theme.PanelStyle().Render(strings.Repeat(" ", innerW)))
	}
	content = append(content, m.hintLine([]ui.KeyHint{
		{Key: "↑↓", Desc: "select"},
		{Key: "1-9", Desc: "open recent"},
		{Key: "enter", Desc: "open"},
		{Key: "q", Desc: "quit"},
	}, innerW))
	return m.frame(w, h, cardW, fitContent(content, h))
}

func fitContent(content []string, h int) []string {
	max := h - 4
	if max < 1 {
		return content[:0]
	}
	if len(content) > max {
		return content[:max]
	}
	return content
}

func (m *Model) hintLine(hints []ui.KeyHint, width int) string {
	line := ui.HintLine(hints)
	return theme.PanelStyle().Render(ui.PaintBackground(ui.Fit(line, width), theme.P.Panel))
}

func (m *Model) recentItem(selected bool, index int, title, detail string, width int) string {
	marker := "  "
	titleStyle := theme.TextStyle().Bold(true)
	detailStyle := theme.MutedStyle()
	if selected {
		marker = theme.AccentBar + " "
		titleStyle = theme.SelectedAccentStyle()
		detailStyle = theme.SelectedMutedStyle()
	}
	head := " " + marker + " " + fmt.Sprintf("%d", index+1) + "  " + title
	sub := "      " + detail
	return titleStyle.Render(ui.Fit(head, width)) + "\n" +
		detailStyle.Render(ui.Fit(sub, width))
}

func (m *Model) menuItem(selected bool, title, detail string, width int, compact bool) string {
	marker := "  "
	titleStyle := theme.TextStyle().Bold(true)
	detailStyle := theme.MutedStyle()
	if selected {
		marker = theme.AccentBar + " "
		titleStyle = theme.SelectedAccentStyle()
		detailStyle = theme.SelectedMutedStyle()
	}
	if compact {
		return titleStyle.Render(ui.Fit(" "+marker+" "+title+"  "+detail, width))
	}
	return titleStyle.Render(ui.Fit(" "+marker+" "+title, width)) + "\n" +
		detailStyle.Render(ui.Fit("     "+detail, width))
}

func (m *Model) frame(w, h, cardW int, content []string) string {
	body := ui.PaintBackground(strings.Join(content, "\n"), theme.P.Panel)
	card := theme.CardStyle().Width(cardW).Render(body)
	lines := strings.Split(card, "\n")
	cw := 0
	for _, line := range lines {
		if n := lipgloss.Width(line); n > cw {
			cw = n
		}
	}
	if cw > w {
		cw = w
	}
	for i := range lines {
		lines[i] = ui.PaintBackground(ui.Fit(lines[i], cw), theme.P.Panel)
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	x := (w - cw) / 2
	if x < 0 {
		x = 0
	}
	y := (h - len(lines)) / 2
	rows := make([]string, h)
	for i := 0; i < h; i++ {
		if i < y || i >= y+len(lines) {
			rows[i] = canvasSpace(w)
			continue
		}
		rows[i] = canvasSpace(x) + lines[i-y] + canvasSpace(w-x-cw)
	}
	return strings.Join(rows, "\n")
}

func canvasSpace(n int) string {
	if n <= 0 {
		return ""
	}
	return theme.CanvasStyle().Render(strings.Repeat(" ", n))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
