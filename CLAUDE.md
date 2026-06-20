# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`starpkg/gum` is an **L4 domain module** of the Star\* ecosystem: it exposes a terminal-UI toolkit to Starlark scripts. A script imports the module and prompts for text, chooses from lists, confirms actions, picks files, shows spinners and notes, renders Markdown, and colorizes text — all by driving the **host's own terminal**.

starpkg as a whole is **support for necessary local operations plus simple abstractions over common online services, for ease of use.** `gum` sits firmly on the *local* side: it is a **local capability**, with **no network access** — it talks only to the controlling TTY. It wraps the Charm stack ([`huh`](https://github.com/charmbracelet/huh) forms, [`bubbletea`](https://github.com/charmbracelet/bubbletea), [`glamour`](https://github.com/charmbracelet/glamour) Markdown, [`lipgloss`](https://github.com/charmbracelet/lipgloss)) plus `bitbucket.org/ai69/colorlogo` for gradient text, behind one flat set of Starlark builtins.

Layer position: depends downward on `starpkg/base` (the module/config system + the `RunStarlarkTests` harness), `1set/starlet` (the Machine + `dataconv`/`dataconv/types` argument helpers), and transitively `1set/starlight` + `go.starlark.net`. Nothing in the ecosystem depends on it.

## Dev commands

Pure Go library with a Makefile. From this repo:

```bash
make test                                  # -race -cover, the working bar
make ci                                    # -race -cover profile + bench compile (what CI runs)
go test ./... -run TestStarlarkScripts     # the integration harness
gofmt -l . && go vet ./...                 # must be clean before commit
go run github.com/1set/meta/doccov@master . # doc-coverage gate (must exit 0)
```

**Verify on the go floor in Docker** — this repo's floor is **go 1.21** (see Release discipline), and may differ from the local toolchain. Behavior on the floor must be checked in a container:

```bash
docker run --rm -v "$PWD":/src -v "$HOME/go/pkg/mod":/go/pkg/mod -w /src golang:1.21 go test -race -count=1 ./...
```

**TTY note.** Every interactive builtin (`input`, `select`, `multi_select`, `filter`, `confirm`, `file_pick`, `write`, `note`, `spin`) opens `/dev/tty` via `huh`/`bubbletea`. Headless environments (CI, sandboxes, plain `go test` without a terminal) make these fail with `could not open a new TTY`. That is an environment limitation, **not** a code regression; non-interactive paths (`colorize`, `md`, argument validation, the `panic-*` scripts) run anywhere. Integration scripts under `../test/gum/*.star` live in the **private `starpkg/test` repo** and the harness **auto-skips** when that directory is absent (e.g. in CI).

## Architecture (the part that spans files)

The module is a **thin, flat bridge**: one `Module` value carries config + a resolved `huh.Theme` + a `huh.KeyMap`, and `LoadModule()` registers a flat `starlark.StringDict` of builtins. Most builtins follow the same shape — `UnpackArgs` into typed locals, build a one-group `huh.NewForm(...)`, `.Run()` it, then map `huh` errors through `ignorableError` (user-abort / timeout become `None`, real errors propagate).

- **`gum.go`** — module entry + shared plumbing. `Module` wraps a `base.ConfigurableModule` (+ its `Extend()`); `NewModule()` / `NewModuleWithConfig(width, height int, themeName string, editor []string)` construct it. `LoadModule()` calls `initialize()` (resolve theme + keymap, once) and registers the additional builtins. Holds the config-key constants, `getWidth`/`getHeight` defaulting, `ignorableError`, the generic `convertList`/`convertListToStrings`/`convertValidator` helpers, `convertDuration`, `applyTheme`, and the overriding `starSetTheme`.
- **`input.go`** — `starWrite` (`write`, multi-line textarea) and `starInput` (`input`, single-line, password/echo modes, suggestions).
- **`select.go`** — `starSelect` (`select`), `starMultiSelect` (`multi_select`), `starConfirm` (`confirm`), `starFilePicker` (`file_pick`), and `convertOptionList` (Starlark list / dict / iterable → `[]huh.Option[string]`; dict keys are displayed, values returned).
- **`output.go`** — `starNote` (`note`), `starSpinner` (`spin`, with `spinStyleMap`), and `starColorize` (`colorize`, with `colorFuncMap` of `colorlogo` gradient renderers + `toRGBA`/`normalizePattern`/`normalizeRenderType`).
- **`markdown.go`** — `starMarkdown` (`md`, `glamour` → ANSI) and `starMarkdownNote` (`md_note`, renders then re-dispatches into `starNote`).
- **`color.go`** — the exported `Color*` palette + `RainbowColors`, the `presetColorMap`/`hexNameMap`/`colorNames` tables, and `ParseColor` (case-insensitive: preset name, `rgb(...)`, `hsb(...)`, `#RRGGBB`, `#RGB`).
- **`filter.go`** — `starFilter` (`filter`), an interactive **fuzzy** picker. huh's own filtering is a fixed substring match with no public hook, so this drives a small `bubbletea` v2 model (`filterModel`) + `textinput`, ranking with `github.com/sahilm/fuzzy` (`fuzzyRank`). `limit==1` is single-select (returns a string); any other limit is multi-select (`Tab` to mark, returns a list). The pure `fuzzyRank`/`filterWindow` are unit-tested; the model is TTY-only.
- **`render.go`** — the **non-interactive** lipgloss v2 static renderers: `starStyle` (`style`), `starTable` (`table`), `starTree` (`tree`). Unlike the huh-driven builtins these never open a TTY — they take data and return a rendered string, so they run headlessly. Colors reuse `ParseColor`; borders/alignment go through `parseBorder`/`parseAlign`; `style`'s padding/margin accept an int or list/tuple via `toIntList`.

**Script-facing surface.** `LoadModule` registers **16 builtins** in this repo's source: `write`, `input`, `select`, `multi_select`, `filter`, `confirm`, `note`, `md`, `md_note`, `spin`, `file_pick`, `colorize`, `style`, `table`, `tree`, and `set_theme` (which **overrides** the auto-generated one so it re-applies the theme immediately). `style`/`table`/`tree` are the non-interactive lipgloss renderers in `render.go`; `filter` is the fuzzy picker in `filter.go`. On top of those, `base.ConfigurableModule.LoadModule` auto-generates a `get_*`/`set_*` accessor pair for every non-secret config option: `get_width`/`set_width`, `get_height`/`set_height`, `get_theme`/`set_theme`, `get_editor`/`set_editor`. `doccov` only inspects this repo's `starlark.NewBuiltin` calls (the 16), but README/`docs/API.md` must document the accessors too.

## Invariants / hardening (preserve when editing)

1. **No host panics / clean cancellation.** `huh`'s `ErrUserAborted` and `ErrTimeout` are *not* errors to the script — `ignorableError` collapses them to `None`. Every builtin's error path routes through it; keep that so a Ctrl-C never becomes a host-crashing error and a timeout returns gracefully.
2. **Validators are sandboxed re-entrancy.** A Starlark `validate=` callback is invoked on a **fresh child `starlark.Thread`** that inherits only `Load`/`Print`/`OnMaxSteps` (`convertValidator`); a `None`/empty-string result means valid, any other value is the error message. The `spin` action callback uses the same fresh-thread pattern. Don't pass the parent thread directly.
3. **Required, non-empty inputs error cleanly.** `note` requires a non-empty `title`; `colorize`/`md` require non-empty `text`. These return script errors, never panics.
4. **Backward compatibility (iron rule).** `NewModule()` is the historical default surface (width 50, height 0, theme `charm`, empty editor). Any new config option or builtin must default to today's observable behavior — old scripts must run identically. `set_theme` must remain the gum-overriding version (immediate apply), not the plain `base` setter.

## Test organization

Group by functional goal — **do not add one `*_test.go` per fix.** `gum_test.go` holds `TestStarlarkScripts`, which drives `base.RunStarlarkTests` over `../test/gum/*.star` (the `test-*` scripts must succeed; `panic-*` scripts must fail). New behavior is verified by adding a `.star` script in the private `starpkg/test` repo (and a section in an existing thematic `*_test.go` if Go-level coverage is needed), not by minting a new test file. Tests are script/table-driven; no third-party test framework. Interactive scripts need a TTY and only run in an attended terminal.

## Documentation

Three layers must stay in sync (enforced by the doc standard, `plan/starpkg文档标准（DOC-STD）`):

- **`README.md`** — every script-facing builtin and config accessor documented as a backtick whole-word; the `doc-coverage` CI gate (`1set/meta` doccov) fails the build on any missing one. Function names, signatures, defaults, and behavior must match the code (e.g. `NewModuleWithConfig` takes **four** args; `colorize`'s color args accept name/`rgb`/`hsb`/hex).
- **GoDoc** — package comment + a doc comment whose first word is the symbol name on every exported symbol (`Module`, `NewModule`, `NewModuleWithConfig`, `LoadModule`, `ParseColor`, `ModuleName`, the `Color*` vars, `RainbowColors`), gated by `revive`'s `exported` rule in CI.
- **This `CLAUDE.md`** — the architecture/invariants map for maintainers.

## Release discipline

- **Floor = go 1.25**, following this repo's `go.mod` (raised only in its own pin PR / SEP, never incidentally). gum is the ecosystem's **go-1.25 exception**: the Charm **v2** stack (`charm.land/bubbletea/v2`, `huh/v2`, `glamour/v2`, `lipgloss/v2`) declares `go 1.25.8` (huh/glamour), forcing the floor — every other Star\* repo stays on its own lower floor. The v2 migration and the floor rise are the same PR. Because the floor (1.25) coincides with the latest stable, the CI matrix's floor leg and latest leg are both `1.25.x`.
- **CI** runs via the centralized reusable workflow in `1set/meta` (`go-ci.yml`, pinned by commit SHA), matrix `floor + latest stable`, with `doc-coverage: true` wired in `.github/workflows/build.yml`.
- **Codacy note:** the `agent-rules` analyzer currently rejects `CLAUDE.md`; that gate is being disabled org-wide, so a red Codacy check on this file is expected and not a blocker — every other check (tests, doc-coverage) must be green.
- **Bumping the version, the go floor, or tagging are user-confirmed actions** — never tag autonomously; default to patch bumps; published tags are immutable.
