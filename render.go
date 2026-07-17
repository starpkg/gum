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

// maxSpacingValues bounds a padding/margin value list. CSS-style spacing takes
// at most 4 components (top/right/bottom/left); a longer list is a mistake, and
// capping it stops an unbounded iterable (e.g. range(1e9)) from being fully
// materialized before its values are validated.
const maxSpacingValues = 4

// spacingInt converts a Starlark int to a Go padding/margin value. A negative
// value clamps to 0 (lipgloss does the same, so rejecting it would be gratuitous
// — and clamping also stops a huge negative from truncating to a positive on a
// 32-bit int). A positive value must fit int64 and stay within maxStyleDimension;
// the range check runs at int64 precision *before* narrowing to int, so an
// oversized value can neither wrap to 0 nor truncate on a 32-bit platform.
func spacingInt(i starlark.Int) (int, error) {
	if i.Sign() < 0 {
		return 0, nil
	}
	n, ok := i.Int64()
	if !ok || n > maxStyleDimension {
		return 0, fmt.Errorf("value %s exceeds the maximum of %d", i.String(), maxStyleDimension)
	}
	return int(n), nil
}

// toIntList converts a Starlark int, or a list/tuple of ints, to a []int. It is
// used for CSS-style padding/margin (1, 2, or 4 values). A None value yields a
// nil slice (meaning "unset").
func toIntList(v starlark.Value) ([]int, error) {
	if v == nil || v == starlark.None {
		return nil, nil
	}
	if i, ok := v.(starlark.Int); ok {
		n, err := spacingInt(i)
		if err != nil {
			return nil, err
		}
		return []int{n}, nil
	}
	elems, err := iterValuesCapped(v, maxSpacingValues)
	if err != nil {
		return nil, err
	}
	return intsFromValues(elems)
}

// intsFromValues converts a slice of Starlark values to []int, rejecting a
// non-int or an out-of-range int.
func intsFromValues(elems []starlark.Value) ([]int, error) {
	out := make([]int, 0, len(elems))
	for _, e := range elems {
		n, ok := e.(starlark.Int)
		if !ok {
			return nil, fmt.Errorf("expected int, got %s", e.Type())
		}
		m, err := spacingInt(n)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
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

// iterValuesCapped returns the elements of a Starlark list/tuple/iterable as a
// slice, but errors after max elements instead of materializing an unbounded
// iterable (e.g. range(1e9)) before the caller can validate its values. It
// terminates early even for a huge concrete list, so nothing is fully copied.
func iterValuesCapped(v starlark.Value, limit int) ([]starlark.Value, error) {
	if _, ok := v.(starlark.String); ok {
		return nil, fmt.Errorf("expected a list/tuple, got string")
	}
	it, ok := v.(starlark.Iterable)
	if !ok {
		return nil, fmt.Errorf("expected a list/tuple, got %s", v.Type())
	}
	out := make([]starlark.Value, 0, limit)
	iter := it.Iterate()
	defer iter.Done()
	var e starlark.Value
	for iter.Next(&e) {
		if len(out) >= limit {
			return nil, fmt.Errorf("expected at most %d values", limit)
		}
		out = append(out, e)
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
	// toIntList (via spacingInt) already bounds every value to maxStyleDimension,
	// so no further range check is needed here.
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
	text := a.text.GoString()
	if a.width > 0 {
		if a.width > maxStyleDimension {
			return none, fmt.Errorf("%s: width %d exceeds the maximum of %d", b.Name(), a.width, maxStyleDimension)
		}
		st = st.Width(a.width)
	}
	// Bound the whole rendered area, not just width×input-lines: horizontal
	// padding shrinks lipgloss's wrap width (so one long line becomes many padded
	// lines), and even with width==0 lipgloss equalizes every line to the widest
	// one. styleAreaBound accounts for both, so no padding/width/line-count combo
	// can amplify a small input into a huge allocation.
	if cells := styleAreaBound(text, st); cells > maxStyleCells {
		return none, fmt.Errorf("%s: rendered area (~%d cells) exceeds the maximum of %d", b.Name(), cells, maxStyleCells)
	}
	return starlark.String(st.Render(text)), nil
}

// ctrlDisplayCells upper-bounds the display width a single control byte can
// contribute after lipgloss renders it: a tab expands to its default 4 spaces
// and other C0 controls render as 0, so 8 is a safe over-estimate. Such bytes
// are invisible to ansi.StringWidth (it reports them as 0), so tab/control-heavy
// lines must be measured separately or they under-count.
const ctrlDisplayCells = 8

// styleAreaBound returns a conservative upper bound on the number of cells
// lipgloss will produce for text under style st, so no width/padding/margin/
// border/line-count combination can amplify a small input into a huge render.
//
// Output cells = output lines × output width. lipgloss wraps content at the
// effective width (fixed width − horizontal padding − border columns). Empirically
// (verified by TestStyleAreaBoundUpperBound): with effWidth ≥ 2 content wraps to
// ~width and only an atomic unit wider than effWidth overflows (bounded by
// ctrlDisplayCells); with effWidth ≤ 1 a break unit may not fit, so lipgloss
// renders unwrapped at the natural line width with up to one unit per row; with
// no width set nothing wraps. styleWrapMetrics bounds the wrap rows per line for
// whichever regime applies.
func styleAreaBound(text string, st lipgloss.Style) int64 {
	frameH := int64(st.GetHorizontalFrameSize())
	frameV := int64(st.GetVerticalFrameSize())
	hPad := int64(st.GetHorizontalPadding())
	borderH := frameH - hPad - int64(st.GetHorizontalMargins())
	width := int64(st.GetWidth())
	effWidth := width - hPad - borderH

	inputLines, maxDisplay, wrapExtra, controls := styleWrapMetrics(text, width, effWidth)

	// width==0 or effWidth ≤ 1: no wrapping (or it is abandoned), so a line renders
	// at its full natural width — bound the row width by the widest line.
	outWidth := max(width, maxDisplay) + frameH
	if width > 0 && effWidth >= 2 {
		// Content wraps to ~width. A wrapped row's display can still exceed width
		// when control bytes (tabs) expand *after* wrapping, so bound such a row by
		// effWidth worth of max-display units, capped by the whole line's display.
		rowWidth := width
		if controls > 0 {
			rowWidth = min(maxDisplay, effWidth*ctrlDisplayCells)
		}
		outWidth = max(width, rowWidth) + frameH
	}
	return (inputLines + wrapExtra + frameV) * outWidth
}

// styleWrapMetrics scans text once and returns the line count, the widest line's
// rendered display width, an upper bound on the extra rows wrapping adds, and the
// total control-byte count. Control bytes (< 0x20 or DEL, tab included) are
// over-counted: lipgloss advances one wrap unit per such byte (its ExecuteAction
// case) though ansi.StringWidth reports them as zero — display width is bounded at
// ctrlDisplayCells each, wrap units at one. (Escape-sequence bytes count too,
// which only over-estimates.)
func styleWrapMetrics(text string, width, effWidth int64) (lines, maxDisplay, wrapExtra, controls int64) {
	lines = 1
	start := 0
	for i := 0; i <= len(text); i++ {
		if i == len(text) || text[i] == '\n' {
			seg := text[start:i]
			ctrl := int64(countControlBytes(seg))
			controls += ctrl
			wrapUnits := int64(lipgloss.Width(seg)) + ctrl
			if display := wrapUnits + (ctrlDisplayCells-1)*ctrl; display > maxDisplay {
				maxDisplay = display
			}
			wrapExtra += lineWrapExtra(wrapUnits, width, effWidth)
			if i < len(text) {
				lines++
			}
			start = i + 1
		}
	}
	return lines, maxDisplay, wrapExtra, controls
}

// lineWrapExtra upper-bounds the rows a single line adds beyond its first when
// wrapped at effWidth. A line that fits (or an unset width) adds none; at effWidth
// ≤ 1 a line may render one unit per row; otherwise word-wrap wastes up to half a
// row per break, so 2×wrapUnits/effWidth + 1 bounds it.
func lineWrapExtra(wrapUnits, width, effWidth int64) int64 {
	switch {
	case width == 0 || wrapUnits <= effWidth:
		return 0
	case effWidth <= 1:
		return wrapUnits
	default:
		return 2*wrapUnits/effWidth + 1
	}
}

// countControlBytes counts the C0 control bytes (below space, or DEL) in s —
// tab included. Each advances lipgloss's wrap position by one while contributing
// nothing to ansi.StringWidth.
func countControlBytes(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if b := s[i]; b < 0x20 || b == 0x7f {
			n++
		}
	}
	return n
}

// blockDims returns the rendered display width (widest line) and height (line
// count) of an already-rendered block. Control bytes (tab included) are bounded
// at ctrlDisplayCells each, since they display wider than ansi.StringWidth reports.
func blockDims(s string) (width, height int64) {
	height = 1
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == '\n' {
			seg := s[start:i]
			w := int64(lipgloss.Width(seg)) + ctrlDisplayCells*int64(countControlBytes(seg))
			if w > width {
				width = w
			}
			if i < len(s) {
				height++
			}
			start = i + 1
		}
	}
	return width, height
}

// composeAreaBound upper-bounds the cells lipgloss.Join{Vertical,Horizontal}
// produces for the pre-rendered blocks. A vertical join stacks the blocks and
// pads each to the widest, so cells ≤ Σheights × maxWidth; a horizontal join
// places them side by side padded to the tallest, so cells ≤ maxHeight × Σwidths.
func composeAreaBound(blocks []string, horizontal bool) int64 {
	var sumW, sumH, maxW, maxH int64
	for _, blk := range blocks {
		w, h := blockDims(blk)
		sumW += w
		sumH += h
		if w > maxW {
			maxW = w
		}
		if h > maxH {
			maxH = h
		}
	}
	if horizontal {
		return maxH * sumW
	}
	return sumH * maxW
}

// tableAreaBound upper-bounds the cells lipgloss renders for a content-sized
// table: output rows ≈ Σ(each row's tallest cell) + header + border/separator
// rows; output width ≈ Σ(each column's widest cell) + border/padding columns.
func tableAreaBound(headers []string, rows [][]string) int64 {
	cols := len(headers)
	for _, r := range rows {
		if len(r) > cols {
			cols = len(r)
		}
	}
	colWidth := make([]int64, cols)
	totalRows := accumulateCells(headers, colWidth) // header height
	for _, r := range rows {
		totalRows += accumulateCells(r, colWidth) // each row's tallest cell
	}
	var tableWidth int64
	for _, w := range colWidth {
		tableWidth += w
	}
	// Generous allowance for lipgloss's per-column padding/separators and the
	// header separator + top/bottom borders.
	tableWidth += int64(cols)*4 + 4
	totalRows += int64(len(rows)) + 4
	return totalRows * tableWidth
}

// accumulateCells folds one row's cells into the per-column max widths and
// returns the row's height (its tallest cell's line count, at least 1).
func accumulateCells(cells []string, colWidth []int64) int64 {
	var rowH int64 = 1
	for c, cell := range cells {
		w, h := blockDims(cell)
		if w > colWidth[c] {
			colWidth[c] = w
		}
		if h > rowH {
			rowH = h
		}
	}
	return rowH
}

// maxStyleDimension bounds a script-supplied width / padding / margin so a huge
// value can't drive lipgloss into a multi-gigabyte padded-string allocation
// (OOM). It is far above any real terminal width.
const maxStyleDimension = 10000

// maxStyleCells bounds the rendered area (output lines × output width, see
// styleAreaBound) so a small input can't be amplified — by a large width, wrap
// -shrinking padding, or many lines — into a multi-gigabyte allocation. 10
// million cells is far beyond any real terminal render.
const maxStyleCells = 10_000_000

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
	// Columns size to their content, so many rows or a very wide cell would
	// amplify a small input into a huge table; bound the rendered area.
	if cells := tableAreaBound(headers, rows); cells > maxStyleCells {
		return none, fmt.Errorf("%s: rendered area (~%d cells) exceeds the maximum of %d", b.Name(), cells, maxStyleCells)
	}

	t := table.New().Headers(headers...).Rows(rows...)
	if t, err = applyTableBorder(t, border, borderFg); err != nil {
		return none, err
	}
	return starlark.String(t.String()), nil
}

// applyTableBorder resolves the optional border style and foreground color onto t.
func applyTableBorder(t *table.Table, border, borderFg *types.NullableStringOrBytes) (*table.Table, error) {
	if !border.IsNullOrEmpty() {
		bd, err := parseBorder(border.GoString())
		if err != nil {
			return t, err
		}
		t = t.Border(bd)
	}
	if !borderFg.IsNullOrEmpty() {
		c, err := ParseColor(borderFg.GoString())
		if err != nil {
			return t, fmt.Errorf("border_fg: %w", err)
		}
		t = t.BorderStyle(lipgloss.NewStyle().Foreground(c))
	}
	return t, nil
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
	if err := appendTreeChildren(t, data, 1); err != nil {
		return none, fmt.Errorf("%s: %w", b.Name(), err)
	}
	return starlark.String(t.String()), nil
}

// maxTreeDepth bounds tree() nesting. Recursing over a script-supplied structure
// with no limit would overflow the goroutine stack on deeply nested input — an
// uncatchable fatal error. 1000 is far beyond any readable display tree, far
// below the stack limit.
const maxTreeDepth = 1000

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
	horizontal := false
	switch strings.ToLower(strings.TrimSpace(dir)) {
	case "v", "vertical", "":
	case "h", "horizontal":
		horizontal = true
	default:
		return none, fmt.Errorf(`unsupported dir: %s (want "h" or "v")`, dir)
	}
	// Joining pads every block to the widest (vertical) or tallest (horizontal)
	// one, so a single large block plus many others amplifies; bound the area.
	if cells := composeAreaBound(parts, horizontal); cells > maxStyleCells {
		return none, fmt.Errorf("%s: composed area (~%d cells) exceeds the maximum of %d", b.Name(), cells, maxStyleCells)
	}
	if horizontal {
		return starlark.String(lipgloss.JoinHorizontal(pos, parts...)), nil
	}
	return starlark.String(lipgloss.JoinVertical(pos, parts...)), nil
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
func appendTreeChildren(parent *tree.Tree, v starlark.Value, depth int) error {
	if depth > maxTreeDepth {
		return fmt.Errorf("tree nesting exceeds %d levels", maxTreeDepth)
	}
	switch t := v.(type) {
	case *starlark.Dict:
		return appendDictBranches(parent, t, depth)
	case *starlark.List:
		return appendTreeSeq(parent, t, depth)
	case starlark.Tuple:
		return appendTreeSeq(parent, t, depth)
	default:
		parent.Child(dataconv.StarString(v))
		return nil
	}
}

// appendDictBranches renders each dict entry as a branch: a scalar value joins
// onto the key, a composite value nests under it.
func appendDictBranches(parent *tree.Tree, d *starlark.Dict, depth int) error {
	for _, k := range d.Keys() {
		val, _, _ := d.Get(k)
		ks := dataconv.StarString(k)
		if isTreeComposite(val) {
			sub := tree.Root(ks)
			if err := appendTreeChildren(sub, val, depth+1); err != nil {
				return err
			}
			parent.Child(sub)
		} else {
			parent.Child(ks + " " + dataconv.StarString(val))
		}
	}
	return nil
}

// appendTreeSeq renders each element of an indexable (list/tuple) as a node.
func appendTreeSeq(parent *tree.Tree, seq starlark.Indexable, depth int) error {
	for i := 0; i < seq.Len(); i++ {
		if err := appendTreeChild(parent, seq.Index(i), depth); err != nil {
			return err
		}
	}
	return nil
}

// appendTreeChild adds a single list element: a composite nests as an unlabeled
// subtree, a scalar becomes a leaf.
func appendTreeChild(parent *tree.Tree, e starlark.Value, depth int) error {
	if isTreeComposite(e) {
		sub := tree.New()
		if err := appendTreeChildren(sub, e, depth+1); err != nil {
			return err
		}
		parent.Child(sub)
	} else {
		parent.Child(dataconv.StarString(e))
	}
	return nil
}
