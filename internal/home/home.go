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
	case "down", "j":
		if m.cursor < items-1 {
			m.cursor++
		}
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
	}
	return nil
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
	w := m.width
	if w < 60 {
		w = 60
	}
	content := make([]string, 0, 20)
	title := lipgloss.NewStyle().Foreground(theme.P.Text).Bold(true).Render("BENTO")
	content = append(content, title, "One project. Multi-pane. Keyboard-first.", "")
	if m.mode != menuMode {
		label := "Folder path"
		if m.mode == cloneMode {
			label = "Repository URL"
		}
		content = append(content, lipgloss.NewStyle().Bold(true).Render(label), "> "+string(m.input)+"_", "", m.message, "", "enter continue   esc back")
		return lipgloss.NewStyle().Width(min(w-8, 78)).Padding(2, 4).Border(lipgloss.RoundedBorder()).Render(strings.Join(content, "\n"))
	}
	content = append(content, lipgloss.NewStyle().Bold(true).Render("Recent projects"))
	for i, path := range m.Recent {
		content = append(content, m.item(i == m.cursor, filepath.Base(path), path))
	}
	content = append(content,
		m.item(m.cursor == len(m.Recent), "Open folder", "Choose a project directory"),
		m.item(m.cursor == len(m.Recent)+1, "Clone repo", "Clone with git"),
		m.item(m.cursor == len(m.Recent)+2, "Quit", ""),
		"", "↑/↓ select   enter open   q quit")
	return lipgloss.NewStyle().Width(min(w-8, 78)).Padding(2, 4).Border(lipgloss.RoundedBorder()).Render(strings.Join(content, "\n"))
}

func (m *Model) item(selected bool, title, detail string) string {
	style := lipgloss.NewStyle().Padding(0, 1)
	if selected {
		style = style.Background(theme.P.Selection).Foreground(theme.P.Text).Bold(true)
	}
	return style.Render(title + "\n  " + detail)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
