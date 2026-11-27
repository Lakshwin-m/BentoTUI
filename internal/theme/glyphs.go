package theme

// Glyph constants. Only box-drawing and common Unicode characters that render
// in Cascadia Mono (the Windows Terminal default) are allowed here.
//
// TODO(user): when the final font arrives, swap glyphs in this file only.
const (
	// Tree expansion.
	TreeExpanded  = "\u25be" // ▾
	TreeCollapsed = "\u25b8" // ▸

	// Folding.
	FoldClosed = "\u2026" // …
	FoldOpen   = "\u25be" // ▾

	// Misc punctuation.
	Ellipsis = "\u2026" // …
	Bullet   = "\u2022" // •
	Dirty    = "\u25cf" // ●
	Check    = "\u2713" // ✓
	Cross    = "\u2717" // ✗
	Cursor   = "\u258c" // ▌

	// Scrollbar.
	ScrollbarThumb = "\u2588" // █
	ScrollbarTrack = "\u2502" // │

	// Separators used in the status bar and lists.
	Sep = " \u00b7 " // " · "
	Bar = "\u2502"   // │

	// AccentBar marks the active row in a list.
	AccentBar = "\u258e" // ▎
)
