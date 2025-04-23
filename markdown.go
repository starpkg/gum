package gum

import (
	"fmt"
	"os"
	"strings"

	"github.com/1set/starlet/dataconv/types"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/huh"
	"github.com/muesli/termenv"
	"go.starlark.net/starlark"
)

// starRenderMarkdown is a Starlark function to render markdown text to ANSI terminal output.
// def render_md(text: str, style: str = "auto", width: int = 0, emoji: bool = True, word_wrap: bool = True, show_help: bool = False, next: str = "") -> str
func (m *Module) starRenderMarkdown(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		text     string                               // markdown text to render
		style    = "auto"                             // style to use (auto, dark, light, notty, or path to custom style)
		width    = 0                                  // width to wrap text (0 = use module width)
		emoji    = true                               // enable emoji support
		wordWrap = true                               // enable word wrapping
		showHelp = false                              // show help text
		wordNext = types.NewNullableStringOrBytes("") // next word for note
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"text", &text,
		"style?", &style,
		"width?", &width,
		"emoji?", &emoji,
		"word_wrap?", &wordWrap,
		"show_help?", &showHelp,
		"next?", wordNext,
	); err != nil {
		return none, err
	}

	// Configure rendering width
	actualWidth := m.getWidth(width)

	// Create renderer options
	opts := []glamour.TermRendererOption{}

	// Handle style
	normalizedStyle := strings.ToLower(style)
	if normalizedStyle == "auto" {
		opts = append(opts, glamour.WithAutoStyle())
	} else if _, err := os.Stat(style); err == nil {
		// If style is a file path
		opts = append(opts, glamour.WithStylePath(style))
	} else {
		// Try as a standard style name
		opts = append(opts, glamour.WithStandardStyle(normalizedStyle))
	}

	// Add other options
	opts = append(opts, glamour.WithWordWrap(actualWidth))

	if emoji {
		opts = append(opts, glamour.WithEmoji())
	}

	// Determine color profile based on terminal capabilities
	if termenv.ColorProfile() == termenv.TrueColor {
		opts = append(opts, glamour.WithColorProfile(termenv.TrueColor))
	} else if termenv.ColorProfile() == termenv.ANSI256 {
		opts = append(opts, glamour.WithColorProfile(termenv.ANSI256))
	} else {
		opts = append(opts, glamour.WithColorProfile(termenv.ANSI))
	}

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

	// If show_help or next is provided, show as a note
	hasNext := !wordNext.IsNullOrEmpty()
	strNext := wordNext.GoString()
	if showHelp || hasNext {
		err := huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("").
					Description(out).
					Height(m.getHeight(0)).
					Next(hasNext).
					NextLabel(strNext),
			),
		).
			WithTheme(m.theme).
			WithKeyMap(m.keymap).
			WithShowHelp(showHelp).
			Run()

		// Handle no result
		if err != nil {
			if ignorableError(err) {
				return none, nil
			}
			return none, err
		}
		return none, nil
	}

	// Otherwise return the rendered string
	return starlark.String(out), nil
}
