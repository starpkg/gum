package gum

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"charm.land/lipgloss/v2/tree"
	"github.com/1set/starlet/dataconv"
	"github.com/1set/starlet/dataconv/types"
	"go.starlark.net/starlark"
)

// render.go holds the non-interactive lipgloss v2 static renderers: style
// (styled text / boxes), table (bordered tables), and tree (nested trees).
// Unlike the huh-driven builtins these never open a TTY — they take data and
// return a rendered string, so they work in headless environments.

// borderMap maps a border name to its lipgloss border. Names are matched
// case-insensitively; "none"/"hidden" yield an invisible (space) border.
var borderMap = map[string]func() lipgloss.Border{
	"normal":  lipgloss.NormalBorder,
	"square":  lipgloss.NormalBorder,
	"rounded": lipgloss.RoundedBorder,
	"round":   lipgloss.RoundedBorder,
	"thick":   lipgloss.ThickBorder,
	"bold":    lipgloss.ThickBorder,
	"double":  lipgloss.DoubleBorder,
	"block":   lipgloss.BlockBorder,
	"hidden":  lipgloss.HiddenBorder,
	"none":    lipgloss.HiddenBorder,
}

// parseBorder resolves a border name to a lipgloss.Border.
func parseBorder(name string) (lipgloss.Border, error) {
	if f, ok := borderMap[strings.ToLower(strings.TrimSpace(name))]; ok {
		return f(), nil
	}
	return lipgloss.Border{}, fmt.Errorf("unsupported border style: %s", name)
}

// parseAlign resolves a horizontal alignment name to a lipgloss.Position.
func parseAlign(name string) (lipgloss.Position, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "left", "start":
		return lipgloss.Left, nil
	case "center", "centre", "middle":
		return lipgloss.Center, nil
	case "right", "end":
		return lipgloss.Right, nil
	default:
		return lipgloss.Left, fmt.Errorf("unsupported align: %s", name)
	}
}

// parsePosition resolves a cross-axis position name to a lipgloss.Position,
// accepting both horizontal (left/center/right) and vertical (top/center/bottom)
// names since compose's alignment axis depends on its direction.
func parsePosition(name string) (lipgloss.Position, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "left", "top", "start":
		return lipgloss.Top, nil // Top == Left == 0
	case "center", "centre", "middle":
		return lipgloss.Center, nil
	case "right", "bottom", "end":
		return lipgloss.Bottom, nil // Bottom == Right == 1
	default:
		return lipgloss.Left, fmt.Errorf("unsupported align: %s", name)
	}
}

// toIntList converts a Starlark int, or a list/tuple of ints, to a []int. It is
// used for CSS-style padding/margin (1, 2, or 4 values). A None value yields a
// nil slice (meaning "unset").
func toIntList(v starlark.Value) ([]int, error) {
	if v == nil || v == starlark.None {
		return nil, nil
	}
	if i, ok := v.(starlark.Int); ok {
		n, _ := i.Int64()
		return []int{int(n)}, nil
	}
	elems, err := iterValues(v)
	if err != nil {
		return nil, err
	}
	out := make([]int, 0, len(elems))
	for _, e := range elems {
		n, ok := e.(starlark.Int)
		if !ok {
			return nil, fmt.Errorf("expected int, got %s", e.Type())
		}
		i, _ := n.Int64()
		out = append(out, int(i))
	}
	return out, nil
}

// starStringSlice converts a Starlark list/tuple/iterable to a []string,
// stringifying each element. A bare string is rejected (it is not a row).
func starStringSlice(v starlark.Value) ([]string, error) {
	switch t := v.(type) {
	case *starlark.List:
		out := make([]string, t.Len())
		for i := 0; i < t.Len(); i++ {
			out[i] = dataconv.StarString(t.Index(i))
		}
		return out, nil
	case starlark.Tuple:
		out := make([]string, len(t))
		for i, e := range t {
			out[i] = dataconv.StarString(e)
		}
		return out, nil
	case starlark.String:
		return nil, fmt.Errorf("expected a list/tuple of values, got string")
	case starlark.Iterable:
		var out []string
		it := t.Iterate()
		defer it.Done()
		var e starlark.Value
		for it.Next(&e) {
			out = append(out, dataconv.StarString(e))
		}
		return out, nil
	default:
		return nil, fmt.Errorf("expected a list/tuple of values, got %s", v.Type())
	}
}

// starStringMatrix converts a Starlark list/tuple/iterable of rows (each itself
// a list/tuple/iterable) to a [][]string.
func starStringMatrix(v starlark.Value) ([][]string, error) {
	rowsAsValues, err := iterValues(v)
	if err != nil {
		return nil, err
	}
	out := make([][]string, 0, len(rowsAsValues))
	for i, rowVal := range rowsAsValues {
		row, err := starStringSlice(rowVal)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", i, err)
		}
		out = append(out, row)
	}
	return out, nil
}

// iterValues returns the elements of a Starlark list/tuple/iterable as a slice,
// rejecting a bare string (which would otherwise iterate by character).
func iterValues(v starlark.Value) ([]starlark.Value, error) {
	switch t := v.(type) {
	case *starlark.List:
		out := make([]starlark.Value, t.Len())
		for i := 0; i < t.Len(); i++ {
			out[i] = t.Index(i)
		}
		return out, nil
	case starlark.Tuple:
		return []starlark.Value(t), nil
	case starlark.String:
		return nil, fmt.Errorf("expected a list/tuple, got string")
	case starlark.Iterable:
		var out []starlark.Value
		it := t.Iterate()
		defer it.Done()
		var e starlark.Value
		for it.Next(&e) {
			out = append(out, e)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("expected a list/tuple, got %s", v.Type())
	}
}

// applyColor parses a gum color string and applies it to st via set; an empty
// value leaves st unchanged.
func applyColor(st lipgloss.Style, v *types.NullableStringOrBytes, set func(lipgloss.Style, color.Color) lipgloss.Style) (lipgloss.Style, error) {
	if v.IsNullOrEmpty() {
		return st, nil
	}
	c, err := ParseColor(v.GoString())
	if err != nil {
		return st, err
	}
	return set(st, c), nil
}

// applyStyleColors applies the foreground, background, and border colors,
// prefixing any parse error with the offending field name.
func applyStyleColors(st lipgloss.Style, fg, bg, borderFg *types.NullableStringOrBytes) (lipgloss.Style, error) {
	var err error
	if st, err = applyColor(st, fg, lipgloss.Style.Foreground); err != nil {
		return st, fmt.Errorf("fg: %w", err)
	}
	if st, err = applyColor(st, bg, lipgloss.Style.Background); err != nil {
		return st, fmt.Errorf("bg: %w", err)
	}
	setBorderFg := func(s lipgloss.Style, c color.Color) lipgloss.Style { return s.BorderForeground(c) }
	if st, err = applyColor(st, borderFg, setBorderFg); err != nil {
		return st, fmt.Errorf("border_fg: %w", err)
	}
	return st, nil
}

// applyTextAttrs applies the boolean text attributes.
func applyTextAttrs(st lipgloss.Style, bold, italic, underline, faint bool) lipgloss.Style {
	if bold {
		st = st.Bold(true)
	}
	if italic {
		st = st.Italic(true)
	}
	if underline {
		st = st.Underline(true)
	}
	if faint {
		st = st.Faint(true)
	}
	return st
}

// applySpacing applies a CSS-style spacing value (int, or list/tuple of ints)
// to st via set, leaving st unchanged when the value is unset.
func applySpacing(st lipgloss.Style, v starlark.Value, set func(lipgloss.Style, ...int) lipgloss.Style) (lipgloss.Style, error) {
	p, err := toIntList(v)
	if err != nil {
		return st, err
	}
	if len(p) > 0 {
		st = set(st, p...)
	}
	return st, nil
}

// applyStyleBox applies the border, padding/margin spacing, and alignment.
func applyStyleBox(st lipgloss.Style, border, align *types.NullableStringOrBytes, padding, margin starlark.Value) (lipgloss.Style, error) {
	if !border.IsNullOrEmpty() {
		bd, err := parseBorder(border.GoString())
		if err != nil {
			return st, err
		}
		st = st.Border(bd)
	}
	var err error
	if st, err = applySpacing(st, padding, lipgloss.Style.Padding); err != nil {
		return st, fmt.Errorf("padding: %w", err)
	}
	if st, err = applySpacing(st, margin, lipgloss.Style.Margin); err != nil {
		return st, fmt.Errorf("margin: %w", err)
	}
	if !align.IsNullOrEmpty() {
		p, err := parseAlign(align.GoString())
		if err != nil {
			return st, err
		}
		st = st.Align(p)
	}
	return st, nil
}

// starStyle is a Starlark function to render styled text with lipgloss (the
// non-interactive equivalent of `gum style`).
// def style(text, fg="", bg="", bold=False, italic=False, underline=False, faint=False, border="", border_fg="", padding=None, margin=None, width=0, align="") -> str
func (m *Module) starStyle(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	a, err := unpackStyleArgs(b, args, kwargs)
	if err != nil {
		return none, err
	}
	st := lipgloss.NewStyle()
	if st, err = applyStyleColors(st, a.fg, a.bg, a.borderFg); err != nil {
		return none, err
	}
	st = applyTextAttrs(st, a.bold, a.italic, a.underline, a.faint)
	if st, err = applyStyleBox(st, a.border, a.align, a.padding, a.margin); err != nil {
		return none, err
	}
	if a.width > 0 {
		st = st.Width(a.width)
	}
	return starlark.String(st.Render(a.text.GoString())), nil
}

// styleArgs holds the parsed arguments for the style builtin.
type styleArgs struct {
	text             types.StringOrBytes
	fg, bg           *types.NullableStringOrBytes
	bold, italic     bool
	underline, faint bool
	border, borderFg *types.NullableStringOrBytes
	padding, margin  starlark.Value
	width            int
	align            *types.NullableStringOrBytes
}

// unpackStyleArgs parses the style() arguments into a styleArgs.
func unpackStyleArgs(b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (*styleArgs, error) {
	a := &styleArgs{
		text:     types.StringOrBytes(""),
		fg:       types.NewNullableStringOrBytes(""),
		bg:       types.NewNullableStringOrBytes(""),
		border:   types.NewNullableStringOrBytes(""),
		borderFg: types.NewNullableStringOrBytes(""),
		align:    types.NewNullableStringOrBytes(""),
	}
	err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"text", &a.text,
		"fg?", a.fg,
		"bg?", a.bg,
		"bold?", &a.bold,
		"italic?", &a.italic,
		"underline?", &a.underline,
		"faint?", &a.faint,
		"border?", a.border,
		"border_fg?", a.borderFg,
		"padding?", &a.padding,
		"margin?", &a.margin,
		"width?", &a.width,
		"align?", a.align,
	)
	return a, err
}

// starTable is a Starlark function to render a bordered table with lipgloss.
// def table(headers, rows, border="rounded", border_fg="") -> str
func (m *Module) starTable(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		headersVal starlark.Value                              // list of header strings
		rowsVal    starlark.Value                              // list of rows (each a list of cell strings)
		border     = types.NewNullableStringOrBytes("rounded") // border style name
		borderFg   = types.NewNullableStringOrBytes("")        // border foreground color
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"headers", &headersVal,
		"rows", &rowsVal,
		"border?", border,
		"border_fg?", borderFg,
	); err != nil {
		return none, err
	}

	headers, err := starStringSlice(headersVal)
	if err != nil {
		return none, fmt.Errorf("headers: %w", err)
	}
	rows, err := starStringMatrix(rowsVal)
	if err != nil {
		return none, fmt.Errorf("rows: %w", err)
	}

	t := table.New().Headers(headers...).Rows(rows...)
	if !border.IsNullOrEmpty() {
		bd, err := parseBorder(border.GoString())
		if err != nil {
			return none, err
		}
		t = t.Border(bd)
	}
	if !borderFg.IsNullOrEmpty() {
		c, err := ParseColor(borderFg.GoString())
		if err != nil {
			return none, fmt.Errorf("border_fg: %w", err)
		}
		t = t.BorderStyle(lipgloss.NewStyle().Foreground(c))
	}
	return starlark.String(t.String()), nil
}

// starTree is a Starlark function to render a nested tree with lipgloss.
// def tree(data, root="") -> str
//
// data is a dict, list, or scalar. A dict renders each key as a branch — a
// scalar value joins onto the key ("key value"), a dict/list value nests under
// it. A list renders each element as a node. root, if set, labels the top.
func (m *Module) starTree(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		data starlark.Value                       // the tree data
		root = types.NewNullableStringOrBytes("") // optional label for the root node
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"data", &data,
		"root?", root,
	); err != nil {
		return none, err
	}

	t := tree.New()
	if !root.IsNullOrEmpty() {
		t = t.Root(root.GoString())
	}
	appendTreeChildren(t, data)
	return starlark.String(t.String()), nil
}

// starCompose is a Starlark function to join already-rendered blocks into a
// layout, horizontally or vertically, with lipgloss.
// def compose(blocks, dir="v", align="left") -> str
func (m *Module) starCompose(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		blocks starlark.Value                           // list of pre-rendered string blocks
		dir    = "v"                                    // "v"/"vertical" or "h"/"horizontal"
		align  = types.NewNullableStringOrBytes("left") // cross-axis alignment
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"blocks", &blocks,
		"dir?", &dir,
		"align?", align,
	); err != nil {
		return none, err
	}
	parts, err := starStringSlice(blocks)
	if err != nil {
		return none, fmt.Errorf("blocks: %w", err)
	}
	pos, err := parsePosition(align.GoString())
	if err != nil {
		return none, err
	}
	switch strings.ToLower(strings.TrimSpace(dir)) {
	case "v", "vertical", "":
		return starlark.String(lipgloss.JoinVertical(pos, parts...)), nil
	case "h", "horizontal":
		return starlark.String(lipgloss.JoinHorizontal(pos, parts...)), nil
	default:
		return none, fmt.Errorf(`unsupported dir: %s (want "h" or "v")`, dir)
	}
}

// isTreeComposite reports whether v should nest as a subtree rather than render
// as a leaf.
func isTreeComposite(v starlark.Value) bool {
	switch v.(type) {
	case *starlark.Dict, *starlark.List, starlark.Tuple:
		return true
	}
	return false
}

// appendTreeChildren adds v's contents to parent as tree children: dict entries
// nest by key, list/tuple elements append in order, and scalars become leaves.
func appendTreeChildren(parent *tree.Tree, v starlark.Value) {
	switch t := v.(type) {
	case *starlark.Dict:
		for _, k := range t.Keys() {
			val, _, _ := t.Get(k)
			ks := dataconv.StarString(k)
			if isTreeComposite(val) {
				sub := tree.Root(ks)
				appendTreeChildren(sub, val)
				parent.Child(sub)
			} else {
				parent.Child(ks + " " + dataconv.StarString(val))
			}
		}
	case *starlark.List:
		for i := 0; i < t.Len(); i++ {
			appendTreeChild(parent, t.Index(i))
		}
	case starlark.Tuple:
		for _, e := range t {
			appendTreeChild(parent, e)
		}
	default:
		parent.Child(dataconv.StarString(v))
	}
}

// appendTreeChild adds a single list element: a composite nests as an unlabeled
// subtree, a scalar becomes a leaf.
func appendTreeChild(parent *tree.Tree, e starlark.Value) {
	if isTreeComposite(e) {
		sub := tree.New()
		appendTreeChildren(sub, e)
		parent.Child(sub)
	} else {
		parent.Child(dataconv.StarString(e))
	}
}
