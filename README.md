# 🎨 `gum` - Starlark Module for Terminal User Interfaces

A powerful Starlark module for building Terminal User Interfaces (TUI), inspired by [charmbracelet/gum](https://github.com/charmbracelet/gum), [huh](https://github.com/charmbracelet/huh), and [bubbletea](https://github.com/charmbracelet/bubbletea). Create beautiful, interactive command-line interfaces in your Starlark scripts.

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
    // Create a new gum module with default settings
    gumModule := gum.NewModule()
    
    // Create a Starlet interpreter with the module
    interpreter := starlet.New(
        starlet.WithModuleLoader("gum", gumModule.LoadModule()),
    )
    
    // Run a Starlark script with TUI components
    script := `
load("gum", "input", "select", "confirm")

# Get user input
name = input(
    title = "What's your name?",
    placeholder = "Enter your name"
)

# Select a favorite color
color = select(
    options = ["Red", "Green", "Blue", "Yellow"],
    title = "Choose your favorite color:"
)

# Confirm the choices
confirmed = confirm(
    title = "Is this information correct?",
    description = "Name: " + name + "\nColor: " + color
)

print("Confirmed:", confirmed)
`
    
    // Execute the script
    if err := interpreter.ExecScript("example.star", script); err != nil {
        fmt.Println("Error:", err)
    }
}
```

## Configuration

The `gum` module has the following configuration options:

- `width`: Default width for TUI components (default: 50)
- `height`: Default height for components (default: 0 - automatic)
- `theme`: Theme name to use (default: "charm")

Available themes:

- `base`: Minimal, monochrome theme
- `base16`: Simple 16-color theme
- `charm`: Default Charm theme
- `dracula`: Dracula color scheme
- `catppuccin`: Catppuccin color scheme

### Module Configuration

```go
// Method 1: With default settings
module := gum.NewModule()

// Method 2: With custom settings
module := gum.NewModuleWithConfig(
    80,               // Width
    10,               // Height
    "dracula",        // Theme
)
```

## Starlark API

### Text Input Functions

#### `input(value?, prompt?, placeholder?, title?, description?, char_limit?, suggestions?, password?, validate?, width?, inline?, show_help?, timeout?)`

Creates a single-line text input field.

Parameters:

- `value`: Initial value (default: "")
- `prompt`: Input prompt (default: "> ")
- `placeholder`: Placeholder text (default: "Type something...")
- `title`: Title text (default: "")
- `description`: Description text (default: "")
- `char_limit`: Maximum character limit (default: 0 - no limit)
- `suggestions`: List of autocomplete suggestions (default: [])
- `password`: Boolean for password input or `None` for no echo (default: False)
- `validate`: Validation function (default: None)
- `width`: Component width (default: configured width)
- `inline`: Display in inline mode (default: False)
- `show_help`: Show help text (default: True)
- `timeout`: Timeout in seconds (default: 0 - no timeout)

Returns the entered text as a string.

#### `write(value?, placeholder?, title?, description?, char_limit?, validate?, width?, height?, show_line?, show_help?, timeout?)`

Creates a multi-line text area.

Parameters:

- `value`: Initial value (default: "")
- `placeholder`: Placeholder text (default: "Write something...")
- `title`: Title text (default: "")
- `description`: Description text (default: "")
- `char_limit`: Maximum character limit (default: 0 - no limit)
- `validate`: Validation function (default: None)
- `width`: Component width (default: configured width)
- `height`: Component height (default: 5)
- `show_line`: Show line numbers (default: False)
- `show_help`: Show help text (default: True)
- `timeout`: Timeout in seconds (default: 0 - no timeout)

Returns the entered text as a string.

### Selection Functions

#### `select(options, value?, title?, description?, validate?, width?, height?, inline?, show_filter?, show_help?, timeout?)`

Creates a single-selection component.

Parameters:

- `options`: List or dictionary of options
- `value`: Initial selected value (default: "")
- `title`: Title text (default: "Choose:")
- `description`: Description text (default: "")
- `validate`: Validation function (default: None)
- `width`: Component width (default: configured width)
- `height`: Maximum visible items (default: 0 - all)
- `inline`: Display in inline mode (default: False)
- `show_filter`: Enable filtering (default: False)
- `show_help`: Show help text (default: True)
- `timeout`: Timeout in seconds (default: 0 - no timeout)

Returns the selected value as a string.

#### `multi_select(options, value?, title?, description?, validate?, limit?, width?, height?, show_filter?, show_help?, timeout?)`

Creates a multi-selection component.

Parameters:

- `options`: List or dictionary of options
- `value`: List of initially selected values (default: [])
- `title`: Title text (default: "Choose:")
- `description`: Description text (default: "")
- `validate`: Validation function (default: None)
- `limit`: Maximum number of selections (default: 0 - no limit)
- `width`: Component width (default: configured width)
- `height`: Maximum visible items (default: 0 - all)
- `show_filter`: Enable filtering (default: False)
- `show_help`: Show help text (default: True)
- `timeout`: Timeout in seconds (default: 0 - no timeout)

Returns a list of selected values.

#### `confirm(value?, title?, description?, yes?, no?, inline?, show_help?, timeout?)`

Creates a yes/no confirmation dialog.

Parameters:

- `value`: Initial value (default: False)
- `title`: Title text (default: "Are you sure?")
- `description`: Description text (default: "")
- `yes`: Text for affirmative option (default: "Yes")
- `no`: Text for negative option (default: "No")
- `inline`: Display in inline mode (default: False)
- `show_help`: Show help text (default: True)
- `timeout`: Timeout in seconds (default: 0 - no timeout)

Returns a boolean value.

### File Picker

#### `file_pick(root?, glob?, hidden?, select_dirs?, multi?, title?, width?, height?, show_help?, timeout?)`

Creates a file picker component.

Parameters:

- `root`: Root directory (default: current directory)
- `glob`: File pattern to match (default: "*")
- `hidden`: Show hidden files (default: False)
- `select_dirs`: Allow directory selection (default: False)
- `multi`: Allow multiple selections (default: False)
- `title`: Title text (default: "Choose file:")
- `width`: Component width (default: configured width)
- `height`: Maximum visible items (default: configured height)
- `show_help`: Show help text (default: True)
- `timeout`: Timeout in seconds (default: 0 - no timeout)

Returns the selected file path(s).

### Visual Elements

#### `note(title, description?, height?, next?, show_help?, timeout?)`

Displays a note with a title and description.

Parameters:

- `title`: Title text (required)
- `description`: Description text (default: "")
- `height`: Component height (default: 0 - automatic)
- `next`: Text for next button (default: "" - no button)
- `show_help`: Show help text (default: True)
- `timeout`: Timeout in seconds (default: 0 - no timeout)

Returns None.

#### `render_md(text, title?, style?, width?, height?, emoji?, word_wrap?, show_help?, next?)`

Renders Markdown content into beautifully formatted terminal output.

Parameters:

- `text`: Markdown text to render (required)
- `title`: Title for the markdown display (default: "")
- `style`: Style to use for rendering (default: "auto")
  - Available styles: "auto", "dark", "light", "notty", or path to a custom style JSON file
  - "auto" will detect the terminal's background color
- `width`: Width to wrap the text at (default: 0 - uses module configuration)
- `height`: Height of the note component (default: 0 - uses module configuration)
- `emoji`: Enable emoji support (default: True)
- `word_wrap`: Enable word wrapping (default: True)
- `show_help`: Show help text (default: False)
- `next`: Text for next button (default: "" - no next button)

Displays rendered markdown as a note and returns None.

Example:

```starlark
load("gum", "render_md")

md_text = """
# Hello World

This is **bold** and *italic* text.

* List item 1
* List item 2

> A blockquote
"""
# Display markdown with a title
render_md(
    text = md_text,
    title = "Documentation Example",
    style = "dark",
    show_help = True,
    next = "Continue"
)
```

#### `spin(title?, style?, action?, timeout?)`

Displays a spinner with optional action function.

Parameters:

- `title`: Spinner title (default: "Loading...")
- `style`: Spinner style (default: "dots")
- `action`: Function to execute while spinner is active (default: None)
- `timeout`: Timeout in seconds when no action is provided (default: 1)

Returns the result of the action function or None.

Available spinner styles: "line", "dots", "mini_dot", "jump", "points", "pulse", "globe", "moon", "monkey", "meter", "hamburger", "ellipsis"

#### `colorize(text, color?, pattern?, render?)`

Colorizes text with gradients or solid colors.

Parameters:

- `text`: Text to colorize (required)
- `color`: Solid color name or hex code (default: "" - use pattern)
- `pattern`: Color pattern for gradient (default: "CherryBlossoms")
- `render`: Render type ("Column" or "Line") (default: "Column")

Returns the colorized text.

Available patterns: "Almost", "Anamnisar", "AnimalCrossing", "BrokenHearts", "CherryBlossoms", "EveningNight", "IbizaSunset", "MiWatch", "Nelson", "OceanSand", "PurpleLove", "RainbowBlue", "RoseWater"

## Examples

### Basic Input and Confirmation

```python
load("gum", "input", "confirm", "colorize")

# Get user input with validation
def validate_name(name):
    if len(name) < 3:
        return "Name must be at least 3 characters long"
    return None

name = input(
    title = "Welcome!",
    description = "Please enter your name",
    placeholder = "John Doe",
    validate = validate_name,
)

if name != None:
    # Colorize output
    welcome_text = colorize("Hello, " + name + "!", pattern="RainbowBlue")
    print(welcome_text)
    
    # Confirm action
    if confirm(
        title = "Would you like to continue?",
        description = "This will start the process",
        yes = "Let's go!",
        no = "Not now",
    ):
        print("Starting process...")
    else:
        print("Maybe next time!")
```

### Selection and Multi-selection

```python
load("gum", "select", "multi_select", "note")

# Single selection from a list
color = select(
    options = ["Red", "Green", "Blue", "Yellow", "Purple"],
    title = "Choose your favorite color:",
    description = "This will be used for your profile",
    show_filter = True,
)

# Multi-selection from a dictionary
selected_fruits = multi_select(
    options = {
        "apple": "Apple 🍎",
        "banana": "Banana 🍌",
        "orange": "Orange 🍊", 
        "grape": "Grape 🍇",
        "watermelon": "Watermelon 🍉"
    },
    title = "Select your favorite fruits:",
    limit = 3,
)

# Display results
if color and selected_fruits:
    fruits_str = ", ".join(selected_fruits)
    note(
        title = "Your Selections",
        description = "Color: " + color + "\nFruits: " + fruits_str,
        next = "Continue",
    )
```

### File Picker and Spinner

```python
load("gum", "file_pick", "spin")

# Pick a file
selected_file = file_pick(
    title = "Select a configuration file:",
    glob = "*.json",
)

if selected_file:
    # Function to process the file
    def process_file():
        # Simulate processing
        sleep(2)
        return "File processed successfully!"
    
    # Show spinner while processing
    result = spin(
        title = "Processing " + selected_file,
        style = "dots",
        action = process_file,
    )
    
    print(result)
```

## License

This package is licensed under the MIT License - see the LICENSE file for details.
