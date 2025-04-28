// Package gum provides a Starlark module for TUI, inspired charmbracelet/gum, huh and bubbletea.
package gum

import (
	"errors"
	"fmt"
	"strings"

	"github.com/1set/starlet"
	"github.com/1set/starlet/dataconv"
	"github.com/1set/starlet/dataconv/types"
	"github.com/charmbracelet/huh"
	"github.com/starpkg/base"
	"go.starlark.net/starlark"
)

const (
	// ModuleName defines the module name.
	ModuleName = "gum"
)

// Configuration key constants
const (
	configKeyWidth  = "width"
	configKeyHeight = "height"
	configKeyTheme  = "theme"
)

var (
	none  = starlark.None
	empty string
)

// Module wraps the ConfigurableModule with specific functionality for TUI components.
type Module struct {
	cfgMod  *base.ConfigurableModule
	ext     *base.ConfigurableModuleExt
	theme   *huh.Theme
	keymap  *huh.KeyMap
	isReady bool
}

// NewModule creates a new instance of Module with default configurations.
func NewModule() *Module {
	return newModuleWithOptions(
		genConfigOption(configKeyWidth, "Default width for components", 50), // (0 for terminal width)
		genConfigOption(configKeyHeight, "Default height for components", 0),
		genConfigOption(configKeyTheme, "Theme name to use (base, base16, charm, dracula, catppuccin)", "charm"),
	)
}

// NewModuleWithConfig creates a new instance of Module with the given configuration values.
func NewModuleWithConfig(width, height int, themeName string) *Module {
	return newModuleWithOptions(
		genConfigOption(configKeyWidth, "Default width for components with preset value", width),
		genConfigOption(configKeyHeight, "Default height for components with preset value", height),
		genConfigOption(configKeyTheme, "Theme name to use with preset value", themeName),
	)
}

// genConfigOption creates a configuration option with common settings.
// It sets up the name, description, default value, and environment variable.
func genConfigOption[T any](name, description string, defaultValue T) *base.ConfigOption[T] {
	return base.NewConfigOption(defaultValue).
		WithName(name).
		WithDescription(description).
		WithEnvVar(strings.ToUpper(ModuleName + "_" + name))
}

// newModuleWithOptions creates a Module with the given configuration options.
func newModuleWithOptions(widthOpt *base.ConfigOption[int], heightOpt *base.ConfigOption[int], themeOpt *base.ConfigOption[string]) *Module {
	cm, _ := base.NewConfigurableModuleWithConfigOptions(
		widthOpt,
		heightOpt,
		themeOpt,
	)
	return &Module{
		cfgMod: cm,
		ext:    cm.Extend(),
	}
}

// LoadModule returns the Starlark module loader with the gum-specific functions.
func (m *Module) LoadModule() starlet.ModuleLoader {
	// Initialize module components
	m.initialize()

	// Additional module functions
	additionalFuncs := starlark.StringDict{
		"write":        starlark.NewBuiltin(ModuleName+".write", m.starWrite),
		"input":        starlark.NewBuiltin(ModuleName+".input", m.starInput),
		"select":       starlark.NewBuiltin(ModuleName+".select", m.starSelect),
		"multi_select": starlark.NewBuiltin(ModuleName+".multi_select", m.starMultiSelect),
		"confirm":      starlark.NewBuiltin(ModuleName+".confirm", m.starConfirm),
		"note":         starlark.NewBuiltin(ModuleName+".note", m.starNote),
		"md":           starlark.NewBuiltin(ModuleName+".md", m.starMarkdown),
		"md_note":      starlark.NewBuiltin(ModuleName+".md_note", m.starMarkdownNote),
		"spin":         starlark.NewBuiltin(ModuleName+".spin", m.starSpinner),
		"file_pick":    starlark.NewBuiltin(ModuleName+".file_pick", m.starFilePicker),
		"colorize":     starlark.NewBuiltin(ModuleName+".colorize", m.starColorize),
		// override the default set_theme function
		"set_theme": starlark.NewBuiltin(ModuleName+".set_theme", m.starSetTheme),
	}
	return m.cfgMod.LoadModule(ModuleName, additionalFuncs)
}

// initialize prepares the module state based on configuration.
func (m *Module) initialize() {
	if m.isReady {
		return
	}

	// Set up theme
	themeName := m.ext.GetString(configKeyTheme, "charm")
	m.theme = m.applyTheme(themeName)

	// Set up keymap
	m.keymap = huh.NewDefaultKeyMap()
	m.keymap.Text.NewLine.SetHelp("ctrl+j", "new line")
	// m.keymap.Quit = key.NewBinding(key.WithKeys("ctrl+c", "esc"))
	m.keymap.FilePicker.Open.SetEnabled(false)

	m.isReady = true
}

// getWidth returns the configured width or default value.
func (m *Module) getWidth(width int) int {
	if width > 0 {
		return width
	}
	return m.ext.GetInt(configKeyWidth, 50)
}

// getHeight returns the configured height or default value.
func (m *Module) getHeight(height int) int {
	if height > 0 {
		return height
	}
	return m.ext.GetInt(configKeyHeight, 0)
}

func ignorableError(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, huh.ErrUserAborted) || errors.Is(err, huh.ErrTimeout) {
		return true
	}
	return false
}

// convertList is a generic function to convert values from a OneOrMany container to a slice of T
func convertList[T any, V starlark.Value](raw *types.OneOrMany[V], converter func(V) T) []T {
	if raw == nil || raw.Len() == 0 {
		return nil
	}
	result := make([]T, raw.Len())
	for i, v := range raw.Slice() {
		result[i] = converter(v)
	}
	return result
}

// convertListToStrings converts a OneOrMany of Starlark values to a slice of Go strings
func convertListToStrings[V starlark.Value](raw *types.OneOrMany[V]) []string {
	return convertList(raw, func(v V) string {
		return dataconv.StarString(v)
	})
}

// genericValidator is a type that can be either a string or a []string
type genericValidator interface {
	string | []string
}

func convertValidator[T genericValidator](thread *starlark.Thread, nc *types.NullableCallable) func(T) error {
	if nc.IsNull() {
		return func(v T) error {
			return nil
		}
	}

	return func(v T) error {
		fc := nc.Value()
		var arg starlark.Value

		switch val := any(v).(type) {
		case string:
			arg = starlark.String(val)
		case []string:
			ss := make([]starlark.Value, len(val))
			for i, s := range val {
				ss[i] = starlark.String(s)
			}
			arg = starlark.NewList(ss)
		default:
			return fmt.Errorf("unsupported type: %T", v)
		}

		// Call the validator function in Starlark
		nt := &starlark.Thread{Name: "validate", Load: thread.Load, Print: thread.Print, OnMaxSteps: thread.OnMaxSteps}
		res, err := starlark.Call(nt, fc, starlark.Tuple{arg}, nil)
		if err != nil {
			return fmt.Errorf("validator error: %v", err)
		}

		// Treat None or nil as valid
		if res == nil || res == starlark.None {
			return nil
		}

		// Treat other values as error message, except empty string
		ss := dataconv.StarString(res)
		if ss == "" {
			return nil
		}
		return errors.New(ss)
	}
}

func convertStringValidator(thread *starlark.Thread, nc *types.NullableCallable) func(string) error {
	return convertValidator[string](thread, nc)
}

func convertStringListValidator(thread *starlark.Thread, nc *types.NullableCallable) func([]string) error {
	return convertValidator[[]string](thread, nc)
}

// applyTheme applies a theme based on its name.
func (m *Module) applyTheme(themeName string) *huh.Theme {
	switch strings.ToLower(themeName) {
	case "base":
		return huh.ThemeBase()
	case "base16":
		return huh.ThemeBase16()
	case "charm":
		return huh.ThemeCharm()
	case "dracula":
		return huh.ThemeDracula()
	case "catppuccin":
		return huh.ThemeCatppuccin()
	default: // "charm" is default
		return huh.ThemeCharm()
	}
}

// starSetTheme implements the set_theme function in Starlark.
// It updates the config option directly via the extension API and then immediately applies the theme change.
// Available themes: "base", "base16", "charm", "dracula", "catppuccin"
// This custom implementation ensures that the theme changes take effect immediately rather than only updating the configuration value.
func (m *Module) starSetTheme(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// Extract the theme name
	var themeName starlark.String
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "theme", &themeName); err != nil {
		return starlark.None, err
	}

	// Find the theme config option
	option, err := m.cfgMod.GetConfigOption(configKeyTheme)
	if err != nil {
		return starlark.None, fmt.Errorf("failed to get theme config option: %w", err)
	}

	// Set the value using Starlark value
	if err := option.SetValueFromStarlark(themeName); err != nil {
		return starlark.None, fmt.Errorf("failed to set theme: %w", err)
	}

	// Apply the theme immediately
	m.theme = m.applyTheme(themeName.GoString())

	return starlark.None, nil
}
