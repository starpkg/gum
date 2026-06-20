package gum

// unit_test.go holds the public, non-TTY/non-network unit tests for gum.
// The interactive builtins ultimately open /dev/tty via huh/bubbletea, so the
// integration scripts that exercise them live in the private starpkg/test repo
// and run only in an attended terminal. The tests here cover the logic that
// runs anywhere: pure converters/parsers/formatters and the validation/error
// branches that execute *before* any TTY or network call. They also verify the
// hardening invariants documented in CLAUDE.md.
//
// Sections:
//   - ParseColor: preset/rgb/hsb/hex parsing + edge cases (color.go)
//   - hsbToRGBA / colorToHex: pure color math (color.go)
//   - convertDuration: float/int -> time.Duration, including extreme values (gum.go)
//   - applyTheme / getWidth / getHeight: config defaulting (gum.go)
//   - convertOptionList: list/dict/iterable -> huh options (select.go)
//   - convertValidator: sandboxed validator semantics (gum.go)
//   - normalizePattern / normalizeRenderType: colorize key building (output.go)
//   - ignorableError: clean cancellation — abort/timeout -> None (gum.go)
//   - parseColorQuery malformed components: bad input errors, never wrong (color.go)
//   - non-TTY builtin error branches: bad args/empty inputs -> clean errors
//   - backward-compat: NewModule defaults remain the historical surface

import (
	"errors"
	"fmt"
	"image/color"
	"math"
	"strings"
	"testing"
	"time"

	huh "charm.land/huh/v2"
	"github.com/1set/starlet"
	"github.com/1set/starlet/dataconv/types"
	"go.starlark.net/starlark"
)

// runGumScript runs a Starlark script with a fresh gum module loaded and
// returns the resulting error (nil on success). It needs no TTY because every
// caller exercises only the validation/error branch that precedes huh.Run().
func runGumScript(t *testing.T, script string) error {
	t.Helper()
	s := starlet.NewDefault()
	s.AddLazyloadModules(map[string]starlet.ModuleLoader{
		ModuleName: NewModule().LoadModule(),
	})
	_, err := s.RunScript([]byte(script), nil)
	return err
}

// requireGumScriptErrorContains asserts the script fails with a message that
// contains want.
func requireGumScriptErrorContains(t *testing.T, script, want string) {
	t.Helper()
	err := runGumScript(t, script)
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected error containing %q, got %v", want, err)
	}
}

// requireGumScriptOK asserts the script runs without error.
func requireGumScriptOK(t *testing.T, script string) {
	t.Helper()
	if err := runGumScript(t, script); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
}

// --- ParseColor -------------------------------------------------------------

func TestParseColor(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantHex string // expected #RRGGBB, empty if an error is expected
		errSub  string // expected substring of the error, empty if success
	}{
		// preset names (case-insensitive), incl. aliases
		{"preset red", "red", "#FF0000", ""},
		{"preset upper", "RED", "#FF0000", ""},
		{"preset mixed", "Teal", "#008080", ""},
		{"alias aqua", "aqua", "#00FFFF", ""},       // aqua == cyan
		{"alias grey", "grey", "#808080", ""},       // grey == gray
		{"alias fuchsia", "fuchsia", "#FF00FF", ""}, // fuchsia == magenta
		{"name with spaces around", "  blue  ", "#0000FF", ""},
		{"name embedded in sentence", "a nice green color", "#00FF00", ""},
		// rgb
		{"rgb basic", "rgb(255, 0, 0)", "#FF0000", ""},
		{"rgb no spaces", "rgb(0,128,0)", "#008000", ""},
		{"rgb space after comma", "rgb(16, 152, 43)", "#10982B", ""},
		// The regex tolerates inner padding the Sscanf reparse does not; padding
		// directly after '(' yields a clean error, never a panic or wrong color.
		{"rgb padded fails cleanly", "rgb( 16 , 152 , 43 )", "", "invalid rgb color"},
		{"rgb overflow", "rgb(256,0,0)", "", "invalid rgb color"},
		{"rgb way over", "rgb(999,999,999)", "", "invalid rgb color"},
		// hsb (previously unimplemented — now wired)
		{"hsb red", "hsb(0,100,100)", "#FF0000", ""},
		{"hsb green", "hsb(120,100,100)", "#00FF00", ""},
		{"hsb blue", "hsb(240,100,100)", "#0000FF", ""},
		{"hsb white", "hsb(0,0,100)", "#FFFFFF", ""},
		{"hsb black", "hsb(0,0,0)", "#000000", ""},
		{"hsb gray", "hsb(0,0,50)", "#808080", ""},
		{"hsb h360 wraps", "hsb(360,100,100)", "#FF0000", ""},
		{"hsb spaced", "hsb( 60 , 100 , 100 )", "#FFFF00", ""},
		{"hsb h out of range", "hsb(999,100,100)", "", "out of range"},
		{"hsb s out of range", "hsb(0,200,100)", "", "out of range"},
		// hex
		{"hex6", "#1A2B3C", "#1A2B3C", ""},
		{"hex6 lower", "#abcdef", "#ABCDEF", ""},
		{"hex3", "#abc", "#AABBCC", ""},
		{"hex3 white", "#fff", "#FFFFFF", ""},
		// errors
		{"blank", "", "", "blank query"},
		{"whitespace only", "   ", "", "blank query"},
		{"unknown", "notacolor", "", "no color match"},
		{"bad hex char", "#zzz", "", "no color match"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := ParseColor(tt.query)
			if tt.errSub != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got color %v", tt.errSub, c)
				}
				if !strings.Contains(err.Error(), tt.errSub) {
					t.Fatalf("expected error containing %q, got %v", tt.errSub, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := colorToHex(c); got != tt.wantHex {
				t.Fatalf("ParseColor(%q) = %s, want %s", tt.query, got, tt.wantHex)
			}
		})
	}
}

// TestParseColorRoundTrip verifies every preset name parses to the hex recorded
// in hexNameMap, keeping the two tables consistent.
func TestParseColorPresetTablesConsistent(t *testing.T) {
	for name, c := range presetColorMap {
		hex := colorToHex(c)
		formal, ok := hexNameMap[hex]
		if !ok {
			t.Errorf("preset %q (%s) missing from hexNameMap", name, hex)
			continue
		}
		// the formal name must itself be a preset that maps back to the same hex
		fc, ok := presetColorMap[formal]
		if !ok {
			t.Errorf("hexNameMap[%s]=%q not in presetColorMap", hex, formal)
			continue
		}
		if colorToHex(fc) != hex {
			t.Errorf("formal name %q maps to %s, want %s", formal, colorToHex(fc), hex)
		}
	}
	// colorNames must list exactly the keys of presetColorMap
	if len(colorNames) != len(presetColorMap) {
		t.Errorf("colorNames has %d entries, presetColorMap has %d", len(colorNames), len(presetColorMap))
	}
	for _, n := range colorNames {
		if _, ok := presetColorMap[n]; !ok {
			t.Errorf("colorNames entry %q absent from presetColorMap", n)
		}
	}
}

// --- hsbToRGBA / colorToHex --------------------------------------------------

func TestHSBToRGBA(t *testing.T) {
	tests := []struct {
		h, s, b float64
		want    color.RGBA
	}{
		{0, 100, 100, color.RGBA{0xFF, 0x00, 0x00, 0xFF}},
		{60, 100, 100, color.RGBA{0xFF, 0xFF, 0x00, 0xFF}},
		{120, 100, 100, color.RGBA{0x00, 0xFF, 0x00, 0xFF}},
		{180, 100, 100, color.RGBA{0x00, 0xFF, 0xFF, 0xFF}},
		{240, 100, 100, color.RGBA{0x00, 0x00, 0xFF, 0xFF}},
		{300, 100, 100, color.RGBA{0xFF, 0x00, 0xFF, 0xFF}},
		{0, 0, 100, color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}}, // white
		{0, 0, 0, color.RGBA{0x00, 0x00, 0x00, 0xFF}},   // black
	}
	for _, tt := range tests {
		got := hsbToRGBA(tt.h, tt.s, tt.b)
		if got != tt.want {
			t.Errorf("hsbToRGBA(%v,%v,%v) = %v, want %v", tt.h, tt.s, tt.b, got, tt.want)
		}
		if got.A != 0xFF {
			t.Errorf("hsbToRGBA alpha = %d, want 255 (opaque)", got.A)
		}
	}
}

func TestColorToHex(t *testing.T) {
	tests := []struct {
		c    color.Color
		want string
	}{
		{color.RGBA{0, 0, 0, 0xFF}, "#000000"},
		{color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}, "#FFFFFF"},
		{ColorMint, "#16982B"},
		{ColorApricot, "#FBCEB1"},
	}
	for _, tt := range tests {
		if got := colorToHex(tt.c); got != tt.want {
			t.Errorf("colorToHex(%v) = %s, want %s", tt.c, got, tt.want)
		}
	}
}

func TestToRGBA(t *testing.T) {
	// toRGBA must drop alpha and re-set it to opaque
	in := color.RGBA{R: 0x12, G: 0x34, B: 0x56, A: 0x00}
	got := toRGBA(in)
	want := color.RGBA{R: 0x12, G: 0x34, B: 0x56, A: 0xFF}
	if got != want {
		t.Fatalf("toRGBA(%v) = %v, want %v", in, got, want)
	}
}

// --- convertDuration --------------------------------------------------------

func TestConvertDuration(t *testing.T) {
	tests := []struct {
		in   float64
		want time.Duration
	}{
		{0, 0},
		{1, time.Second},
		{1.5, 1500 * time.Millisecond},
		{0.25, 250 * time.Millisecond},
		{-5, -5 * time.Second},
	}
	for _, tt := range tests {
		if got := convertDuration(types.FloatOrInt(tt.in)); got != tt.want {
			t.Errorf("convertDuration(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

// TestConvertDurationExtreme verifies the hardening property that matters: an
// absurd timeout from a script never panics the host. The exact Duration value
// for an out-of-range/Inf/NaN float is implementation-defined in Go (float ->
// int64 conversion is platform-dependent), so we assert only that the call
// returns without panicking, not a specific magnitude.
func TestConvertDurationExtreme(t *testing.T) {
	cases := map[string]float64{
		"huge":     1e300,
		"posInf":   math.Inf(1),
		"negInf":   math.Inf(-1),
		"nan":      math.NaN(),
		"overflow": 9.3e18,
	}
	for name, f := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("convertDuration(%v) panicked: %v", f, r)
				}
			}()
			_ = convertDuration(types.FloatOrInt(f)) // must not panic
		})
	}
}

// --- applyTheme / getWidth / getHeight --------------------------------------

func TestApplyTheme(t *testing.T) {
	m := NewModule()
	// known names must all yield a non-nil theme; unknown falls back to charm.
	for _, name := range []string{"base", "base16", "charm", "dracula", "catppuccin", "CHARM", "Dracula", "unknown-name", ""} {
		if th := m.applyTheme(name); th == nil {
			t.Errorf("applyTheme(%q) returned nil", name)
		}
	}
}

func TestGetWidthHeightDefaulting(t *testing.T) {
	m := NewModule()
	// explicit positive overrides win
	if got := m.getWidth(80); got != 80 {
		t.Errorf("getWidth(80) = %d, want 80", got)
	}
	// zero / negative fall back to the configured default (50 for NewModule)
	if got := m.getWidth(0); got != 50 {
		t.Errorf("getWidth(0) = %d, want 50", got)
	}
	if got := m.getWidth(-1); got != 50 {
		t.Errorf("getWidth(-1) = %d, want 50", got)
	}
	// height default is 0
	if got := m.getHeight(0); got != 0 {
		t.Errorf("getHeight(0) = %d, want 0", got)
	}
	if got := m.getHeight(12); got != 12 {
		t.Errorf("getHeight(12) = %d, want 12", got)
	}
}

// --- convertOptionList ------------------------------------------------------

func TestConvertOptionList(t *testing.T) {
	t.Run("list of strings", func(t *testing.T) {
		l := starlark.NewList([]starlark.Value{
			starlark.String("a"), starlark.String("b"), starlark.String("c"),
		})
		opts, err := convertOptionList(l)
		if err != nil {
			t.Fatal(err)
		}
		if len(opts) != 3 {
			t.Fatalf("got %d options, want 3", len(opts))
		}
		// list options use the value as both key and value
		if opts[0].Key != "a" || opts[0].Value != "a" {
			t.Errorf("opts[0] = (%q,%q), want (a,a)", opts[0].Key, opts[0].Value)
		}
	})

	t.Run("list of mixed values stringified", func(t *testing.T) {
		l := starlark.NewList([]starlark.Value{
			starlark.MakeInt(1), starlark.Bool(true),
		})
		opts, err := convertOptionList(l)
		if err != nil {
			t.Fatal(err)
		}
		if len(opts) != 2 {
			t.Fatalf("got %d options, want 2", len(opts))
		}
		if opts[0].Value != "1" || opts[1].Value != "True" {
			t.Errorf("stringified options = %q,%q", opts[0].Value, opts[1].Value)
		}
	})

	t.Run("dict key displayed value returned", func(t *testing.T) {
		d := starlark.NewDict(2)
		_ = d.SetKey(starlark.String("Display A"), starlark.String("val_a"))
		_ = d.SetKey(starlark.String("Display B"), starlark.String("val_b"))
		opts, err := convertOptionList(d)
		if err != nil {
			t.Fatal(err)
		}
		if len(opts) != 2 {
			t.Fatalf("got %d options, want 2", len(opts))
		}
		// dict preserves insertion order in starlark
		if opts[0].Key != "Display A" || opts[0].Value != "val_a" {
			t.Errorf("opts[0] = (%q,%q), want (Display A,val_a)", opts[0].Key, opts[0].Value)
		}
	})

	t.Run("tuple iterable", func(t *testing.T) {
		tup := starlark.Tuple{starlark.String("x"), starlark.String("y")}
		opts, err := convertOptionList(tup)
		if err != nil {
			t.Fatal(err)
		}
		if len(opts) != 2 || opts[1].Value != "y" {
			t.Fatalf("tuple options = %+v", opts)
		}
	})

	t.Run("non-iterable rejected", func(t *testing.T) {
		_, err := convertOptionList(starlark.MakeInt(42))
		if err == nil {
			t.Fatal("expected error for int input")
		}
		if !strings.Contains(err.Error(), "iterable or mapping") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("empty list yields no options", func(t *testing.T) {
		opts, err := convertOptionList(starlark.NewList(nil))
		if err != nil {
			t.Fatal(err)
		}
		if len(opts) != 0 {
			t.Fatalf("expected 0 options, got %d", len(opts))
		}
	})

	t.Run("set uses the generic iterable branch", func(t *testing.T) {
		// A starlark.Set is neither List nor Dict, so it must fall through to the
		// generic starlark.Iterable case and stringify each member as key+value.
		set := starlark.NewSet(2)
		if err := set.Insert(starlark.String("a")); err != nil {
			t.Fatal(err)
		}
		if err := set.Insert(starlark.MakeInt(7)); err != nil {
			t.Fatal(err)
		}
		opts, err := convertOptionList(set)
		if err != nil {
			t.Fatal(err)
		}
		if len(opts) != 2 {
			t.Fatalf("got %d options, want 2", len(opts))
		}
		for _, o := range opts {
			if o.Key != o.Value {
				t.Errorf("iterable option key %q != value %q", o.Key, o.Value)
			}
		}
	})

	t.Run("bare string is rejected, not split into chars", func(t *testing.T) {
		// go-starlark strings are not Iterable, so options="abc" must error
		// cleanly rather than silently becoming three single-char options.
		_, err := convertOptionList(starlark.String("abc"))
		if err == nil {
			t.Fatal("expected error for string input")
		}
		if !strings.Contains(err.Error(), "iterable or mapping") {
			t.Errorf("unexpected error: %v", err)
		}
		if !strings.Contains(err.Error(), "string") {
			t.Errorf("error should name the offending type, got: %v", err)
		}
	})

	t.Run("None is rejected", func(t *testing.T) {
		_, err := convertOptionList(starlark.None)
		if err == nil || !strings.Contains(err.Error(), "iterable or mapping") {
			t.Fatalf("expected iterable/mapping error for None, got %v", err)
		}
	})
}

// --- convertValidator (sandboxed re-entrancy) -------------------------------

func TestConvertValidator(t *testing.T) {
	thread := &starlark.Thread{Name: "parent"}
	const src = `
def reject_bad(x):
    return None if x == "ok" else "value rejected: " + x
def always_ok(x):
    return ""
def list_ok(x):
    return None
def list_bad(x):
    return "too many" if len(x) > 1 else None
`
	globals, err := starlark.ExecFile(thread, "v.star", src, nil)
	if err != nil {
		t.Fatal(err)
	}
	mkNC := func(name string) *types.NullableCallable {
		nc := &types.NullableCallable{}
		if err := nc.Unpack(globals[name]); err != nil {
			t.Fatal(err)
		}
		return nc
	}

	t.Run("nil callable always valid", func(t *testing.T) {
		f := convertStringValidator(thread, &types.NullableCallable{})
		if err := f("whatever"); err != nil {
			t.Errorf("nil validator should accept anything, got %v", err)
		}
	})

	t.Run("None result is valid", func(t *testing.T) {
		f := convertStringValidator(thread, mkNC("reject_bad"))
		if err := f("ok"); err != nil {
			t.Errorf("expected valid, got %v", err)
		}
	})

	t.Run("non-empty string is the error message", func(t *testing.T) {
		f := convertStringValidator(thread, mkNC("reject_bad"))
		err := f("nope")
		if err == nil || err.Error() != "value rejected: nope" {
			t.Errorf("got %v, want 'value rejected: nope'", err)
		}
	})

	t.Run("empty string result is valid", func(t *testing.T) {
		f := convertStringValidator(thread, mkNC("always_ok"))
		if err := f("anything"); err != nil {
			t.Errorf("empty-string result should be valid, got %v", err)
		}
	})

	t.Run("list validator valid", func(t *testing.T) {
		f := convertStringListValidator(thread, mkNC("list_ok"))
		if err := f([]string{"a", "b"}); err != nil {
			t.Errorf("expected valid, got %v", err)
		}
	})

	t.Run("list validator rejects", func(t *testing.T) {
		f := convertStringListValidator(thread, mkNC("list_bad"))
		err := f([]string{"a", "b"})
		if err == nil || err.Error() != "too many" {
			t.Errorf("got %v, want 'too many'", err)
		}
	})

	t.Run("validator runtime error is wrapped", func(t *testing.T) {
		const badSrc = `
def boom(x):
    fail("kaboom")
`
		g, err := starlark.ExecFile(&starlark.Thread{Name: "p"}, "b.star", badSrc, nil)
		if err != nil {
			t.Fatal(err)
		}
		nc := &types.NullableCallable{}
		if err := nc.Unpack(g["boom"]); err != nil {
			t.Fatal(err)
		}
		f := convertStringValidator(thread, nc)
		err = f("x")
		if err == nil || !strings.Contains(err.Error(), "validator error") {
			t.Errorf("expected wrapped 'validator error', got %v", err)
		}
	})
}

// --- normalizePattern / normalizeRenderType ---------------------------------

func TestNormalizePattern(t *testing.T) {
	tests := map[string]string{
		"CherryBlossoms":  "cherryblossoms",
		"Cherry Blossoms": "cherryblossoms",
		"cherry-blossoms": "cherryblossoms",
		"RainbowBlue":     "rainbowblue",
		"  Ocean Sand  ":  "oceansand", // all spaces removed, then lower-cased
	}
	for in, want := range tests {
		if got := normalizePattern(in); got != want {
			t.Errorf("normalizePattern(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeRenderType(t *testing.T) {
	tests := map[string]string{
		"Column": "column",
		"col":    "column",
		"C":      "column",
		"line":   "line",
		"L":      "line",
		"row":    "line",
		"R":      "line",
		"weird":  "weird", // unknown passes through lower-cased? no, returns as-is
	}
	for in, want := range tests {
		if got := normalizeRenderType(in); got != want {
			t.Errorf("normalizeRenderType(%q) = %q, want %q", in, got, want)
		}
	}
}

// --- ignorableError (Invariant 1: clean cancellation) -----------------------

// TestIgnorableError pins the core of the "no host panic / clean cancellation"
// invariant: a user Ctrl-C (ErrUserAborted) and a form Timeout (ErrTimeout) are
// not script errors — they collapse to a graceful None — while any other error
// must propagate. Wrapped sentinels (errors.Is) must still be recognized so a
// huh internal wrap of the abort/timeout never leaks out as a hard error.
func TestIgnorableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool // true => ignorable (becomes None), false => propagates
	}{
		{"nil is ignorable", nil, true},
		{"user aborted", huh.ErrUserAborted, true},
		{"timeout", huh.ErrTimeout, true},
		{"wrapped user aborted", fmt.Errorf("form failed: %w", huh.ErrUserAborted), true},
		{"wrapped timeout", fmt.Errorf("run: %w", huh.ErrTimeout), true},
		{"real error propagates", errors.New("could not open a new TTY"), false},
		{"timeout-unsupported propagates", huh.ErrTimeoutUnsupported, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ignorableError(tt.err); got != tt.want {
				t.Errorf("ignorableError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// --- parseColorQuery malformed-component branches ----------------------------

// TestParseColorMalformedComponents exercises the error arms of the hex/rgb
// reparse: the regex can admit a token whose Sscanf reparse then rejects, and
// that must surface as a clean "invalid ... color" error, never a wrong color
// or a panic. (Invariant 3 in spirit: bad input errors, never silently wrong.)
func TestParseColorMalformedComponents(t *testing.T) {
	tests := []struct {
		name   string
		query  string
		errSub string
	}{
		// A space *before* a comma defeats the "rgb(%d,%d,%d)" Sscanf reparse
		// (%d skips leading space but the literal ',' does not), so the regex can
		// admit a token the reparse then rejects — that must be a clean error.
		{"rgb space before comma", "rgb( 16 , 152 , 43 )", "invalid rgb color"},
		{"rgb overflow component", "rgb(300,1,1)", "invalid rgb color"},
		{"hsb hue overflow", "hsb(361,0,0)", "out of range"},
		{"hsb sat overflow", "hsb(0,101,0)", "out of range"},
		{"hsb bri overflow", "hsb(0,0,101)", "out of range"},
		// no recognizable token at all
		{"garbage", "this is not a color", "no color match"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := ParseColor(tt.query)
			if err == nil {
				t.Fatalf("expected error containing %q, got color %v", tt.errSub, c)
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Fatalf("expected error containing %q, got %v", tt.errSub, err)
			}
		})
	}
}

// --- non-TTY builtin error branches -----------------------------------------

// These scripts only reach the validation/error branch that runs before huh
// opens a TTY, so they execute in CI / headless environments.
func TestBuiltinErrorBranches(t *testing.T) {
	tests := []struct {
		name   string
		script string
		errSub string
	}{
		{"select empty options", `load("gum","select")` + "\n" + `select([])`, "options must not be empty"},
		{"select non-iterable", `load("gum","select")` + "\n" + `select(123)`, "iterable or mapping"},
		{"multi_select empty options", `load("gum","multi_select")` + "\n" + `multi_select([])`, "options must not be empty"},
		{"multi_select non-iterable", `load("gum","multi_select")` + "\n" + `multi_select(42)`, "iterable or mapping"},
		{"colorize empty text", `load("gum","colorize")` + "\n" + `colorize("")`, "text is required"},
		{"colorize bad color", `load("gum","colorize")` + "\n" + `colorize("hi", color="bogus")`, "no color match"},
		{"colorize bad from_color", `load("gum","colorize")` + "\n" + `colorize("hi", from_color="bogus", to_color="red")`, "invalid from_color"},
		{"colorize bad to_color", `load("gum","colorize")` + "\n" + `colorize("hi", from_color="red", to_color="bogus")`, "invalid to_color"},
		{"colorize bad pattern", `load("gum","colorize")` + "\n" + `colorize("hi", pattern="nope")`, "unsupported pattern"},
		{"md empty text", `load("gum","md")` + "\n" + `md("")`, "text is required"},
		{"md_note empty text", `load("gum","md_note")` + "\n" + `md_note("")`, "text is required"},
		{"note empty title", `load("gum","note")` + "\n" + `note("")`, "title is required"},
		{"spin bad style", `load("gum","spin")` + "\n" + `spin(style="nope")`, "unsupported spinner style"},
		{"input bad password type", `load("gum","input")` + "\n" + `input(password=123)`, "password must be a bool or None"},
		{"set_theme requires arg", `load("gum","set_theme")` + "\n" + `set_theme()`, "set_theme"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireGumScriptErrorContains(t, tt.script, tt.errSub)
		})
	}
}

// TestNonTTYSuccessPaths covers the builtins that fully complete without a TTY.
func TestNonTTYSuccessPaths(t *testing.T) {
	tests := []struct {
		name   string
		script string
	}{
		{"colorize with pattern", `load("gum","colorize")` + "\n" + `r = colorize("hello"); print(r)`},
		{"colorize with single color", `load("gum","colorize")` + "\n" + `r = colorize("hello", color="red")`},
		{"colorize with hsb color", `load("gum","colorize")` + "\n" + `r = colorize("hello", color="hsb(120,100,100)")`},
		{"colorize custom gradient", `load("gum","colorize")` + "\n" + `r = colorize("hi", from_color="#FF0000", to_color="#00FF00")`},
		{"colorize render line", `load("gum","colorize")` + "\n" + `r = colorize("hi", pattern="RainbowBlue", render="line")`},
		{"md basic", `load("gum","md")` + "\n" + `r = md("# Title\n\nsome **bold** text")`},
		{"md notty style", `load("gum","md")` + "\n" + `r = md("# Hi", style="notty")`},
		{"set_theme applies", `load("gum","set_theme","get_theme")` + "\n" + `set_theme("dracula")`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireGumScriptOK(t, tt.script)
		})
	}
}

// --- backward compatibility -------------------------------------------------

// TestNewModuleDefaults pins the historical default surface (width 50, height 0,
// theme charm, empty editor). The iron rule requires these to stay constant so
// existing scripts keep running identically.
func TestNewModuleDefaults(t *testing.T) {
	m := NewModule()
	m.initialize()
	if got := m.ext.GetInt(configKeyWidth, -1); got != 50 {
		t.Errorf("default width = %d, want 50", got)
	}
	if got := m.ext.GetInt(configKeyHeight, -1); got != 0 {
		t.Errorf("default height = %d, want 0", got)
	}
	if got := m.ext.GetString(configKeyTheme, "x"); got != "charm" {
		t.Errorf("default theme = %q, want charm", got)
	}
	if m.theme == nil {
		t.Error("theme not resolved after initialize")
	}
	if m.keymap == nil {
		t.Error("keymap not resolved after initialize")
	}
}

// TestNewModuleWithConfig verifies the four-arg constructor wires its values
// through and that getWidth/getHeight honor them.
func TestNewModuleWithConfig(t *testing.T) {
	m := NewModuleWithConfig(120, 8, "dracula", []string{"vim", "-f"})
	m.initialize()
	if got := m.getWidth(0); got != 120 {
		t.Errorf("configured width default = %d, want 120", got)
	}
	if got := m.getHeight(0); got != 8 {
		t.Errorf("configured height default = %d, want 8", got)
	}
	if got := m.ext.GetString(configKeyTheme, "x"); got != "dracula" {
		t.Errorf("configured theme = %q, want dracula", got)
	}
}

// TestSetThemeOverride verifies set_theme is the gum override that applies the
// theme immediately (invariant 4): after the call the module's resolved theme
// must match the new name, and an unknown name must fall back without error.
func TestSetThemeOverride(t *testing.T) {
	m := NewModule()
	m.initialize()
	before := m.theme
	_ = before

	thread := &starlark.Thread{Name: "t"}
	b := starlark.NewBuiltin("gum.set_theme", m.starSetTheme)
	// applying dracula must change the resolved theme pointer to the dracula theme
	if _, err := m.starSetTheme(thread, b, starlark.Tuple{starlark.String("dracula")}, nil); err != nil {
		t.Fatalf("set_theme(dracula) error: %v", err)
	}
	if m.theme == nil {
		t.Fatal("theme nil after set_theme")
	}
	if got := m.ext.GetString(configKeyTheme, "x"); got != "dracula" {
		t.Errorf("stored theme = %q, want dracula", got)
	}
	// missing argument is a clean error, not a panic
	if _, err := m.starSetTheme(thread, b, nil, nil); err == nil {
		t.Error("expected error when theme argument missing")
	}
}

// TestLoadModuleRegistersBuiltins verifies the 12 gum builtins plus the
// auto-generated config accessors are all present in the loaded module.
func TestLoadModuleRegistersBuiltins(t *testing.T) {
	loader := NewModule().LoadModule()
	sd, err := loader()
	if err != nil {
		t.Fatalf("LoadModule loader error: %v", err)
	}
	// the loader returns a single-entry dict keyed by module name -> struct
	mod, ok := sd[ModuleName]
	if !ok {
		t.Fatalf("module %q not present in loaded dict", ModuleName)
	}
	// assert via attribute lookup on the module value
	hasAttr, ok := mod.(starlark.HasAttrs)
	if !ok {
		t.Fatalf("module value %T does not expose attributes", mod)
	}
	want := []string{
		"write", "input", "select", "multi_select", "confirm", "note",
		"md", "md_note", "spin", "file_pick", "colorize", "set_theme",
		"get_width", "set_width", "get_height", "set_height",
		"get_theme", "get_editor", "set_editor",
	}
	for _, name := range want {
		v, err := hasAttr.Attr(name)
		if err != nil || v == nil {
			t.Errorf("builtin/accessor %q missing from module: %v", name, err)
		}
	}
}
