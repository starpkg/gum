package gum

import (
	"errors"
	"os"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
	"github.com/sahilm/fuzzy"
	"go.starlark.net/starlark"
)

// filter.go holds the fuzzy filter builtin. huh's own filtering is a fixed
// case-insensitive substring match with no public hook, so a genuinely fuzzy,
// incremental picker (distinct from select's substring filter) is driven by a
// small bubbletea v2 model here, ranking with github.com/sahilm/fuzzy.

// fuzzyRank returns the indices of items matching query, best match first. An
// empty query returns every index in original order. When useFuzzy is false it
// falls back to a case-insensitive substring match (the same behavior as
// select's filter). The pure ranking is unit-tested; the interactive model
// below is TTY-only.
func fuzzyRank(items []string, query string, useFuzzy bool) []int {
	if query == "" {
		idx := make([]int, len(items))
		for i := range items {
			idx[i] = i
		}
		return idx
	}
	if useFuzzy {
		matches := fuzzy.Find(query, items)
		idx := make([]int, len(matches))
		for i, mt := range matches {
			idx[i] = mt.Index
		}
		return idx
	}
	var idx []int
	q := strings.ToLower(query)
	for i, it := range items {
		if strings.Contains(strings.ToLower(it), q) {
			idx = append(idx, i)
		}
	}
	return idx
}

// filterModel is the bubbletea v2 model backing the filter builtin: a query
// input over a fuzzy-ranked, scrollable list. limit == 1 is single-select;
// any other limit (0 = unlimited) is multi-select toggled with Tab.
type filterModel struct {
	ti       textinput.Model
	opts     []huh.Option[string] // Key is displayed/matched, Value is returned
	keys     []string             // opts' keys, for ranking
	filtered []int                // indices into opts, the current match set
	cursor   int                  // index into filtered
	selected map[int]bool         // marked option indices (multi-select)
	limit    int
	height   int
	useFuzzy bool
	title    string
	chosen   []string // result values, set on confirm
	aborted  bool
}

func newFilterModel(opts []huh.Option[string], value, placeholder, title string, useFuzzy bool, limit, height int) filterModel {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.SetValue(value)
	ti.Focus()
	keys := make([]string, len(opts))
	for i, o := range opts {
		keys[i] = o.Key
	}
	return filterModel{
		ti:       ti,
		opts:     opts,
		keys:     keys,
		filtered: fuzzyRank(keys, value, useFuzzy),
		selected: map[int]bool{},
		limit:    limit,
		height:   height,
		useFuzzy: useFuzzy,
		title:    title,
	}
}

func (m filterModel) Init() tea.Cmd { return textinput.Blink }

func (m filterModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		if cmd, handled := m.handleKey(k.String()); handled {
			return m, cmd
		}
	}
	var cmd tea.Cmd
	m.ti, cmd = m.ti.Update(msg)
	m.refilter()
	return m, cmd
}

// handleKey processes a control/navigation key. The bool reports whether the
// key was consumed (so non-handled keys fall through to the text input).
func (m *filterModel) handleKey(key string) (tea.Cmd, bool) {
	switch key {
	case "ctrl+c", "esc":
		m.aborted = true
		return tea.Quit, true
	case "enter":
		m.confirm()
		return tea.Quit, true
	case "up", "ctrl+p":
		m.move(-1)
		return nil, true
	case "down", "ctrl+n":
		m.move(1)
		return nil, true
	case "tab":
		m.toggle()
		return nil, true
	}
	return nil, false
}

// move shifts the cursor within the current match set.
func (m *filterModel) move(delta int) {
	if n := m.cursor + delta; n >= 0 && n < len(m.filtered) {
		m.cursor = n
	}
}

// refilter re-ranks the options against the current query, keeping the cursor
// within range.
func (m *filterModel) refilter() {
	m.filtered = fuzzyRank(m.keys, m.ti.Value(), m.useFuzzy)
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

// confirm records the selection into chosen.
func (m *filterModel) confirm() {
	if m.limit != 1 && len(m.selected) > 0 {
		for i := range m.opts {
			if m.selected[i] {
				m.chosen = append(m.chosen, m.opts[i].Value)
			}
		}
		return
	}
	// single-select, or multi with nothing marked: take the highlighted item.
	if len(m.filtered) > 0 {
		m.chosen = []string{m.opts[m.filtered[m.cursor]].Value}
	}
}

// toggle marks/unmarks the highlighted item (multi-select only), honoring limit.
func (m *filterModel) toggle() {
	if m.limit == 1 || len(m.filtered) == 0 {
		return
	}
	i := m.filtered[m.cursor]
	switch {
	case m.selected[i]:
		delete(m.selected, i)
	case m.limit == 0 || len(m.selected) < m.limit:
		m.selected[i] = true
	}
}

func (m filterModel) View() tea.View {
	var b strings.Builder
	if m.title != "" {
		b.WriteString(m.title + "\n")
	}
	b.WriteString(m.ti.View() + "\n")
	start, end := filterWindow(m.cursor, len(m.filtered), m.height)
	for vi := start; vi < end; vi++ {
		oi := m.filtered[vi]
		line := "  "
		if vi == m.cursor {
			line = "> "
		}
		if m.limit != 1 {
			if m.selected[oi] {
				line += "[x] "
			} else {
				line += "[ ] "
			}
		}
		b.WriteString(line + m.keys[oi] + "\n")
	}
	return tea.NewView(b.String())
}

// filterWindow returns the [start,end) slice of a list of length n to show so
// that cursor stays visible within at most height rows.
func filterWindow(cursor, n, height int) (int, int) {
	if height <= 0 || height >= n {
		return 0, n
	}
	start := cursor - height/2
	if start < 0 {
		start = 0
	}
	end := start + height
	if end > n {
		end = n
		start = end - height
	}
	return start, end
}

// starFilter is a Starlark function to interactively fuzzy-filter a list.
// def filter(options, value="", placeholder="Filter...", title="", fuzzy=True, limit=1, height=10) -> Union[str, List[str], None]
//
// Unlike select (a fixed substring filter), filter ranks options by fuzzy match
// as you type. limit=1 (default) returns the chosen string; any other limit
// (0 = unlimited) is multi-select (Tab to mark) and returns a list. Ctrl-C / Esc
// returns None. Requires a TTY.
func (m *Module) starFilter(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		options     starlark.Value
		value       = ""
		placeholder = "Filter..."
		title       = ""
		useFuzzy    = true
		limit       = 1
		height      = 10
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"options", &options,
		"value?", &value,
		"placeholder?", &placeholder,
		"title?", &title,
		"fuzzy?", &useFuzzy,
		"limit?", &limit,
		"height?", &height,
	); err != nil {
		return none, err
	}

	opts, err := convertOptionList(options)
	if err != nil {
		return none, err
	}
	if len(opts) == 0 {
		return none, errors.New("options must not be empty")
	}

	fm := newFilterModel(opts, value, placeholder, title, useFuzzy, limit, height)
	out, err := tea.NewProgram(fm, tea.WithOutput(os.Stderr)).Run()
	if err != nil {
		return none, err
	}
	return filterResult(out, limit), nil
}

// filterResult converts the finished filter model into the Starlark value: a
// single string for limit==1, a list otherwise, or None when aborted/empty.
func filterResult(out tea.Model, limit int) starlark.Value {
	res, ok := out.(filterModel)
	if !ok || res.aborted {
		return none
	}
	if limit == 1 {
		if len(res.chosen) == 0 {
			return none
		}
		return starlark.String(res.chosen[0])
	}
	vals := make([]starlark.Value, len(res.chosen))
	for i, s := range res.chosen {
		vals[i] = starlark.String(s)
	}
	return starlark.NewList(vals)
}
