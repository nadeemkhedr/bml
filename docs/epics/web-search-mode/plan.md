# Plan: Search mode (web search)

> Source PRD: `docs/epics/web-search-mode/prd.md`

## Architectural decisions

Durable decisions that apply across all phases (this is a TUI/CLI tool — no routes
or database):

- **Engine model**: a **search engine** is a named URL template containing a
  `{{input}}` placeholder. Substitution is `url.QueryEscape(query)` into
  `{{input}}` (spaces → `+`, specials percent-encoded). Exposed as a pure method
  so it is testable without a TUI or browser.
- **Built-in engines** (see ADR 0003):
  - `google` → `https://www.google.com/search?q={{input}}`
  - `duckduckgo` → `https://duckduckgo.com/?q={{input}}`
  - `duckduckgo_lucky` → `https://duckduckgo.com/?q=!ducky+{{input}}`
  - No per-engine "lucky" variant — `duckduckgo_lucky` is its own engine.
- **Key bindings**: **Enter → primary engine**, **Tab → secondary engine** (Tab,
  not Shift+Enter — Bubble Tea v1.3.10 can't distinguish them; ADR 0003). Esc →
  leader mode, Ctrl+C → quit, empty query → no-op.
- **Dispatch**: always `OpenOrFocus(url, forceNew=true)` (new tab, never
  focus-or-open), then fire-and-exit via the existing `actedMsg` path.
- **Config schema** (all optional; defaults Google primary / `duckduckgo_lucky`
  secondary):
  ```toml
  [search]
  default_engine   = "google"
  secondary_engine = "duckduckgo_lucky"

  [search.engines]
  name = "url-template-with-{{input}}"
  ```
  `default_engine`/`secondary_engine` reference any engine by name; user engines
  merge over built-ins (same-name override wins).
- **Module boundaries**: the web-search TUI model lives in its own model/file,
  separate from the existing bookmarks-mode finder (CONTEXT.md: `/` = **bookmarks
  mode**, `s` = **search mode**). Engine registry, parsing, and validation live in
  the `config` package. Dispatch reuses the existing `browser.Browser` interface
  and `act` command.
- **Testing seams**: `browser.Fake` (records `OpenOrFocus` calls + `forceNew`);
  TUI models driven by feeding `tea.KeyMsg` into `Update`; `config` tested as pure
  parse/validate functions. Prior art: `leader_test.go`, `search_test.go`,
  `config_test.go`.

---

## Phase 1: Tracer — `s` opens a Google search, end-to-end

**User stories**: 1, 2, 3, 4, 5, 8, 9, 10, 11, 12, 23, 24, 25, 28

### What to build

The thinnest complete path through every layer. Introduce the `Engine` type with a
pure `URL(query)` substitution method, seeded with a single hardcoded `google`
engine. Add a new web-search TUI model holding a text input and the injected
browser. Pressing `s` at the top level of leader mode (prefix empty; ignored inside
a group) swaps to the search model, mirroring the existing `/` → bookmarks-mode
transition. Typing builds the query; Enter dispatches the URL-escaped Google search
in a new tab and fires-and-exits. Esc returns to leader mode, Ctrl+C quits, and an
empty query does nothing. The view is visually distinct from bookmarks mode and
shows a footer of controls.

### Acceptance criteria

- [ ] Pressing `s` at the top level of leader mode enters the web-search model; pressing `s` while inside a group does not.
- [ ] Typing characters accumulates a query that is visible in the view.
- [ ] Enter on a non-empty query acts on `https://www.google.com/search?q=<escaped-query>` with `forceNew == true`, then quits (fire-and-exit).
- [ ] A query with spaces and special characters (e.g. `c++ a & b`) is correctly URL-escaped in the dispatched URL.
- [ ] Enter with an empty query produces no act command and records no browser call.
- [ ] Esc returns a `Leader` model; Ctrl+C produces a `QuitMsg`.
- [ ] `Engine.URL(query)` is unit-tested directly (escaping, `{{input}}` substitution) without a TUI or browser.
- [ ] The search view is distinguishable from bookmarks mode and shows a footer with the available keys.

---

## Phase 2: Secondary engine via Tab + built-in registry

**User stories**: 6, 7, 14 (default behavior), 23 (Tab label)

### What to build

Promote the single engine to the three built-in engines and give the model a
primary and a secondary engine slot. Default primary stays `google`; default
secondary is `duckduckgo_lucky`. Bind **Tab → secondary engine**, so Tab dispatches
the `!ducky` lucky URL while Enter still dispatches the primary. Engines remain
hardcoded defaults — no config parsing yet. The footer names both engines so the
user knows what Enter and Tab will do.

### Acceptance criteria

- [ ] All three built-in engines exist with the exact templates from the architectural decisions.
- [ ] Tab on a non-empty query acts on `https://duckduckgo.com/?q=!ducky+<escaped-query>` with `forceNew == true`, then quits.
- [ ] Enter continues to dispatch the primary engine (`google`) as in Phase 1.
- [ ] Tab with an empty query produces no act command and records no browser call.
- [ ] The footer shows both the primary (Enter) and secondary (Tab) engine names.
- [ ] Model tests cover both Enter→primary and Tab→secondary dispatch.

---

## Phase 3: Configurable engines (`[search]` + `[search.engines]`)

**User stories**: 13, 14, 15, 16, 17, 18, 26, 27

### What to build

Parse a `[search]` table (`default_engine`, `secondary_engine`) and a
`[search.engines]` map of `name = "template"` from the config file. Build the
engine registry by merging the built-ins with user-defined engines, where a
user engine of the same name overrides the built-in. Resolve the primary and
secondary slots by name and pass the resolved engines into the web-search model.
When `[search]` is absent, fall back to `google` / `duckduckgo_lucky`. Existing
configs with no search settings keep working unchanged. Add commented `[search]` /
`[search.engines]` examples to the starter config so the feature is discoverable on
first run.

### Acceptance criteria

- [ ] `default_engine` and `secondary_engine` select which engines Enter and Tab dispatch to.
- [ ] Custom engines defined under `[search.engines]` are usable by name for either slot.
- [ ] A custom engine whose name matches a built-in overrides the built-in template.
- [ ] A config with no `[search]` table resolves to primary `google` and secondary `duckduckgo_lucky`.
- [ ] An existing bookmark-only config loads and runs unchanged (round-trip preserved).
- [ ] The starter config includes commented examples of `[search]` and a custom engine.
- [ ] Config tests cover built-in presence, custom merge, same-name override, slot resolution, and the no-`[search]` defaults.

---

## Phase 4: Validation hard errors + `s`-key reservation

**User stories**: 19, 20, 21, 22

### What to build

Add hard errors at config load, consistent with how bad/duplicate leader keys are
already rejected. Reject a `default_engine`/`secondary_engine` that names an unknown
engine; reject a custom engine template that lacks `{{input}}`; and reject any
leader key whose first character is `s`, with a message explaining that `s` is
reserved for entering search mode (so an `s…` key would be unreachable). Each error
prevents launch with a clear, actionable message.

### Acceptance criteria

- [ ] A `default_engine` or `secondary_engine` naming an unknown engine fails to load with a message naming the offending value.
- [ ] A custom engine whose template omits `{{input}}` fails to load with a message naming the offending engine.
- [ ] Any bookmark leader key beginning with `s` (e.g. `s`, `sg`, `sw`) fails to load with a message explaining `s` is reserved for search mode.
- [ ] Valid configs (including those with no `[search]` table) continue to load without error.
- [ ] Config tests cover each rejection path and at least one valid-config control.
