package gum

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
	"github.com/1set/starlet/dataconv"
	"github.com/1set/starlet/dataconv/types"
	"go.starlark.net/starlark"
)

// starWrite is a Starlark function to create a TUI text area for getting multi-line input from the user.
// def write(value: str = "", placeholder: str = "Write something...", title: str = "", description: str = "", char_limit: int = 0, validate: Callable = None, width: int = 50, height: int = 5, show_line: bool = false, show_help: bool = true, timeout: float = 0) -> str
// Parameters:
// - value: Initial text value
// - placeholder: Placeholder text when empty
// - title: Title text
// - description: Description text
// - char_limit: Maximum number of characters (0 for no limit)
// - validate: Validation function that returns error message or None
// - width: Text area width (0 for terminal width)
// - height: Text area height
// - show_line: Show line numbers
// - show_help: Show help key binds
// - timeout: Timeout in seconds (0 for no timeout)
//
// The external editor (opened with Ctrl+E) is the host-configured `editor`, which
// huh runs via os/exec — a script cannot NAME it (no per-call editor argument, no
// set_editor; resolveEditor picks it from host-controlled sources only) nor
// PATH-hijack it (the command is frozen to an absolute path at construction).
//
// Residual (not closable within gum): huh/bubbletea, not gum, construct the
// subprocesses they run, and gum cannot set their Cmd.Env (unlike the `cmd`
// module's util.BuildChildEnv). So a host that lets an untrusted script mutate the
// process environment (e.g. by exposing a `runtime` module's `setenv`) leaves two
// surfaces: (1) the editor subprocess inherits the live environment, so editor env
// such as VIMINIT executes for editors that honor it (e.g. vim); (2) bubbletea's
// terminal color-detection execs a bare `tmux` (live PATH) on ANY form — including
// `spin`, whose huh spinner exposes no way to pass a sanitized environment. Close
// these at the host/sandbox level: don't grant untrusted scripts process
// -environment mutation, or run the interpreter with an isolated environment.
func (m *Module) starWrite(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		initialValue    starlark.Value         // initial value, converted to string if not already
		placeholder     = "Write something..." // placeholder value
		title           = ""                   // title text
		description     = ""                   // description text
		charLimit       = 0                    // maximum value length (0 for no limit)
		validateFunc    types.NullableCallable // validation function
		width           = 50                   // text area width (0 for terminal width)
		height          = 5                    // text area height
		showLineNumbers = false                // show line numbers
		showHelp        = true                 // show help key binds
		timeoutSec      = types.FloatOrInt(0)  // timeout in seconds (0 for no timeout)
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"value?", &initialValue,
		"placeholder?", &placeholder,
		"title?", &title,
		"description?", &description,
		"char_limit?", &charLimit,
		"validate?", &validateFunc,
		"width?", &width,
		"height?", &height,
		"show_line?", &showLineNumbers,
		"show_help?", &showHelp,
		"timeout?", &timeoutSec,
	); err != nil {
		return none, err
	}

	// The editor COMMAND is host-configured only — a script can neither pass it per
	// call (no editor= argument) nor set it (no set_editor). resolveEditor picks it
	// from host-controlled sources, so huh runs a host-chosen command, never one the
	// script named. (huh still resolves that command via the live PATH and the
	// subprocess inherits the live environment — see the note above starWrite for
	// the residual that only the host/sandbox can close.)
	value := dataconv.StarString(initialValue)
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewText().
				Title(title).
				Description(description).
				Placeholder(placeholder).
				Validate(convertStringValidator(thread, &validateFunc)).
				CharLimit(charLimit).
				ShowLineNumbers(showLineNumbers).
				Editor(m.resolveEditor()...).
				Value(&value),
		),
	).
		WithWidth(m.getWidth(width)).
		WithHeight(m.getHeight(height)).
		WithTheme(m.theme).
		WithKeyMap(m.keymap).
		WithShowHelp(showHelp).
		WithTimeout(convertDuration(timeoutSec)).
		Run()

	// handle results
	if err != nil {
		if ignorableError(err) {
			return none, nil
		}
		return none, err
	}
	return starlark.String(value), nil
}

// starInput is a Starlark function to create a TUI input field for getting single-line text from the user.
// def input(value: str = "", prompt: str = "> ", placeholder: str = "Type something...", title: str = "", description: str = "", char_limit: int = 0, suggestions: List[str] = [], password: bool = false, validate: Callable = None, width: int = 50, inline: bool = false, show_help: bool = true, timeout: float = 0) -> str
func (m *Module) starInput(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		initialValue starlark.Value                                                          // initial value, converted to string if not already
		prompt                              = "> "                                           // prompt text
		placeholder                         = "Type something..."                            // placeholder value
		title                               = ""                                             // title text
		description                         = ""                                             // description text
		charLimit                           = 0                                              // maximum value length (0 for no limit)
		suggestions                         = types.NewOneOrManyNoDefault[starlark.String]() // suggestions as string or list of strings
		password     starlark.Value         = starlark.Bool(false)                           // password mode
		validateFunc types.NullableCallable                                                  // validation function
		width        = 50                                                                    // text area width (0 for terminal width)
		inline       = false                                                                 // inline mode
		showHelp     = true                                                                  // show help key binds
		timeoutSec   = types.FloatOrInt(0)                                                   // timeout in seconds (0 for no timeout)
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"value?", &initialValue,
		"prompt?", &prompt,
		"placeholder?", &placeholder,
		"title?", &title,
		"description?", &description,
		"char_limit?", &charLimit,
		"suggestions?", suggestions,
		"password?", &password,
		"validate?", &validateFunc,
		"width?", &width,
		"inline?", &inline,
		"show_help?", &showHelp,
		"timeout?", &timeoutSec,
	); err != nil {
		return none, err
	}

	// convert password mode to echo mode
	var echoMode huh.EchoMode
	switch t := password.(type) {
	case starlark.NoneType:
		echoMode = huh.EchoModeNone
	case starlark.Bool:
		if t.Truth() {
			echoMode = huh.EchoModePassword
		} else {
			echoMode = huh.EchoModeNormal
		}
	default:
		return none, fmt.Errorf("%s: password must be a bool or None", b.Name())
	}

	// convert suggestions
	suggests := convertListToStrings(suggestions)

	// run form
	value := dataconv.StarString(initialValue)
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Prompt(prompt).
				Title(title).
				Description(description).
				Placeholder(placeholder).
				Validate(convertStringValidator(thread, &validateFunc)).
				CharLimit(charLimit).
				Suggestions(suggests).
				EchoMode(echoMode).
				Inline(inline).
				Value(&value),
		),
	).
		WithWidth(m.getWidth(width)).
		WithTheme(m.theme).
		WithKeyMap(m.keymap).
		WithShowHelp(showHelp).
		WithTimeout(convertDuration(timeoutSec)).
		WithProgramOptions(tea.WithOutput(os.Stderr)).
		Run()

	// handle results
	if err != nil {
		if ignorableError(err) {
			return none, nil
		}
		return none, err
	}
	return starlark.String(value), nil
}
