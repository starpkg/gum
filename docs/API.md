# `gum` Starlark API Reference

This is the complete reference for the `gum` module's script-facing surface:
every builtin (signature, parameters, returns, errors, examples) and every
configuration accessor. For an overview and quickstart, see the
[README](../README.md).

The module is loaded under the name `gum`; load individual builtins with
`load("gum", "input", "select", ...)`.

## Contents

- [Text input](#text-input)
  - [`input`](#input)
  - [`write`](#write)
- [Selection](#selection)
  - [`select`](#select)
  - [`multi_select`](#multi_select)
  - [`confirm`](#confirm)
- [File picker](#file-picker)
  - [`file_pick`](#file_pick)
- [Visual elements](#visual-elements)
  - [`note`](#note)
  - [`md`](#md)
  - [`md_note`](#md_note)
  - [`spin`](#spin)
  - [`colorize`](#colorize)
- [Theming](#theming)
  - [`set_theme`](#set_theme)
- [Configuration](#configuration)
  - Accessors: `get_width` / `set_width`, `get_height` / `set_height`, `get_theme` / `set_theme`, `get_editor` / `set_editor`

> **TTY note.** Every interactive builtin (`input`, `write`, `select`,
> `multi_select`, `confirm`, `file_pick`, `note`, `spin`) drives the host's
> controlling terminal. In a headless environment (CI, sandbox, a plain run
> without a terminal) they fail with `could not open a new TTY`. The
> non-interactive builtins (`md`, `colorize`) and all argument validation work
> anywhere.

## Text input

### `input`

```text
input(value="", prompt="> ", placeholder="Type something...", title="", description="",
      char_limit=0, suggestions=[], password=False, validate=None, width=<config>,
      inline=False, show_help=True, timeout=0)
```

Creates a single-line text input field.

Parameters:

- `value`: Initial value (default: `""`).
- `prompt`: Input prompt (default: `"> "`).
- `placeholder`: Placeholder text (default: `"Type something..."`).
- `title`: Title text (default: `""`).
- `description`: Description text (default: `""`).
- `char_limit`: Maximum character limit (default: `0` — no limit).
- `suggestions`: A string or list of autocomplete suggestions (default: `[]`).
- `password`: `True` for masked password input, `False` for normal echo, or
  `None` for no echo (default: `False`).
- `validate`: Validation function (default: `None`). It receives the current
  string; return `None` or an empty string for valid input, any other value as
  the error message.
- `width`: Component width (default: the configured `width`).
- `inline`: Display in inline mode (default: `False`).
- `show_help`: Show help key bindings (default: `True`).
- `timeout`: Timeout in seconds (default: `0` — no timeout).

Returns the entered text as a string, or `None` if the prompt is cancelled or
times out.

Errors: `password` must be a bool or `None`.

### `write`

```text
write(value="", placeholder="Write something...", title="", description="",
      char_limit=0, validate=None, editor=None, width=<config>, height=5,
      show_line=False, show_help=True, timeout=0)
```

Creates a multi-line text area.

Parameters:

- `value`: Initial value (default: `""`).
- `placeholder`: Placeholder text (default: `"Write something..."`).
- `title`: Title text (default: `""`).
- `description`: Description text (default: `""`).
- `char_limit`: Maximum character limit (default: `0` — no limit).
- `validate`: Validation function (default: `None`); same contract as `input`.
- `editor`: External editor command as a string (e.g. `"vim"`) or list (e.g.
  `["code", "--wait"]`). When omitted/empty, falls back to the module's
  configured `editor`; if that is also empty, `huh` uses its own default
  (`$EDITOR`, else `nano`).
- `width`: Component width (default: the configured `width`).
- `height`: Component height (default: `5`).
- `show_line`: Show line numbers (default: `False`).
- `show_help`: Show help key bindings (default: `True`).
- `timeout`: Timeout in seconds (default: `0` — no timeout).

Returns the entered text as a string, or `None` if cancelled or timed out.

Example:

```python
load("gum", "write", "set_editor")

# Set the module's default editor.
set_editor(["vim"])

# Uses the module's default editor on Ctrl+E.
notes = write(
    title = "Meeting Notes",
    description = "Press Ctrl+E to open in your default editor",
)

# Override the editor for this call only.
vscode_notes = write(
    title = "VSCode Notes",
    editor = ["code", "--wait"],
)

print("Notes recorded:", len(notes) if notes else 0, "characters")
```

## Selection

### `select`

```text
select(options, value="", title="Choose:", description="", validate=None,
       width=<config>, height=0, inline=False, show_filter=False,
       show_help=True, timeout=0)
```

Creates a single-selection component.

Parameters:

- `options`: A list/iterable of option values, or a dict (keys are displayed,
  values are returned). Required and must not be empty.
- `value`: Initially selected value (default: `""`).
- `title`: Title text (default: `"Choose:"`).
- `description`: Description text (default: `""`).
- `validate`: Validation function (default: `None`); same contract as `input`.
- `width`: Component width (default: the configured `width`).
- `height`: Maximum visible items (default: `0` — all).
- `inline`: Display in inline mode (default: `False`).
- `show_filter`: Enable filtering (default: `False`).
- `show_help`: Show help key bindings (default: `True`).
- `timeout`: Timeout in seconds (default: `0` — no timeout).

Returns the selected value as a string, or `None` if cancelled or timed out.

Errors: `options` must be an iterable or mapping, and must not be empty.

### `multi_select`

```text
multi_select(options, value=[], title="Choose:", description="", validate=None,
             limit=0, width=<config>, height=0, show_filter=False,
             show_help=True, timeout=0)
```

Creates a multi-selection component.

Parameters:

- `options`: A list/iterable of option values, or a dict (keys are displayed,
  values are returned). Required and must not be empty.
- `value`: A string or list of initially selected values (default: `[]`).
- `title`: Title text (default: `"Choose:"`).
- `description`: Description text (default: `""`).
- `validate`: Validation function receiving the list of selected values
  (default: `None`); return `None`/empty string for valid, otherwise the error
  message.
- `limit`: Maximum number of selections (default: `0` — no limit).
- `width`: Component width (default: the configured `width`).
- `height`: Maximum visible items (default: `0` — all).
- `show_filter`: Enable filtering (default: `False`).
- `show_help`: Show help key bindings (default: `True`).
- `timeout`: Timeout in seconds (default: `0` — no timeout).

Returns a list of selected values, or `None` if cancelled or timed out.

Errors: `options` must be an iterable or mapping, and must not be empty.

Example:

```python
load("gum", "select", "multi_select", "note")

# Single selection from a list, with filtering.
color = select(
    options = ["Red", "Green", "Blue", "Yellow", "Purple"],
    title = "Choose your favorite color:",
    show_filter = True,
)

# Multi-selection from a dict (keys shown, values returned).
fruits = multi_select(
    options = {
        "apple": "Apple",
        "banana": "Banana",
        "orange": "Orange",
    },
    value = ["Orange"],
    title = "Select your favorite fruits:",
    limit = 3,
)

if color and fruits:
    note(
        title = "Your Selections",
        description = "Color: " + color + "\nFruits: " + ", ".join(fruits),
        next = "Continue",
    )
```

### `confirm`

```text
confirm(value=False, title="Are you sure?", description="", yes="Yes", no="No",
        inline=False, show_help=True, timeout=0)
```

Creates a yes/no confirmation dialog.

Parameters:

- `value`: Initial value (default: `False`).
- `title`: Title text (default: `"Are you sure?"`).
- `description`: Description text (default: `""`).
- `yes`: Text for the affirmative option (default: `"Yes"`).
- `no`: Text for the negative option (default: `"No"`).
- `inline`: Display in inline mode (default: `False`).
- `show_help`: Show help key bindings (default: `True`).
- `timeout`: Timeout in seconds (default: `0` — no timeout).

Returns a boolean if the user makes a selection, or `None` if cancelled or
timed out.

## File picker

### `file_pick`

```text
file_pick(path=".", title="", description="", validate=None, allow_ext=[],
          allow_dir=False, allow_file=True, show_hidden=False, show_perm=True,
          show_size=False, height=10, show_help=True, timeout=0)
```

Creates a file picker component. The starting path is resolved to an absolute
path before the picker opens.

Parameters:

- `path`: Initial path to start in (default: `"."`).
- `title`: Title text (default: `""`).
- `description`: Description text (default: `""`).
- `validate`: Validation function (default: `None`); same contract as `input`.
- `allow_ext`: Allowed file extensions as a string or list of strings
  (default: `[]` — all).
- `allow_dir`: Allow directory selection (default: `False`).
- `allow_file`: Allow file selection (default: `True`).
- `show_hidden`: Show hidden files (default: `False`).
- `show_perm`: Show file permissions (default: `True`).
- `show_size`: Show file size (default: `False`).
- `height`: Maximum visible items (default: `10`).
- `show_help`: Show help key bindings (default: `True`).
- `timeout`: Timeout in seconds (default: `0` — no timeout).

Returns the selected file path as a string, or `None` if cancelled or timed
out.

Errors: an invalid `path` that cannot be resolved is reported as a script
error.

Example:

```python
load("gum", "file_pick", "spin")

selected_file = file_pick(
    title = "Select a configuration file:",
    allow_ext = ["json"],
    show_hidden = False,
)

if selected_file:
    def process_file():
        return "File processed successfully!"

    result = spin(
        title = "Processing " + selected_file,
        style = "dots",
        action = process_file,
    )
    print(result)
```

## Visual elements

### `note`

```text
note(title, description="", height=0, next="", show_help=True, timeout=0)
```

Displays a note with a title and description.

Parameters:

- `title`: Title text (required, must not be empty).
- `description`: Description text (default: `""`).
- `height`: Component height (default: `0` — the configured height/automatic).
- `next`: Text for the next button (default: `""` — no button).
- `show_help`: Show help key bindings (default: `True`).
- `timeout`: Timeout in seconds (default: `0` — no timeout).

Returns `None`.

Errors: `title` is required and cannot be empty.

### `md`

```text
md(text, style="auto", width=0, emoji=True, word_wrap=True)
```

Renders Markdown content into formatted terminal text. This builtin is
non-interactive and works in headless environments.

Parameters:

- `text`: Markdown text to render (required, must not be empty).
- `style`: Style to use for rendering (default: `"auto"`). Standard styles from
  the `glamour` package: `"auto"` (defaults to the dark theme — glamour v2 no
  longer probes the terminal), `"ascii"`, `"dark"`, `"dracula"`, `"light"`,
  `"notty"`, `"pink"`, `"tokyo-night"`. A path to a custom style JSON file is
  also accepted.
- `width`: Width to wrap text at (default: `0` — uses the configured `width`).
- `emoji`: Enable emoji support (default: `True`).
- `word_wrap`: Enable word wrapping (default: `True`).

Returns the rendered Markdown as a string with ANSI escape codes.

Errors: `text` is required and cannot be empty; a renderer/render failure is
reported as a script error.

Example:

```python
load("gum", "md")

rendered = md(
    text = "# Hello World\n\nThis is **bold** and *italic* text.",
    style = "dark",
)
print(rendered)
```

### `md_note`

```text
md_note(text, title="", style="auto", width=0, height=0, emoji=True,
        word_wrap=True, show_help=False, next="", timeout=0)
```

Renders Markdown content and displays it in a TUI note. Internally it renders
with `md` and then displays the result with `note`.

Parameters:

- `text`: Markdown text to render (required, must not be empty).
- `title`: Title for the note (default: `""`).
- `style`: Style to use for rendering (default: `"auto"`); same set as `md`.
- `width`: Width to wrap text at (default: `0` — uses the configured `width`).
- `height`: Height of the note (default: `0` — uses the configured height).
- `emoji`: Enable emoji support (default: `True`).
- `word_wrap`: Enable word wrapping (default: `True`).
- `show_help`: Show help text (default: `False`).
- `next`: Text for the next button (default: `""` — no button).
- `timeout`: Timeout in seconds (default: `0` — no timeout).

Returns `None`.

Errors: same as `md` and `note` (`text` must be non-empty; render failures
propagate).

### `spin`

```text
spin(title="Loading...", style="dots", action=None, timeout=1)
```

Displays a spinner, optionally running an action function while it spins.

Parameters:

- `title`: Spinner title (default: `"Loading..."`).
- `style`: Spinner style (default: `"dots"`). Available styles: `"line"`,
  `"dots"`, `"mini_dot"`, `"jump"`, `"points"`, `"pulse"`, `"globe"`, `"moon"`,
  `"monkey"`, `"meter"`, `"hamburger"`, `"ellipsis"` (common aliases such as
  `"dot"`, `"mini"`, `"earth"`, `"burger"` are also accepted).
- `action`: Function to execute while the spinner is active (default: `None`).
  When omitted, the spinner runs for `timeout` seconds.
- `timeout`: Timeout in seconds used only when no `action` is provided
  (default: `1`).

Returns the result of the `action` function, or `None` when there is no action.

Errors: an unsupported `style` is reported as a script error; an error raised
inside `action` propagates to the caller.

### `colorize`

```text
colorize(text, color="", pattern="CherryBlossoms", render="Column",
         from_color="", to_color="")
```

Colorizes text with a solid color, a built-in gradient pattern, or a custom
two-color gradient. This builtin is non-interactive and works in headless
environments.

Parameters:

- `text`: Text to colorize (required, must not be empty).
- `color`: Solid color (default: `""` — use a pattern). When set, `text` is
  rendered in this single color.
- `pattern`: Built-in gradient pattern (default: `"CherryBlossoms"`), used when
  neither `color` nor a `from_color`/`to_color` pair is given.
- `render`: Gradient direction, `"Column"` or `"Line"` (default: `"Column"`;
  case-insensitive, with aliases such as `"col"`/`"c"` and `"row"`/`"l"`/`"r"`).
- `from_color`: Custom gradient start color (default: `""`).
- `to_color`: Custom gradient end color (default: `""`). A custom gradient is
  used only when **both** `from_color` and `to_color` are set.

Returns the colorized text as a string.

Color resolution order: `color` (if set) → `from_color`/`to_color` gradient
(if both set) → `pattern`.

The `color`, `from_color`, and `to_color` arguments accept any case-insensitive
color description understood by the module's parser: a preset name (e.g. `red`,
`teal`, `lavender`), an `rgb(r, g, b)` triple, an `hsb(h, s, b)` triple, a
`#RRGGBB` hex code, or a `#RGB` short hex code.

Available patterns: `Almost`, `Anamnisar`, `AnimalCrossing`, `BrokenHearts`,
`CherryBlossoms`, `EveningNight`, `EveningSunshine`, `IbizaSunset`, `MiWatch`,
`Nelson`, `OceanSand`, `PurpleLove`, `PurpleParadise`, `RainbowBlue`,
`RelaxingRed`, `RoseWater`, `SublimeVivid`.

Errors: `text` is required and cannot be empty; an unknown `pattern` or an
invalid color string is reported as a script error.

Example:

```python
load("gum", "colorize")

# Built-in pattern.
print(colorize("Hello, Starlark!", pattern = "RainbowBlue"))

# Custom gradient.
print(colorize(
    text = "Hello, Starlark!",
    from_color = "#FF5733",  # orange-red
    to_color = "#33FF57",    # green
    render = "Column",
))
```

## Theming

### `set_theme`

```text
set_theme(theme)
```

Sets the active theme and re-applies it immediately, so subsequent components
render with the new theme within the same script run. This **overrides** the
auto-generated `set_theme` accessor from `base` (which would only update the
stored configuration value); see [Configuration](#configuration).

Parameters:

- `theme`: Theme name — one of `base`, `base16`, `charm`, `dracula`,
  `catppuccin` (any other value falls back to `charm`).

Returns `None`.

```python
load("gum", "set_theme", "select")

set_theme("dracula")
select(options = ["Red", "Green", "Blue"], title = "Pick a color:")
```

## Configuration

The `gum` module's configuration is provided by `starpkg/base`. Each option has
a default, can be set from its environment variable (uppercased, prefixed with
`GUM_`), or set programmatically — and every option is also reachable from a
script through an auto-generated accessor pair.

For each non-secret option, `base` generates a `get_<key>` builtin (returns the
current value) and a `set_<key>` builtin (takes a single value, returns
`None`). All four `gum` options are non-secret, so each exposes both accessors.

> **Secret options** would expose only a `set_<key>` builtin and no getter (the
> value is never readable from a script). The `gum` module has no secret
> options.

| Option | `get_` accessor | `set_` accessor | Env var | Default | Description |
|--------|-----------------|-----------------|---------|---------|-------------|
| `width` | `get_width` | `set_width` | `GUM_WIDTH` | `50` | Default width for TUI components (`0` = terminal width). |
| `height` | `get_height` | `set_height` | `GUM_HEIGHT` | `0` | Default height for components (`0` = automatic). |
| `theme` | `get_theme` | `set_theme` | `GUM_THEME` | `charm` | Theme name. **`set_theme` is overridden by `gum`** to re-apply the theme immediately (see above). |
| `editor` | `get_editor` | `set_editor` | `GUM_EDITOR` | `[]` | Default external editor command for `write` (e.g. `["vim", "-f"]`); empty falls back to `huh`. |

Available themes: `base` (minimal, monochrome), `base16` (simple 16-color),
`charm` (default), `dracula`, `catppuccin`.

Note: `set_theme` is the one accessor `gum` replaces with its own builtin so the
theme change takes effect immediately rather than only updating the stored
value; the other `set_*`/`get_*` accessors are the standard `base` ones.

### Configuring the module from Go

```go
// Method 1: default settings (width 50, height 0, theme "charm", no editor).
module := gum.NewModule()

// Method 2: custom settings —
// NewModuleWithConfig(width, height int, themeName string, editor []string).
module := gum.NewModuleWithConfig(
    80,               // width
    10,               // height
    "dracula",        // theme
    []string{"vim"},  // default editor command
)
```
