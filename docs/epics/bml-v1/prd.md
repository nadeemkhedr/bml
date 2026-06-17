# PRD: bml (bookmark launcher) — v1

## Problem Statement

I keep my frequently-used URLs scattered across browser bookmarks, history, and
memory. Reaching the ones I use constantly is slow: I either hunt through
browser UI or retype URLs, and I usually end up opening a *new* tab for a site I
already have open, accumulating duplicates. I want a fast, keyboard-driven way
to jump to my most-used sites — ideally focusing the tab I already have open
instead of piling on new ones — without leaving the terminal.

## Solution

`bml` is a macOS terminal launcher for bookmarks. Launched bare, it opens a
**leader mode**: a which-key-style screen where every bookmark I've bound to a
single key is shown, and pressing that key immediately **acts on a URL** —
focusing an already-open tab if one matches, or opening a new one. Pressing the
**uppercase** of a key forces a brand-new tab instead. Pressing `/` drops into
**search mode**, a fuzzy finder over my full bookmark list (matching name, URL,
and tags) so I can reach anything, keyed or not. `bml` fires and exits, getting
out of the way.

The same tool doubles as a scriptable command: `bml g` acts on the bookmark
bound to `g`, and `bml github.com` acts on an arbitrary URL, with `-n/--new-tab`
to force a new tab. Bookmarks live in a hand-edited TOML file; `bml edit` opens
it.

All browser interaction goes through a single pluggable **browser backend**, so
v1 targets macOS Chromium browsers (Brave/Chrome/Arc/Edge) while leaving the
door open to Safari, Firefox, and other platforms later without reworking the
app.

## User Stories

1. As a user, I want to launch `bml` with no arguments, so that I land in leader
   mode immediately.
2. As a user, I want leader mode to show all my keyed bookmarks as a `key → name`
   list, so that I can see my bindings without memorizing them.
3. As a user, I want to press a single letter and have its bookmark acted on
   instantly with no Enter, so that launching feels like a leader-key shortcut.
4. As a user, I want pressing a lowercase key to focus an already-open tab for
   that URL when one exists, so that I stop accumulating duplicate tabs.
5. As a user, I want pressing a lowercase key to open a new tab when no matching
   tab is open, so that the bookmark still works the first time.
6. As a user, I want pressing the uppercase of a key to always open a new tab, so
   that I can deliberately get a fresh instance.
7. As a user, I want `bml` to fire and exit after acting, so that the browser
   comes forward and my terminal is free.
8. As a user, I want to press `/` in leader mode to enter search mode, so that I
   can reach bookmarks I haven't bound to a key.
9. As a user, I want search mode to fuzzy-match across name, URL, and tags, so
   that I can find a bookmark however I remember it.
10. As a user, I want to type to filter and see results narrow live, so that I
    can home in on the right bookmark quickly.
11. As a user, I want to select a search result with Enter and have it acted on
    (focus-or-open), so that finding and launching are one motion.
12. As a user, I want to press Esc in search mode to return to leader mode, so
    that I can back out of a search without quitting.
13. As a user, I want to quit leader mode with Esc, `q`, or Ctrl-C, so that
    exiting is obvious however my muscle memory works.
14. As a user, I want to run `bml g` from the shell, so that I can fire a keyed
    bookmark without entering the TUI.
15. As a user, I want to run `bml github.com` from the shell, so that I can act
    on an arbitrary URL with my focus-or-open behavior.
16. As a user, I want `bml -n g` / `bml --new-tab g` to force a new tab, so that
    the CLI path has the same "force new" affordance as uppercase keys.
17. As a user, I want a single-character argument treated as a bookmark key, so
    that `bml g` is unambiguous.
18. As a user, I want a multi-character argument containing a `.` treated as a
    URL, so that `bml github.com` works without a scheme.
19. As a user, I want a multi-character argument with no `.` to error clearly, so
    that a typo like `bml github` tells me what went wrong instead of guessing.
20. As a user, I want `bml g` to error clearly when no bookmark is bound to `g`,
    so that I know the key is unbound rather than silently doing nothing.
21. As a user, I want to run `bml edit`, so that I can open my bookmark file in
    `$EDITOR` without remembering its path.
22. As a user, I want my bookmarks stored in a hand-editable TOML file, so that I
    can curate them directly with comments and version control.
23. As a user, I want a bookmark to require a name and URL and optionally carry a
    key and tags, so that I can record as much or as little structure as I want.
24. As a user, I want any bookmark — keyed or not — to be reachable via search,
    so that adding a key is an optional convenience, not a requirement for
    findability.
25. As a user, I want `bml` to refuse to start and tell me which key collides
    when two bookmarks claim the same key, so that I never get surprised by which
    one wins.
26. As a user, I want `bml` to print a clear parse error and refuse to launch on
    invalid TOML, so that it never acts on a half-read config.
27. As a user, I want a starter config written for me on first run, so that I
    have a working example to edit instead of a blank file.
28. As a user, I want to choose which browser bml drives via config, so that I
    can point it at Brave, Chrome, Arc, or Edge.
29. As a user, I want to override the config location, so that I can keep my
    bookmark file wherever I prefer.
30. As a user, I want the focus-or-open behavior to match tabs forgivingly (a
    bookmark for `github.com` focuses any github.com tab), so that minor URL
    differences don't make it open a duplicate.
31. As a maintainer, I want browser automation behind one small interface, so
    that adding Safari, Firefox, or another platform later is a contained change.

## Implementation Decisions

- **Command shell — Cobra.** Root command `bml` launches the TUI when given no
  resolvable positional argument. Subcommands/usages: `bml` (TUI), `bml edit`
  (open config in `$EDITOR`), `bml <arg>` (act on a key or URL). Global flag
  `-n/--new-tab` forces a new tab on the `<arg>` path. `--config` (and
  `$BML_CONFIG`) overrides the config location.
- **Interactive UI — Bubble Tea.** Leader mode and search mode are Bubble Tea
  models. Leader mode renders a which-key-style `key → name` list of keyed
  bookmarks plus hints (`/` to search, Esc/`q` to quit). Search mode renders a
  filtered list with a query input. Keyed bookmarks only are shown in leader
  mode; unkeyed bookmarks are reachable via search.
- **Single core entity — Bookmark.** One flat list. Fields: `name` (required),
  `url` (required), `key` (optional single character), `tags` (optional list).
  A "favorite" is simply a bookmark carrying a `key`. No separate favorites
  collection.
- **Config — TOML.** Default path `~/.config/bml/bookmarks.toml`, overridable via
  `--config`/`$BML_CONFIG`. Array-of-tables, one `[[bookmark]]` block per entry.
  Loading validates: required fields present, and **duplicate `key` values are a
  hard error** that names the conflicting key and refuses to start. Invalid TOML
  is a hard parse error. On first run (no config), a commented starter file is
  written at the default path.
- **Argument resolution (`bml <arg>`).** Rule: if `arg` is exactly one
  character, treat it as a **bookmark key** (error if unbound). Otherwise, if it
  contains `.`, treat it as a **URL** (scheme optional). Otherwise, **error**.
  No name-based lookup on the CLI — names are a search-mode concern only.
- **Modifier semantics in leader mode.** Lowercase key = focus-or-open;
  uppercase key (Shift) = force new tab. Only letters get the case variant; non-
  letter keys are focus-or-open only. Ctrl is not used (terminal control-byte
  collisions).
- **Search action.** Enter = focus-or-open. No force-new variant in search mode
  in v1 (Shift+Enter is unreliable across terminals).
- **After acting — fire and exit.** Any successful act (leader, search, or CLI)
  terminates bml; the browser is brought forward by the backend.
- **Act-on-a-URL core.** A single routine backs leader, search, and CLI. It
  delegates to the **Browser backend**, passing the URL and a `forceNew` flag.
- **Browser backend interface (per ADR 0001).** One coarse method:
  `Browser.OpenOrFocus(url string, forceNew bool) error`. The backend owns its
  own focus-or-open mechanism and matching. v1 ships a single backend, **macOS
  Chromium**, which runs AppleScript via `osascript` and covers
  Brave/Chrome/Arc/Edge by parameterizing the application name (config:
  `browser`, default `Brave Browser`). Matching is **scheme-insensitive
  substring** match on open tab URLs, performed inside the AppleScript; if no tab
  matches (or `forceNew`), a new tab opens with the real stored URL. The backend
  is selected by OS for now, with room to add an explicit selector when a second
  backend exists. Adding a backend means implementing the one method and
  registering it — no changes to leader/search/CLI code.
- **Decision deviation noted:** bml reimplements the previously-existing bash/
  AppleScript open-focus script in Go rather than shelling out, for a self-
  contained binary and clean full-URL handling. See ADR 0001.

## Testing Decisions

- **What makes a good test here:** assert *external behavior*, not
  implementation. For the launcher that means: given a config and an input
  (a keypress, a query, a CLI arg), the correct `OpenOrFocus(url, forceNew)`
  call is made — or the correct error/state results. Tests must not reach into
  private rendering details or the AppleScript text.
- **Primary seam — the `Browser` interface (dependency injection).** Tests
  inject a **fake backend** that records `(url, forceNew)` calls; production
  injects the macOS Chromium backend. This is the highest-leverage seam and is
  reused by the CLI and TUI tests below.
- **CLI behavior (Cobra root command).** Drive the root command with arguments,
  an in-memory/temp config, and the fake backend; assert the recorded call or
  the error. Cases: `bml g` (key hit and unbound-key error), `bml github.com`
  (URL), `bml github` (no-dot error), `-n` forces `forceNew=true`.
- **TUI behavior (Bubble Tea `Update`).** Feed `tea.KeyMsg` sequences into the
  model's `Update` and assert resulting state and emitted actions: lowercase key
  → act with `forceNew=false`; uppercase key → `forceNew=true`; `/` → search
  mode; typing → filtered results; Enter → act on selection; Esc → return/quit.
  This is the standard Bubble Tea model-testing approach (drive `Update`, inspect
  the returned model and command).
- **Pure-function units.** Config `Load`/validation (required fields present,
  duplicate-key error, optional fields parsed), argument `Resolve` (the
  one-char / dotted / error rule), and fuzzy `Filter` over name+URL+tags.
- **Out of automated scope:** the AppleScript tab-matching itself. Per ADR 0001's
  coarse seam, matching lives inside the AppleScript and is verified by
  **manual/integration testing on macOS** against a real Chromium browser. The Go
  suite tests everything around it via the fake backend.
- **Prior art:** none yet — this is a greenfield repo. These patterns (fake
  injected via interface, Cobra command tests, Bubble Tea `Update` tests, pure
  table-driven function tests) become the prior art for later features.

## Out of Scope

- Non-macOS platforms (Linux, Windows) — designed for via the backend interface,
  not built in v1.
- Non-Chromium browsers (Safari, Firefox) — future backends.
- A force-new-tab variant inside search mode (Shift+Enter and similar are
  unreliable across terminals).
- Bookmark management beyond hand-editing + `bml edit`: no `bml add`, `bml rm`,
  `bml list` in v1.
- Name-based lookup on the CLI positional argument (names are search-only).
- Showing currently-open browser tabs inside bml (a possible later mode).
- Tag-based grouping or sectioning in leader mode (flat which-key list in v1);
  tags exist for search only.
- Multi-key leader sequences — keys are single characters.
- Sticky/multi-launch mode — bml fires and exits.
- Sync, cloud storage, or import from browser bookmarks/history.

## Further Notes

- **macOS Automation permission:** the first AppleScript invocation will trigger
  a macOS TCC prompt to control the browser. bml should detect and surface a
  clear "automation permission denied / not yet granted" error rather than
  failing opaquely.
- **Tags carry forward without display:** `tags` is in the schema and used by
  search, but has no leader-mode UI in v1. It's intentionally there to seed
  later grouping features.
- **Glossary & decisions:** terminology is defined in `CONTEXT.md`; the
  browser-automation architecture is recorded in
  `docs/adr/0001-reimplement-browser-automation-in-go.md`. Use that vocabulary
  (leader mode, search mode, bookmark, favorite, act on a URL, browser backend)
  consistently in implementation.
