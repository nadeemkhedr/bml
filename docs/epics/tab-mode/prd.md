# Tab mode — PRD

## Problem Statement

I keep dozens of tabs open in my browser, and finding the one I want is slow:
I either squint across a crowded tab strip or alt-tab hunt through windows. bml
already knows how to *focus an open tab* for a known bookmark, but it can only
reach tabs I've curated as bookmarks. The tabs I actually have open right now —
the half-read article, the three pull requests, the doc I opened an hour ago —
are invisible to bml unless I happened to bookmark them. I want the same fast,
keyboard-driven "type a few letters, jump to it" experience bml gives my
bookmarks, but pointed at my *live* open tabs.

## Solution

A new **tab mode**, entered from leader mode by pressing the **Tab key**. It
lists the open tabs of the configured browser and lets me fuzzy-filter them by
typing — matching over each tab's title and URL, exactly like bookmarks mode.
Selecting a tab focuses it in the browser and exits. It is a pure switcher: it
only ever *focuses* an existing tab; it never opens, closes, or rearranges tabs.

The experience parallels the two finders bml already has: **bookmarks mode**
fuzzy-finds over *stored* URLs and acts on the pick; **tab mode** fuzzy-finds
over *live* tabs and focuses the pick. Same skeleton, different source.

## User Stories

1. As a bml user with many tabs open, I want to press the Tab key from the
   launcher and see a list of my open tabs, so that I can switch to one without
   hunting through the browser's tab strip.
2. As a bml user, I want each open tab shown as its title with a friendly URL
   beside it, so that I can recognize the tab at a glance rather than parsing a
   raw address.
3. As a bml user, I want to type a few characters to fuzzy-filter the open-tab
   list by title or URL, so that I can narrow 40 tabs down to the one I want in a
   couple of keystrokes.
4. As a bml user, I want the matched characters highlighted in the filtered
   results, so that I can see why a tab matched my query — consistent with
   bookmarks mode.
5. As a bml user, I want to move the selection with the arrow keys (and Ctrl-P /
   Ctrl-N), so that I can pick a tab using the same keys as bookmarks mode.
6. As a bml user, I want pressing Enter on the selected tab to focus that tab in
   my browser and exit bml, so that I land on the page immediately with no
   further steps.
7. As a bml user, I want tab mode to bring the browser to the foreground only
   when I actually select a tab — not when I merely open the mode — so that
   opening the switcher never steals my focus away from the terminal.
8. As a bml user, I want pressing Esc in tab mode to return me to the launcher,
   so that I can back out and use a different mode without quitting bml.
9. As a bml user, I want Ctrl-C to quit from tab mode, so that quitting works the
   same everywhere in bml.
10. As a bml user whose browser isn't running, I want tab mode to tell me the
    browser isn't running rather than launching it, so that opening the switcher
    never boots a browser I deliberately closed.
11. As a bml user, I want a brief "loading tabs…" indication while the list is
    being fetched, so that an empty screen doesn't look like a bug.
12. As a bml user with a browser open but no tabs, I want a clear "no open tabs"
    message, so that I understand the empty list is real and not an error.
13. As a bml user whose filter matches nothing, I want a "no matches" message, so
    that I know my query excluded everything — consistent with bookmarks mode.
14. As a bml user who hasn't granted Automation permission, I want tab mode to
    show the same friendly "grant access under System Settings → Privacy &
    Security → Automation" guidance bml already gives, so that I know how to fix
    it.
15. As a bml user, I want the Tab key to enter tab mode without costing me a
    bookmark key, so that I keep every letter available for my own leader keys.
16. As a bml user with two tabs open on similar URLs, I accept that focusing may
    occasionally land on the near-duplicate, so that the feature stays simple;
    this is a known v1 limitation, not a defect I should report.
17. As a bml user, I want the footer of the launcher to advertise the Tab key for
    tabs, so that the feature is discoverable.
18. As a bml user, I want tab mode reachable only from the top level of leader
    mode (not from inside a group), so that the Tab key is unambiguous while
    navigating key sequences.
19. As a bml user, I want a tab with a blank or missing title to fall back to
    showing its URL, so that no row is ever unidentifiable.
20. As a bml user on a backend that can't enumerate tabs, I want the Tab key to
    simply do nothing rather than error, so that the feature degrades quietly
    where it isn't supported.

## Implementation Decisions

**New domain entities and modes (see CONTEXT.md).** An **open tab** is a live tab
in the configured browser, captured as a `{Title, URL}` pair — ephemeral browser
state, never persisted, distinct from a stored **bookmark**. **Tab mode** is the
mode that lists and focuses open tabs.

**`TabLister` is a separate optional capability interface (ADR 0005).** The core
`Browser` seam (`OpenOrFocus`) stays coarse and unchanged. Tab listing is exposed
as a distinct interface — `TabLister { ListTabs() ([]Tab, error) }` — that a
backend may additionally implement. The macOS Chromium backend implements both;
future backends that cannot enumerate tabs simply omit it. Tab mode obtains the
lister by type-asserting the injected browser for `TabLister`.

**Focusing reuses `OpenOrFocus`, not positional identity (ADR 0005).** Selecting
a tab passes that tab's full URL to the existing `OpenOrFocus(url, false)` path.
No new focusing capability is added — the only new capability is *listing*.
Consequence: focusing matches by scheme-insensitive substring and may resolve to
the wrong tab when two open tabs share a URL prefix. This is an accepted v1
limitation.

**The list AppleScript is read-only and side-effect-free (ADR 0005).** Listing
runs a *second* script, distinct from the focus script. It must **not**
`activate` the browser (opening the mode must never steal focus), and it must
**not** launch a closed browser — it guards on `application "X" is running` and
yields an empty list when the browser is not running. The script iterates windows
and tabs emitting each tab's title and URL.

**Listing is fetched asynchronously.** On entering tab mode, listing runs as a
Bubble Tea command (the same deferral pattern as `act`), delivering results via a
`tabsLoadedMsg` (carrying tabs or an error). Until it arrives, the model renders a
`loading tabs…` state.

**Entry point is the Tab key at the top level of leader mode.** Tab arrives as a
non-printable key event, so it costs no bookmark keyspace (unlike the reserved
`s` and `/`). It is handled only at the top level — ignored inside a group.
Selecting it constructs the tab-mode model carrying the current terminal size
(models are swapped without a resize event, matching how bookmarks/search mode are
entered). The launcher footer advertises the Tab key.

**Display.** Each tab renders on a single line as `Title (friendly url)` — title
emphasized (bold), the parenthesized friendly URL faint. "Friendly URL" strips
the scheme, a leading `www.`, and a trailing `/`; the full URL is retained
internally for focusing. A blank/missing title falls back to showing the friendly
URL alone (no empty parens). Single-line rows (vs. bookmarks mode's two-line rows)
fit more tabs on screen; over-long rows truncate.

**Interaction model (mirrors bookmarks mode).** A text input filters live; the
shared fuzzy matcher is generalized to match tabs over title + friendly URL and
to drive match-highlighting. Arrow keys (and Ctrl-P/Ctrl-N) move the selection
within a scrolling window; Enter focuses the selected tab and exits (fire and
exit); Esc returns to leader mode; Ctrl-C quits. Enter is a no-op while loading or
when the result set is empty.

**States.** The model renders five distinct states: *loading*, *browser not
running*, *running but zero tabs*, *filter matched nothing* ("no matches"), and
*automation denied* (reusing the existing `ErrAutomationDenied` guidance, surfaced
the way act errors are). A backend that does not implement `TabLister` makes the
Tab key a silent no-op — no error, no UI — since today's only backend supports it
and that branch is currently unreachable.

**Modules built/modified.**
- `internal/browser`: add the `TabLister` interface and the `Tab` type; implement
  `ListTabs` on the Chromium backend via a new pure `buildListScript(app)` plus a
  pure `parseTabs(output)`; extend `browser.Fake` to implement `TabLister`.
- `internal/tui`: add the tab-mode model (Bubble Tea), wire the Tab key in leader
  mode and add the footer hint, generalize the fuzzy filter to cover tabs, and
  reuse the existing row/highlight rendering styles.

## Testing Decisions

**What makes a good test here:** it asserts externally observable behavior — the
AppleScript text produced, the `[]Tab` parsed from canned output, which URL the
model acts on, and what the model renders — never private fields or call
sequencing. Tests inject the `Fake` (no real browser, no `osascript`) and drive
the model through `Update` with synthetic key messages, exactly as the existing
suite does.

**Modules and their tests:**

- **`buildListScript` (browser, string-assertion seam).** Mirrors
  `TestBuildScript_*` in `chromium_test.go`: assert the script targets the
  configured app, iterates windows/tabs, emits title and URL, **omits `activate`**,
  and **includes the `is running` guard**.
- **`parseTabs` (browser, pure-parse seam).** Table-driven over canned osascript
  output: empty output → no tabs; one tab; many tabs; a tab with a blank title →
  URL-fallback representation. No browser involved. Prior art: `TestSchemeless`,
  `TestWithScheme`, `TestEscapeAppleScript`.
- **Tab-mode model (tui, `Fake`-injection seam).** Mirrors `websearch_test.go`:
  inject a `Fake` returning canned tabs, then assert — Enter on the selection
  calls `OpenOrFocus(selectedURL, false)` (focus, not force-new); typing narrows
  the result set and highlights matches; Esc returns a `Leader`; Ctrl-C quits;
  Enter while loading or on an empty list is a no-op; the not-running, zero-tabs,
  no-matches, and automation-denied states render their expected content. Prior
  art for the leader wiring: `TestLeader_SEntersWebSearch` and
  `TestLeader_SInsideGroupDoesNotEnterWebSearch` — add the analogues for the Tab
  key entering tab mode at the top level and being ignored inside a group.
- **Generalized filter (tui).** Extend `filter_test.go`-style cases to cover
  matching over tabs (title + friendly URL), so tab filtering shares the
  bookmarks-mode matcher's coverage.

## Out of Scope

- Any tab action other than focus: closing, opening, reloading, moving, or
  reordering tabs. v1 is a pure switcher (ADR 0005).
- Positional (window+index) tab identity and the disambiguation of two open tabs
  sharing a URL prefix — explicitly accepted as a v1 limitation.
- Showing or grouping by browser window; the list is flat across all windows.
- Non-Chromium and non-macOS backends (Safari, Firefox, Linux, Windows). They may
  implement `TabLister` later without touching tab-mode UI code.
- Persisting, bookmarking, or otherwise capturing open tabs; open tabs are read on
  demand and never stored.
- Configuration for tab mode (filtering rules, display format, key rebinding).
- Live-refreshing the list while the mode is open; the snapshot is taken once on
  entry.

## Further Notes

- **Terminology:** "Tab" is deliberately overloaded — the **Tab key** enters tab
  mode at the leader level, while in **search mode** Tab dispatches the secondary
  engine. Different modes, no collision; recorded in CONTEXT.md so it reads as
  intentional.
- **Terminal reality:** "smaller font" for the URL is not achievable in a TUI;
  faint/dim styling stands in for it.
- The relevant decisions are recorded in `docs/adr/0005-tab-mode-and-tablister-capability.md`
  and the glossary entries (*open tab*, *tab mode*, *tab listing*) in CONTEXT.md.
