package gum

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/1set/starlet/dataconv"
	"github.com/1set/starlet/dataconv/types"
	"github.com/charmbracelet/huh"
	"go.starlark.net/starlark"
)

// starSelect is a Starlark function to create a TUI select for choosing an option from a list.
// def select(options: Union[Iterable, Mapping], value: str = "", title: str = "Choose:", description: str = "", validate: Callable = None, width: int = 50, height: int = 0, inline: bool = False, show_filter: bool = False, show_help: bool = True, timeout: float = 0) -> str
func (m *Module) starSelect(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		options      starlark.Value         // list of option values, or map of key-value pairs of options
		initialValue starlark.Value         // initial value, converted to string if not already
		title        = "Choose:"            // title text
		description  = ""                   // description text
		validateFunc types.NullableCallable // validation function
		width        = 50                   // text area width (0 for terminal width)
		height       = 0                    // maximum number of items to show (0 for all)
		inline       = false                // inline mode
		showFilter   = false                // filtering state as default
		showHelp     = true                 // show help key binds
		timeoutSec   = types.FloatOrInt(0)  // timeout in seconds (0 for no timeout)
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"options", &options,
		"value?", &initialValue,
		"title?", &title,
		"description?", &description,
		"validate?", &validateFunc,
		"width?", &width,
		"height?", &height,
		"inline?", &inline,
		"show_filter?", &showFilter,
		"show_help?", &showHelp,
		"timeout?", &timeoutSec,
	); err != nil {
		return starlark.None, err
	}

	// convert options
	opts, err := convertOptionList(options)
	if err != nil {
		return none, err
	}
	if len(opts) == 0 {
		return none, errors.New("options must not be empty")
	}

	// run form
	value := dataconv.StarString(initialValue)
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(title).
				Description(description).
				Options(opts...).
				Height(m.getHeight(height)).
				Validate(convertStringValidator(thread, &validateFunc)).
				Inline(inline).
				Filtering(showFilter).
				Value(&value),
		),
	).
		WithWidth(m.getWidth(width)).
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

// starMultiSelect is a Starlark function to create a TUI multi-select for choosing multiple options from a list.
// def multi_select(options: Union[Iterable, Mapping], value: List[str] = [], title: str = "Choose:", description: str = "", validate: Callable = None, limit: int = 0, width: int = 50, height: int = 0, show_filter: bool = False, show_help: bool = True, timeout: float = 0) -> List[str]
func (m *Module) starMultiSelect(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		options      starlark.Value                                   // list of option values, or map of key-value pairs of options
		initialValue = types.NewOneOrManyNoDefault[starlark.String]() // initial value as string or list of strings
		title        = "Choose:"                                      // title text
		description  = ""                                             // description text
		validateFunc types.NullableCallable                           // validation function
		limit        = 0                                              // maximum number of items to select (0 for no limit)
		width        = 50                                             // text area width (0 for terminal width)
		height       = 0                                              // maximum number of items to show (0 for all)
		showFilter   = false                                          // filtering state as default
		showHelp     = true                                           // show help key binds
		timeoutSec   = types.FloatOrInt(0)                            // timeout in seconds (0 for no timeout)
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"options", &options,
		"value?", initialValue,
		"title?", &title,
		"description?", &description,
		"validate?", &validateFunc,
		"limit", &limit,
		"width?", &width,
		"height?", &height,
		"show_filter?", &showFilter,
		"show_help?", &showHelp,
		"timeout?", &timeoutSec,
	); err != nil {
		return starlark.None, err
	}

	// convert options
	opts, err := convertOptionList(options)
	if err != nil {
		return none, err
	}
	if len(opts) == 0 {
		return none, errors.New("options must not be empty")
	}

	// convert default values
	values := convertListToStrings(initialValue)

	// run form
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title(title).
				Description(description).
				Options(opts...).
				Limit(limit).
				Height(m.getHeight(height)).
				Validate(convertStringListValidator(thread, &validateFunc)).
				Filtering(showFilter).
				Filterable(true).
				Value(&values),
		),
	).
		WithWidth(m.getWidth(width)).
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
	ss := make([]starlark.Value, len(values))
	for i, v := range values {
		ss[i] = starlark.String(v)
	}
	return starlark.NewList(ss), nil
}

// starConfirm is a Starlark function to create a TUI confirmation dialog for asking a yes/no question.
// def confirm(value: bool = False, title: str = "Are you sure?", description: str = "", yes: str = "Yes", no: str = "No", inline: bool = False, show_help: bool = True, timeout: float = 0) -> bool
func (m *Module) starConfirm(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		initialValue = starlark.Bool(false) // initial value, should be a boolean
		title        = "Are you sure?"      // title text
		description  = ""                   // description text
		wordYes      = "Yes"                // text for affirmative option
		wordNo       = "No"                 // text for negative option
		inline       = false                // inline mode
		showHelp     = true                 // show help key binds
		timeoutSec   = types.FloatOrInt(0)  // timeout in seconds (0 for no timeout)
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"value?", &initialValue,
		"title?", &title,
		"description?", &description,
		"yes?", &wordYes,
		"no?", &wordNo,
		"inline?", &inline,
		"show_help?", &showHelp,
		"timeout?", &timeoutSec,
	); err != nil {
		return starlark.None, err
	}

	// run form
	choice := bool(initialValue)
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Affirmative(wordYes).
				Negative(wordNo).
				Title(title).
				Description(description).
				Inline(inline).
				Value(&choice),
		),
	).
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
	return starlark.Bool(choice), nil
}

// convertOptionList converts from a Starlark iterable/mapping to a list of huh.Options.
func convertOptionList(r starlark.Value) ([]huh.Option[string], error) {
	var (
		opts []huh.Option[string]
	)

	// handle various types of options input
	switch t := r.(type) {
	case *starlark.List:
		// list of option values
		for i := 0; i < t.Len(); i++ {
			v := t.Index(i)
			s := dataconv.StarString(v)
			opts = append(opts, huh.NewOption(s, s))
		}
	case *starlark.Dict:
		// map of key -> value mapping (key is displayed, value is returned)
		for _, k := range t.Keys() {
			v, _, _ := t.Get(k)
			opts = append(opts, huh.NewOption(dataconv.StarString(k), dataconv.StarString(v)))
		}
	case starlark.Iterable:
		// other iterables
		iter := t.Iterate()
		defer iter.Done()
		var v starlark.Value
		for iter.Next(&v) {
			s := dataconv.StarString(v)
			opts = append(opts, huh.NewOption(s, s))
		}
	default:
		return nil, fmt.Errorf("options expected iterable or mapping, got %s", r.Type())
	}

	return opts, nil
}

// starFilePicker is a Starlark function to create a TUI file picker for selecting a file or directory.
// def file_pick(path: str = ".", title: str = "", description: str = "", validate: Callable = None, allow_ext: Union[str, List[str]] = [], allow_dir: bool = False, allow_file: bool = True, show_hidden: bool = False, show_perm: bool = True, show_size: bool = False, height: int = 10, show_help: bool = True, timeout: float = 0) -> str
func (m *Module) starFilePicker(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		initialPath     = "."                                            // initial path string
		title           = ""                                             // title text
		description     = ""                                             // description text
		validateFunc    types.NullableCallable                           // validation function
		allowExtensions = types.NewOneOrManyNoDefault[starlark.String]() // allowed file extensions as string or list of strings
		allowDirs       = false                                          // allow directories
		allowFiles      = true                                           // allow files
		showHidden      = false                                          // show hidden files
		showPermissions = true                                           // show file permissions
		showSize        = false                                          // show file size
		height          = 10                                             // maximum number of items to show (0 for all)
		showHelp        = true                                           // show help key binds
		timeoutSec      = types.FloatOrInt(0)                            // timeout in seconds (0 for no timeout)
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"path?", &initialPath,
		"title?", &title,
		"description?", &description,
		"validate?", &validateFunc,
		"allow_ext?", allowExtensions,
		"allow_dir?", &allowDirs,
		"allow_file?", &allowFiles,
		"show_hidden?", &showHidden,
		"show_perm?", &showPermissions,
		"show_size?", &showSize,
		"height?", &height,
		"show_help?", &showHelp,
		"timeout?", &timeoutSec,
	); err != nil {
		return starlark.None, err
	}

	// convert allowed extensions
	extensions := convertListToStrings(allowExtensions)

	// get initial path
	path, err := filepath.Abs(initialPath)
	if err != nil {
		return none, fmt.Errorf("%s: %w", b.Name(), err)
	}

	// run form
	value := path
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewFilePicker().
				Picking(true).
				CurrentDirectory(path).
				Title(title).
				Description(description).
				Validate(convertStringValidator(thread, &validateFunc)).
				AllowedTypes(extensions).
				DirAllowed(allowDirs).
				FileAllowed(allowFiles).
				ShowHidden(showHidden).
				ShowPermissions(showPermissions).
				ShowSize(showSize).
				Height(height).
				Value(&value),
		),
	).
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
