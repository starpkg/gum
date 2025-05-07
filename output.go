package gum

import (
	"fmt"
	"strings"
	"time"

	"bitbucket.org/ai69/colorlogo"
	"github.com/1set/starlet/dataconv/types"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"go.starlark.net/starlark"
)

// starNote is a Starlark function to create a TUI note for showing information to the user.
// def note(title: str, description: str = "", height: int = 0, next: str = "", show_help: bool = True, timeout: float = 0) -> None
func (m *Module) starNote(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		title       = types.StringOrBytes("")            // title text
		description = types.NewNullableStringOrBytes("") // description text
		height      = 0                                  // maximum number of items to show (0 for all)
		wordNext    = types.NewNullableStringOrBytes("") // next word
		showHelp    = true                               // show help key binds
		timeoutSec  = types.FloatOrInt(0)                // timeout in seconds (0 for no timeout)
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"title", &title,
		"description?", description,
		"height?", &height,
		"next?", wordNext,
		"show_help?", &showHelp,
		"timeout?", &timeoutSec,
	); err != nil {
		return none, err
	}

	// Get text content
	if title.IsEmpty() {
		return none, fmt.Errorf("title is required and cannot be empty")
	}

	// next button
	hasNext := !wordNext.IsNullOrEmpty()
	strNext := wordNext.GoString()

	// run note
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title(title.GoString()).
				Description(description.GoString()).
				Height(m.getHeight(height)).
				Next(hasNext).
				NextLabel(strNext),
		),
	).
		WithTheme(m.theme).
		WithKeyMap(m.keymap).
		WithShowHelp(showHelp).
		WithTimeout(convertDuration(timeoutSec)).
		Run()

	// handle no result
	if err != nil {
		if ignorableError(err) {
			return none, nil
		}
		return none, err
	}
	return none, nil
}

var spinStyleMap = map[string]spinner.Type{
	"line": spinner.Line,
	"dots": spinner.Dots, "dot": spinner.Dots,
	"mini_dot": spinner.MiniDot, "minidot": spinner.MiniDot, "mini": spinner.MiniDot,
	"jump":   spinner.Jump,
	"points": spinner.Points, "point": spinner.Points,
	"pulse": spinner.Pulse,
	"globe": spinner.Globe, "earth": spinner.Globe,
	"moon":      spinner.Moon,
	"monkey":    spinner.Monkey,
	"meter":     spinner.Meter,
	"hamburger": spinner.Hamburger, "burger": spinner.Hamburger,
	"ellipsis": spinner.Ellipsis,
}

// starSpinner is a Starlark function to show a spinner with an optional action.
// def spin(title: str = "Loading", style: str = "dots", action: Callable = None, timeout: float = 1) -> Any
func (m *Module) starSpinner(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		title      = types.NewNullableStringOrBytes("Loading...") // title text
		style      = types.NewNullableStringOrBytes("dots")       // spinner style
		actionFunc types.NullableCallable                         // action function
		timeoutSec = types.FloatOrInt(1)                          // timeout in seconds, it won't be used if action is set
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"title?", title,
		"style?", style,
		"action?", &actionFunc,
		"timeout?", &timeoutSec,
	); err != nil {
		return none, err
	}

	// convert spinner style
	st, ok := spinStyleMap[strings.ToLower(style.GoString())]
	if !ok {
		return none, fmt.Errorf("unsupported spinner style: %s", style.GoString())
	}

	// action function
	var (
		actRes  starlark.Value = none
		actErr  error
		actFunc = func() {
			if actionFunc.IsNull() {
				// default action: sleep for timeout
				time.Sleep(convertDuration(timeoutSec))
			} else {
				// custom action: call and pass through the result and error
				nt := &starlark.Thread{Name: "spin", Load: thread.Load, Print: thread.Print, OnMaxSteps: thread.OnMaxSteps}
				actRes, actErr = starlark.Call(nt, actionFunc.Value(), nil, nil)
			}
		}
	)

	// run spinner and action
	ts := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#00020A", Dark: "#FFFDF5"})
	err := spinner.New().
		Title(title.GoString()).
		TitleStyle(ts).
		Type(st).
		Action(actFunc).
		Run()

	// handle no result
	if err != nil {
		if ignorableError(err) {
			return none, nil
		}
		return none, err
	}

	// return action result or default
	return actRes, actErr
}

var colorFuncMap = map[string]func(string) string{
	"almost|column":         colorlogo.AlmostByColumn,
	"almost|line":           colorlogo.AlmostByLine,
	"anamnisar|column":      colorlogo.AnamnisarByColumn,
	"anamnisar|line":        colorlogo.AnamnisarByLine,
	"animalcrossing|column": colorlogo.AnimalCrossingByColumn,
	"animalcrossing|line":   colorlogo.AnimalCrossingByLine,
	"brokenhearts|column":   colorlogo.BrokenHeartsByColumn,
	"brokenhearts|line":     colorlogo.BrokenHeartsByLine,
	"cherryblossoms|column": colorlogo.CherryBlossomsByColumn,
	"cherryblossoms|line":   colorlogo.CherryBlossomsByLine,
	"eveningnight|column":   colorlogo.EveningNightByColumn,
	"eveningnight|line":     colorlogo.EveningNightByLine,
	"ibizasunset|column":    colorlogo.IbizaSunsetByColumn,
	"ibizasunset|line":      colorlogo.IbizaSunsetByLine,
	"miwatch|column":        colorlogo.MiWatchByColumn,
	"miwatch|line":          colorlogo.MiWatchByLine,
	"nelson|column":         colorlogo.NelsonByColumn,
	"nelson|line":           colorlogo.NelsonByLine,
	"oceansand|column":      colorlogo.OceanSandByColumn,
	"oceansand|line":        colorlogo.OceanSandByLine,
	"purplelove|column":     colorlogo.PurpleLoveByColumn,
	"purplelove|line":       colorlogo.PurpleLoveByLine,
	"rainbowblue|column":    colorlogo.RainbowBlueByColumn,
	"rainbowblue|line":      colorlogo.RainbowBlueByLine,
	"rosewater|column":      colorlogo.RoseWaterByColumn,
	"rosewater|line":        colorlogo.RoseWaterByLine,
}

// starColorize is a Starlark function to colorize a string.
// def colorize(text: str, color: str = "", pattern: str = "CherryBlossoms", render: str = "Column", from_color: str = "", to_color: str = "") -> str
func (m *Module) starColorize(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		text      = types.StringOrBytes("")                          // text to colorize
		colorName = types.NewNullableStringOrBytes("")               // color name
		pattern   = types.NewNullableStringOrBytes("CherryBlossoms") // color pattern
		render    = types.NewNullableStringOrBytes("Column")         // render type
		fromColor = types.NewNullableStringOrBytes("")               // from color for custom gradient
		toColor   = types.NewNullableStringOrBytes("")               // to color for custom gradient
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"text", &text,
		"color?", colorName,
		"pattern?", pattern,
		"render?", render,
		"from_color?", fromColor,
		"to_color?", toColor,
	); err != nil {
		return none, err
	}

	// Get text content
	if text.IsEmpty() {
		return none, fmt.Errorf("text is required and cannot be empty")
	}
	textStr := text.GoString()

	// if color is set, use it
	if !colorName.IsNullOrEmpty() {
		colorStr := colorName.GoString()
		rc, err := ParseColor(colorStr)
		if err != nil {
			return none, err
		}
		tc := termenv.ColorProfile().FromColor(rc)
		styled := termenv.String(textStr).Foreground(tc).String()
		return starlark.String(styled), nil
	}

	// if from_color and to_color are set, use custom gradient
	if !fromColor.IsNullOrEmpty() && !toColor.IsNullOrEmpty() {
		fromColorStr := fromColor.GoString()
		toColorStr := toColor.GoString()
		renderStr := render.GoString()

		// Parse the colors to ensure they're in the correct format
		fromRGB, err := ParseColor(fromColorStr)
		if err != nil {
			return none, fmt.Errorf("invalid from_color: %w", err)
		}
		fromHex := colorToHex(fromRGB)

		toRGB, err := ParseColor(toColorStr)
		if err != nil {
			return none, fmt.Errorf("invalid to_color: %w", err)
		}
		toHex := colorToHex(toRGB)

		var result string
		switch normalizeRenderType(renderStr) {
		case "column":
			result, err = colorlogo.GradientByColumn(textStr, fromHex, toHex)
		case "line":
			result, err = colorlogo.GradientByLine(textStr, fromHex, toHex)
		default:
			return none, fmt.Errorf("unsupported render type: %s", renderStr)
		}

		if err != nil {
			return none, err
		}
		return starlark.String(result), nil
	}

	// otherwise, use pattern
	patternStr := pattern.GoString()
	renderStr := render.GoString()
	normalized := normalizePattern(patternStr) + "|" + normalizeRenderType(renderStr)
	colorFunc, ok := colorFuncMap[strings.ToLower(normalized)]
	if !ok {
		return none, fmt.Errorf("unsupported pattern: %s", patternStr)
	}
	result := colorFunc(textStr)
	return starlark.String(result), nil
}

// normalizePattern normalizes the pattern name for use as a key in colorFuncMap.
func normalizePattern(s string) string {
	return strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(s, " ", ""), "-", ""))
}

// normalizeRenderType normalizes the render type for use as a key in colorFuncMap.
func normalizeRenderType(s string) string {
	switch strings.ToLower(s) {
	case "column", "col", "c":
		return "column"
	case "line", "l", "row", "r":
		return "line"
	default:
		return s
	}
}
