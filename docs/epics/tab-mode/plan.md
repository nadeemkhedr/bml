# Plan: Tab mode

> Source PRD: `docs/epics/tab-mode/prd.md` (see also ADR 0005 and the CONTEXT.md glossary)

## Architectural decisions

Durable decisions that apply across all phases:

- **Entry point**: the **Tab key** (a non-printable key event) at the **top level**
  of leader mode enters tab mode; ignored inside a group. Costs no bookmark
  keyspace (unlike reserved `s`/`/`). The launcher footer advertises it.
- **Seam**: tab listing is a *separate optional* interface — `TabLister { ListTabs() ([]Tab, error) }`
  — not a widening of the core `Browser` seam. Tab mode obtains it by
  type-asserting the injected browser. The macOS Chromium backend implements both.
- **Key model**: an **open tab** is `Tab{ Title, URL }` — ephemeral browser state,
  flat across all windows, never persisted; distinct from a stored **bookmark**.
- **Focusing**: reuses the existing `OpenOrFocus(url, false)` path with the tab's
  full URL — no new focusing capability. Accepts substring near-duplicate
  ambiguity as a v1 limitation.
- **List script**: a *second*, read-only AppleScript distinct from the focus
  script — **no `activate`** (never steal focus), **guarded by `is running`**
  (never launch a closed browser). Fetched asynchronously on mode entry (a
  `tea.Cmd`, like `act`), results delivered via a load message.
- **Display**: single-line `Title (friendly url)` — title bold, friendly-url
  faint. "Friendly URL" strips scheme, leading `www.`, trailing `/`; full URL
  retained internally. Blank title → friendly-URL-only fallback.
- **Interaction**: mirrors bookmarks mode — fire-and-exit on Enter, Esc returns a
  `Leader`, Ctrl-C quits, scrolling selection window.
- **Test seams**: pure `buildListScript(app)` (string assertions) and
  `parseTabs(output)` (canned-output parse) in the browser package; `browser.Fake`
  implements `TabLister` to drive the model in-memory. No new test *style*.

---

## Phase 1: Tracer bullet — list open tabs and focus one

**User stories**: 1, 2, 5, 6, 7, 8, 9, 15, 17, 18, 19

### What to build

The minimal complete path end-to-end. In the browser package: the `Tab` type, the
`TabLister` interface, and the Chromium backend's `ListTabs` — built from a pure
read-only `buildListScript(app)` (iterates windows/tabs emitting title + URL, with
**no `activate`**) and a pure `parseTabs(output)` that turns osascript stdout into
`[]Tab`. Extend `browser.Fake` to implement `TabLister` (a canned `Tabs` field).

In the tui package: wire the **Tab key** at the top level of leader mode (ignored
inside a group) to construct a new tab-mode model carrying the current terminal
size; add the footer hint. The model fetches the tab list asynchronously on entry
and renders a **plain** (unfiltered) single-line `Title (friendly url)` list —
title bold, friendly URL faint, blank-title rows falling back to the URL. ↑↓ (and
Ctrl-P/Ctrl-N) move a scrolling selection; Enter focuses the selected tab via
`OpenOrFocus(url, false)` and exits; Esc returns to the launcher; Ctrl-C quits.

### Acceptance criteria

- [ ] Pressing Tab at the top level of leader mode enters tab mode; pressing Tab
      inside a group does nothing.
- [ ] `buildListScript` targets the configured app, iterates windows/tabs emitting
      title and URL, and **omits `activate`**.
- [ ] `parseTabs` converts canned osascript output into `[]Tab`, with a
      blank/missing title falling back to the URL.
- [ ] Open tabs render single-line as `Title (friendly url)` (scheme / leading
      `www.` / trailing `/` stripped), title bold and friendly URL faint.
- [ ] ↑↓ / Ctrl-P / Ctrl-N move the selection within a scrolling window.
- [ ] Enter on the selected tab calls `OpenOrFocus(<full URL>, false)` (focus, not
      force-new) and exits.
- [ ] Esc returns a `Leader`; Ctrl-C quits.
- [ ] The launcher footer advertises the Tab key for tabs.
- [ ] Model tests drive the flow via an injected `Fake` (no real browser);
      `buildListScript`/`parseTabs` are tested as pure functions.

---

## Phase 2: Fuzzy filter + match highlighting

**User stories**: 3, 4, 13

### What to build

Turn the plain list into the full switcher. Generalize the shared fuzzy matcher so
it ranks and highlights over **open tabs** (matching title + friendly URL) as well
as bookmarks. Add the live text input above the list; typing filters results in
real time and highlights the matched characters using the existing row/highlight
styles. Add the "no matches" empty state when a query excludes everything. Enter
acts on the selected *filtered* result.

### Acceptance criteria

- [ ] Typing filters the open-tab list live, matching over title and friendly URL.
- [ ] Matched characters are highlighted in the results, consistent with bookmarks
      mode.
- [ ] A query that matches nothing shows "no matches".
- [ ] Enter focuses the currently selected filtered tab; Enter on an empty result
      set is a no-op.
- [ ] Filter behavior over tabs is covered by matcher tests mirroring the existing
      filter tests.

---

## Phase 3: Robust states

**User stories**: 10, 11, 12, 14, 20

### What to build

Render the remaining degenerate states distinctly. While the async fetch is in
flight, show `loading tabs…`. Add the `is running` guard to the list script so a
closed browser yields an empty list surfaced as a **browser-not-running** state
(never launching it). Distinguish **running-but-zero-tabs** ("no open tabs") from
not-running. Map the osascript automation denial to the existing
`ErrAutomationDenied` guidance, surfaced the way act errors are. Make the Tab key a
silent no-op when the injected browser does not implement `TabLister`.

### Acceptance criteria

- [ ] Between entry and results, the model shows `loading tabs…`.
- [ ] When the browser isn't running, tab mode shows a "browser isn't running"
      state and does **not** launch the browser (`is running` guard present in the
      script).
- [ ] When the browser is running with no tabs, the model shows "no open tabs"
      (distinct from not-running).
- [ ] An automation-permission denial surfaces the existing System Settings →
      Privacy & Security → Automation guidance.
- [ ] Pressing Tab when the backend does not implement `TabLister` does nothing —
      no error, no UI.
- [ ] Enter while loading or on an empty list is a no-op.
- [ ] Each state is covered by a model test using the injected `Fake` (including
      its error path).
