package app

// Binding is one entry in the global keymap. It is the single source of truth
// for both key handling and the help overlay.
type Binding struct {
	Keys  []string
	Help  string
	Scope string
}

// GlobalBindings lists shortcuts handled by the root model. The help overlay
// renders directly from this table, so it never advertises unimplemented keys.
var GlobalBindings = []Binding{
	{Keys: []string{"tab", "shift+tab"}, Help: "cycle focus", Scope: "Global"},
	{Keys: []string{"alt+1..5"}, Help: "jump to pane (Tree, Preview, Terminal, Changes, Agent)", Scope: "Global"},
	{Keys: []string{"alt+t"}, Help: "toggle file tree column", Scope: "Global"},
	{Keys: []string{"alt+a"}, Help: "toggle agent column", Scope: "Global"},
	{Keys: []string{"ctrl+p"}, Help: "fuzzy file finder", Scope: "Global"},
	{Keys: []string{"ctrl+k"}, Help: "command palette", Scope: "Global"},
	{Keys: []string{"alt+z"}, Help: "zoom focused pane", Scope: "Global"},
	{Keys: []string{"ctrl+f"}, Help: "search in open file", Scope: "Global"},
	{Keys: []string{"ctrl+s"}, Help: "save file (editor)", Scope: "Global"},
	{Keys: []string{"ctrl+a"}, Help: "focus agent launcher", Scope: "Global"},
	{Keys: []string{"ctrl+t"}, Help: "new shell tab", Scope: "Global"},
	{Keys: []string{"alt+[", "alt+]"}, Help: "switch console tabs", Scope: "Terminal"},
	{Keys: []string{"alt+w"}, Help: "close shell tab", Scope: "Terminal"},
	{Keys: []string{"?"}, Help: "help", Scope: "Global"},
	{Keys: []string{"ctrl+q"}, Help: "quit", Scope: "Global"},
}
