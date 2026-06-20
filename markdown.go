package gum

import (
	"fmt"
	"os"
	"strings"

	glamour "charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"github.com/1set/starlet/dataconv/types"
	"go.starlark.net/starlark"
)

// starMarkdown is a Starlark function to render markdown text to ANSI terminal output.
// def md(text: str, style: str = "auto", width: int = 0, emoji: bool = True, word_wrap: bool = True) -> str
// Available styles from glamour package:
// - "auto": Automatically detect terminal background
// - "ascii": Plain ASCII style
// - "dark": Dark theme
// - "dracula": Dracula theme
// - "light": Light theme
// - "notty": No TTY style
// - "pink": Pink theme
// - "tokyo-night": Tokyo Night theme
// - Custom style file path
func (m *Module) starMarkdown(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		textMd   = types.StringOrBytes("")                // markdown text to render
		style    = types.NewNullableStringOrBytes("auto") // style to use (auto, dark, light, notty, or path to custom style)
		width    = 0                                      // width to wrap text (0 = use module width)
		emoji    = true                                   // enable emoji support
		wordWrap = true                                   // enable word wrapping
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"text", &textMd,
		"style?", style,
		"width?", &width,
		"emoji?", &emoji,
		"word_wrap?", &wordWrap,
	); err != nil {
		return none, err
	}

	// Get text content
	if textMd.IsEmpty() {
		return none, fmt.Errorf("text is required and cannot be empty")
	}
	text := textMd.GoString()

	// Create renderer options
	opts := []glamour.TermRendererOption{}

	// Handle style
	styleStr := style.GoString()
	if style.IsNullOrEmpty() {
		styleStr = "auto"
	}
	normalizedStyle := strings.ToLower(styleStr)
	if normalizedStyle == "auto" {
		// glamour v2 removed WithAutoStyle — the renderer is now "pure" and no
		// longer probes the terminal itself. Resolve light/dark here so "auto"
		// keeps adapting to the terminal background. HasDarkBackground returns
		// true (dark) on a non-TTY, matching glamour v2's documented default.
		opts = append(opts, glamour.WithStandardStyle(mdAutoStyleName(lipgloss.HasDarkBackground(os.Stdin, os.Stdout))))
	} else if _, err := os.Stat(styleStr); err == nil {
		// If style is a file path
		opts = append(opts, glamour.WithStylePath(styleStr))
	} else {
		// Try as a standard style name
		opts = append(opts, glamour.WithStandardStyle(normalizedStyle))
	}

	// Add other options
	if wordWrap {
		opts = append(opts, glamour.WithWordWrap(m.getWidth(width)))
	}
	if emoji {
		opts = append(opts, glamour.WithEmoji())
	}

	// glamour v2 is "pure": it always emits full-color ANSI and no longer
	// accepts a WithColorProfile downgrade hint (removed upstream). Color
	// downsampling for limited terminals now happens at print time, not here.

	// Create the renderer
	r, err := glamour.NewTermRenderer(opts...)
	if err != nil {
		return none, fmt.Errorf("failed to create markdown renderer: %v", err)
	}

	// Render the markdown
	out, err := r.Render(text)
	if err != nil {
		return none, fmt.Errorf("failed to render markdown: %v", err)
	}

	// Return the rendered markdown as a string
	return starlark.String(out), nil
}

// starMarkdownNote is a Starlark function to render markdown text and display it in a TUI note.
// def md_note(text: str, title: str = "", style: str = "auto", width: int = 0, height: int = 0, emoji: bool = True, word_wrap: bool = True, show_help: bool = False, next: str = "", timeout: float = 0) -> None
func (m *Module) starMarkdownNote(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		textMd     = types.StringOrBytes("")                // markdown text to render
		title      = types.NewNullableStringOrBytes("")     // title for the markdown display
		style      = types.NewNullableStringOrBytes("auto") // style to use (auto, dark, light, notty, or path to custom style)
		width      = 0                                      // width to wrap text (0 = use module width)
		height     = 0                                      // height for the note display (0 = use module height)
		emoji      = true                                   // enable emoji support
		wordWrap   = true                                   // enable word wrapping
		showHelp   = false                                  // show help text
		wordNext   = types.NewNullableStringOrBytes("")     // next word for note
		timeoutSec = types.FloatOrInt(0)                    // timeout in seconds (0 for no timeout)
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"text", &textMd,
		"title?", title,
		"style?", style,
		"width?", &width,
		"height?", &height,
		"emoji?", &emoji,
		"word_wrap?", &wordWrap,
		"show_help?", &showHelp,
		"next?", wordNext,
		"timeout?", &timeoutSec,
	); err != nil {
		return none, err
	}

	// First render the markdown to a string using starMarkdown
	mdArgs := starlark.Tuple{starlark.String(textMd.GoString())}
	mdKwargs := []starlark.Tuple{
		{starlark.String("style"), starlark.String(style.GoString())},
		{starlark.String("width"), starlark.MakeInt(width)},
		{starlark.String("emoji"), starlark.Bool(emoji)},
		{starlark.String("word_wrap"), starlark.Bool(wordWrap)},
	}
	rendered, err := m.starMarkdown(thread, b, mdArgs, mdKwargs)
	if err != nil {
		return none, err
	}

	// Then display the rendered markdown in a note
	noteArgs := starlark.Tuple{starlark.String(title.GoString())}
	noteKwargs := []starlark.Tuple{
		{starlark.String("description"), rendered},
		{starlark.String("height"), starlark.MakeInt(height)},
		{starlark.String("next"), starlark.String(wordNext.GoString())},
		{starlark.String("show_help"), starlark.Bool(showHelp)},
		{starlark.String("timeout"), starlark.Float(timeoutSec.GoFloat())},
	}
	return m.starNote(thread, b, noteArgs, noteKwargs)
}

// mdAutoStyleName maps a terminal background (dark/light) to the glamour
// builtin style name for the "auto" style. It replaces glamour v2's removed
// WithAutoStyle: the caller supplies the detected background and this picks the
// matching standard style ("dark" on a dark or non-TTY terminal, "light"
// otherwise).
func mdAutoStyleName(isDark bool) string {
	if isDark {
		return "dark"
	}
	return "light"
}
