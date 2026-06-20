# 🎨 `gum` - Starlark Module for Terminal User Interfaces

[![godoc](https://pkg.go.dev/badge/github.com/starpkg/gum.svg)](https://pkg.go.dev/github.com/starpkg/gum)
[![license](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![codecov](https://codecov.io/gh/starpkg/gum/graph/badge.svg)](https://codecov.io/gh/starpkg/gum)
![binary footprint](https://img.shields.io/badge/binary_footprint-%2B10.1_MB-blue)

A powerful Starlark module for building Terminal User Interfaces (TUI), inspired by [charmbracelet/gum](https://github.com/charmbracelet/gum), [huh](https://github.com/charmbracelet/huh), and [bubbletea](https://github.com/charmbracelet/bubbletea). Create beautiful, interactive command-line interfaces in your Starlark scripts.

## Overview

Within the Star\* ecosystem, **starpkg provides support for necessary local operations plus simple abstractions over common online services, for ease of use.** `gum` is squarely a **local capability**: it drives the host's own terminal — prompts, selections, spinners, Markdown rendering, gradient text — and touches no network service. It is an L4 domain module that depends downward on `starpkg/base` (the module/config system) and `1set/starlet` (the Machine runner), and transitively `1set/starlight` + `go.starlark.net`.

## Features

- **Text Input**: Single and multi-line text inputs with validation
- **Selection**: Single and multi-option selection components
- **Confirmation**: Yes/No prompts with customizable text
- **File Picking**: Navigate and select files and directories
- **Visual Elements**: Spinners, notes, and colorized text output
- **Theming Support**: Multiple built-in themes (Charm, Dracula, Catppuccin, etc.)
- **Customization**: Configure width, height, timeouts, and more

## Installation

```bash
go get github.com/starpkg/gum
```

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/1set/starlet"
    "github.com/starpkg/gum"
)

func main() {
    // Create a new gum module with default settings.
    gumModule := gum.NewModule()

    // Create a Starlet interpreter with the module.
    interpreter := starlet.New(
        starlet.WithModuleLoader("gum", gumModule.LoadModule()),
    )

    // Run a Starlark script with TUI components.
    script := `
load("gum", "input", "select", "confirm")

name = input(title = "What's your name?", placeholder = "Enter your name")
color = select(options = ["Red", "Green", "Blue"], title = "Choose a color:")
ok = confirm(title = "Is this correct?", description = "Name: " + name + "\nColor: " + color)
print("Confirmed:", ok)
`

    // Execute the script.
    if err := interpreter.ExecScript("example.star", script); err != nil {
        fmt.Println("Error:", err)
    }
}
```

A self-contained script example:

```python
load("gum", "input", "confirm", "colorize")

def validate_name(name):
    if len(name) < 3:
        return "Name must be at least 3 characters long"
    return None

name = input(title = "Welcome!", placeholder = "John Doe", validate = validate_name)
if name != None:
    print(colorize("Hello, " + name + "!", pattern = "RainbowBlue"))
    if confirm(title = "Continue?", yes = "Let's go!", no = "Not now"):
        print("Starting process...")
```

## Starlark API at a glance

Load builtins with `load("gum", ...)`. Every builtin is listed below; see
**[`docs/API.md`](docs/API.md)** for the full reference — signatures,
parameters, returns, errors, and examples.

| Builtin | Summary |
|---------|---------|
| `input` | Single-line text input, with validation, suggestions, and password/echo modes. |
| `write` | Multi-line text area, with optional external editor support. |
| `select` | Single-selection from a list or dict of options. |
| `multi_select` | Multi-selection with an optional selection limit. |
| `filter` | Fuzzy-filter a list as you type (single or multi-select). |
| `confirm` | Yes/No confirmation dialog. |
| `file_pick` | File/directory picker with extension and visibility filters. |
| `note` | Display an informational note with a title and description. |
| `md` | Render Markdown to ANSI terminal text (non-interactive). |
| `md_note` | Render Markdown and display it in a TUI note. |
| `spin` | Show a spinner, optionally running an action while it spins. |
| `colorize` | Colorize text with a solid color or a gradient (non-interactive). |
| `code_block` | Syntax-highlight source code to ANSI via chroma (non-interactive). |
| `style` | Render styled text — colors, attributes, border, padding (non-interactive). |
| `table` | Render a bordered table from headers and rows (non-interactive). |
| `tree` | Render a nested tree from a dict/list (non-interactive). |
| `compose` | Join rendered blocks into a layout, horizontally or vertically (non-interactive). |
| `set_theme` | Set the active theme and re-apply it immediately. |

> **TTY note.** The interactive builtins drive the host's controlling terminal
> and fail with `could not open a new TTY` in headless environments (CI,
> sandboxes). `md`, `colorize`, `code_block`, `style`, `table`, `tree`, and
> `compose` are non-interactive and run anywhere.

## Configuration

The `gum` module is configured through `starpkg/base`: options `width`,
`height`, `theme`, and `editor`, each with an environment variable
(`GUM_<KEY>`) and an auto-generated `get_<key>` / `set_<key>` script accessor
pair. See **[`docs/API.md` → Configuration](docs/API.md#configuration)** for the
full table and the Go constructors (`NewModule`, `NewModuleWithConfig`).

## License

This package is licensed under the MIT License - see the LICENSE file for details.
