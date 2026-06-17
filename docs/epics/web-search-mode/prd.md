# PRD: Search mode (web search)

## Problem Statement

When I'm in bml's launcher, I can only reach things I've already bookmarked.
Constantly, what I actually want is to search the web for something I *haven't*
bookmarked — a quick Google, or a DuckDuckGo "I'm feeling lucky" jump straight to
the top result. Today I have to leave bml, switch to the browser, click the
address bar, and type. The launcher is right there under my fingers but can't do
the one thing I reach for most.

## Solution

A new **search mode**, entered from leader mode by pressing `s`. I type a free-text
**query**, press Enter, and bml opens a web search for it in a new tab — then
exits, just like acting on a bookmark. The engine is configurable: Enter uses my
**primary engine** (default Google); Tab uses my **secondary engine** (default
DuckDuckGo's `!ducky` "lucky" jump). Three engines ship built in (`google`,
`duckduckgo`, `duckduckgo_lucky`) and I can define my own in config as named URL
templates. Because `s` now means "search," it's reserved: no leader key may begin
with `s`.

## User Stories

1. As a launcher user, I want to press `s` from leader mode to start a web search, so that I can look something up without leaving bml.
2. As a launcher user, I want to type a free-text query in search mode, so that I can search for anything, not just my bookmarks.
3. As a launcher user, I want to press Enter to search with my primary engine, so that the common case is a single keystroke after typing.
4. As a launcher user, I want my search results to open in a new browser tab, so that I don't disturb whatever tab I'm currently looking at.
5. As a launcher user, I want bml to exit after dispatching my search, so that the launcher behaves consistently with acting on a bookmark (fire and exit).
6. As a launcher user, I want to press Tab to search with my secondary engine, so that I can trigger an alternate action (e.g. "I'm feeling lucky") without retyping.
7. As a DuckDuckGo user, I want the `duckduckgo_lucky` engine to use the `!ducky` bang, so that I jump straight to the first result instead of a results page.
8. As a user mid-query, I want pressing Esc to return me to leader mode, so that I can back out of a search I no longer want.
9. As a user mid-query, I want pressing Ctrl+C to quit bml entirely, so that quitting works the same everywhere in the app.
10. As a careful typist, I want an empty query with Enter or Tab to do nothing, so that I don't open a blank search by accident.
11. As a user with special characters in my query, I want them correctly URL-escaped, so that searching `c++ templates` or `a & b` reaches the right results page.
12. As a user, I want spaces in my query handled correctly, so that a multi-word search like `golang context cancellation` works as one query.
13. As a config owner, I want to set `default_engine` in config, so that Enter searches with the engine I prefer (e.g. switch the default from Google to DuckDuckGo).
14. As a config owner, I want to set `secondary_engine` in config, so that Tab triggers whichever engine I choose, not just the built-in default.
15. As a power user, I want to define my own engines under `[search.engines]` as `name = "url-template"`, so that I can search Kagi, YouTube, GitHub, or any site with a query URL.
16. As a power user, I want my custom engine to override a built-in of the same name, so that I can, for example, point `google` at a regional or privacy-preserving variant.
17. As a config owner, I want to reference any engine (built-in or custom) by name for the primary/secondary slots, so that selection and definition use one consistent vocabulary.
18. As a config owner with no `[search]` table, I want sensible defaults (Google primary, DuckDuckGo-lucky secondary), so that the feature works out of the box without configuration.
19. As a config owner who typos an engine name, I want bml to refuse to launch with a clear message, so that I find out immediately rather than getting silent wrong behavior.
20. As a config owner defining a custom engine, I want bml to reject a template that lacks `{{input}}`, so that I can't accidentally create an engine that drops my query.
21. As a leader-mode user, I want bml to reject any leader key beginning with `s`, so that I'm warned rather than silently creating a key I can never reach.
22. As a leader-mode user, I want a clear error explaining *why* an `s`-prefixed key is rejected, so that I understand `s` is reserved for search mode.
23. As a user in search mode, I want to see which engines Enter and Tab will use, so that I know what each key does before I press it.
24. As a user in search mode, I want a visible footer of available keys (Enter, Tab, Esc), so that the controls are discoverable.
25. As a user who pressed `s` by mistake, I want search mode to look distinct from bookmarks mode, so that I immediately realize where I am.
26. As an existing user, I want my current bookmarks config to keep working untouched, so that adding search mode doesn't force me to change anything.
27. As a first-run user, I want the starter config to document the `[search]` options as comments, so that I can discover and enable custom engines without reading external docs.
28. As a leader-mode user, I want `s` to only trigger search at the top level (not while navigating inside a group), so that group navigation isn't hijacked.

## Implementation Decisions

**Modules built/modified**

- **`config` package** — extended to parse a `[search]` table (`default_engine`,
  `secondary_engine`) and a `[search.engines]` map of `name = "template"`. Owns
  the **engine registry**: the three built-ins merged with user-defined engines
  (same-name override wins), plus resolution of the primary/secondary slots.
  Validation lives here.
- **New search-mode TUI model** — a Bubble Tea model for web search, kept
  separate from the existing bookmarks-mode finder (the file currently named for
  "search" backs **bookmarks mode**; the new model gets its own file to avoid
  the naming collision resolved in CONTEXT.md). Holds a `textinput`, the resolved
  primary/secondary engines, and the injected `browser.Browser`.
- **Leader model** — pressing `s` at the top level (`prefix == ""`) swaps to the
  new search model, mirroring the existing `/` → bookmarks-mode transition;
  inside a group, `s` is ignored.
- **`Engine` type** — a named URL template with a pure `URL(query)` method that
  substitutes `url.QueryEscape(query)` into the `{{input}}` placeholder.

**Engine model (see ADR 0003)**

- An **engine** is just a named URL template containing `{{input}}`. There is no
  per-engine "lucky" variant — `duckduckgo_lucky` is its own engine, because no
  reliable URL-template "lucky" exists for Google.
- Built-ins:
  - `google` → `https://www.google.com/search?q={{input}}`
  - `duckduckgo` → `https://duckduckgo.com/?q={{input}}`
  - `duckduckgo_lucky` → `https://duckduckgo.com/?q=!ducky+{{input}}`

**Actions / keys (see ADR 0003)**

- **Enter → primary engine**, **Tab → secondary engine**. Tab — not Shift+Enter —
  because Bubble Tea v1.3.10 cannot distinguish Shift+Enter from Enter (no
  `KeyShiftEnter`, no `Shift` modifier, no kitty keyboard protocol).
- Dispatch always opens a **new tab** (`OpenOrFocus(url, forceNew=true)`); never
  focus-or-open. After dispatch the program fires-and-exits via the existing
  `actedMsg` path.
- **Esc** returns to leader mode; **Ctrl+C** quits; an **empty query** is a no-op.

**Config contract**

```toml
[search]
default_engine   = "google"            # Enter
secondary_engine = "duckduckgo_lucky"  # Tab

[search.engines]
kagi = "https://kagi.com/search?q={{input}}"
```

- No `[search]` table → primary `google`, secondary `duckduckgo_lucky`.
- `default_engine` / `secondary_engine` reference any engine by name (built-in or
  custom).

**Validation (hard errors at config load, consistent with existing key validation)**

- A leader key beginning with `s` is rejected (any `s…` key would be unreachable).
- `default_engine` / `secondary_engine` naming an unknown engine is rejected.
- A custom engine template missing `{{input}}` is rejected.

**Query handling**

- The query is `url.QueryEscape`d before substitution, so spaces become `+` and
  special characters are percent-encoded. Example: query `foo bar` against
  `duckduckgo_lucky` yields `https://duckduckgo.com/?q=!ducky+foo+bar`.

## Testing Decisions

Good tests here assert **external behavior** through existing seams, never
internal model fields beyond what the existing tests already touch. The spine is
`browser.Fake`, which records `OpenOrFocus(url, forceNew)` calls; TUI models are
driven by feeding `tea.KeyMsg` values into `Update` and asserting the resulting
act command and model transition. This mirrors the current `leader_test.go`,
`search_test.go`, and `config_test.go`.

**Modules tested and prior art**

- **`Engine.URL(query)` (new, highest seam for the escaping rule)** — pure unit
  tests: `{{input}}` substituted with the URL-escaped query; spaces → `+`;
  special characters percent-encoded; the `!ducky+{{input}}` template produces
  the expected lucky URL. No TUI or browser needed.
- **`config` parse/validate** — prior art: `config_test.go`. Assert built-ins are
  present; `[search.engines]` entries merge in; a same-name custom engine
  overrides the built-in; `default_engine`/`secondary_engine` resolve to the
  right engine; an unknown engine name errors; a custom template missing
  `{{input}}` errors; **any leader key starting with `s` errors**; and a config
  with no `[search]` table yields the documented defaults.
- **Web-search TUI model** — prior art: `search_test.go`. Type a query via
  `KeyRunes`, then assert: `KeyEnter` dispatches the primary engine's URL with
  `ForceNew == true`; `KeyTab` dispatches the secondary engine's URL; an empty
  query + Enter/Tab produces no act and no call; `KeyEsc` returns a `Leader`
  model; `KeyCtrlC` produces a `QuitMsg`.
- **Leader → search transition** — prior art: `TestSearch_EscReturnsToLeader` and
  the `/` handling in `leader_test.go`. Assert `s` at the top level returns the
  web-search model type, and `s` inside a group does not.

## Out of Scope

- **Literal Shift+Enter** for the secondary action (blocked by Bubble Tea v1;
  revisit if the project upgrades to v2 — see ADR 0003).
- **A reliable Google "I'm feeling lucky"** engine (Google's `btnI` redirect is
  broken; only DuckDuckGo's `!ducky` is offered).
- **Search history, autocomplete, or query suggestions.**
- **More than two action slots** (only primary/Enter and secondary/Tab).
- **Saving a search as a bookmark**, or any cross-over with bookmarks mode.
- **Per-engine focus-or-open behavior** — searches always open a new tab.
- **Renaming the existing bookmarks mode's file/identifiers** beyond what's needed
  to avoid the new model colliding with it (the CONTEXT.md term rename to
  "bookmarks mode" is documentation; code-symbol renames are not required by this
  PRD).

## Further Notes

- Terminology is settled in `CONTEXT.md`: the `/` fuzzy bookmark finder is now
  **bookmarks mode**; **search mode** is this web-search feature; **search
  engine**, **query**, and **primary/secondary engine** are defined there.
- The two surprising decisions (lucky-as-its-own-engine and Tab-instead-of-Shift+Enter)
  are recorded in `docs/adr/0003-search-mode-engine-model.md`.
- The starter config (`config.go`'s `starter` constant) should gain commented
  `[search]` / `[search.engines]` examples so the feature is discoverable on
  first run.
