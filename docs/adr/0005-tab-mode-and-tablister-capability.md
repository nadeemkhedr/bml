# Tab mode and the TabLister capability seam

Tab mode (entered with the **Tab key** from leader mode) lists the configured
browser's currently open tabs, fuzzy-filters them over title + URL like bookmarks
mode, and focuses the selected one before exiting. It is a pure switcher in v1:
read-only, no opening/closing/rearranging. Three sub-decisions are worth
recording because each took a non-obvious path.

## Tab listing is a separate optional interface, not a wider `Browser`

ADR 0001 fought to keep the browser seam a single coarse method
(`OpenOrFocus`). Enumerating tabs is a different shape — it returns data rather
than performing a side effect — so it cannot fold into `OpenOrFocus`. Rather than
widen `Browser` (which would force every future backend — Safari, Firefox, Linux,
Windows — to implement enumeration even when it can't or won't), we added a
separate optional interface:

```go
type TabLister interface { ListTabs() ([]Tab, error) }
```

The macOS Chromium backend implements both `Browser` and `TabLister`. Tab mode
type-asserts for `TabLister` at the entry point and is simply unavailable on a
backend that doesn't implement it (today that branch is unreachable — the only
backend is a `TabLister` — so the assertion just no-ops on failure). This keeps
the core seam minimal and makes "can this browser enumerate tabs?" an explicit,
per-backend capability rather than a method everyone must stub.

## Focusing reuses `OpenOrFocus`, accepting URL re-match ambiguity

Selecting a tab does **not** focus it by positional identity (window + tab
index). Instead the tab's full URL is passed to the existing
`OpenOrFocus(url, false)` path. This means the new capability is *only* listing —
focusing is free, reusing code already shipped, and staying truer to the
one-coarse-method philosophy.

The cost: `OpenOrFocus` matches by scheme-insensitive **substring** and focuses
the first match, so two open tabs sharing a URL prefix (e.g. `…/pull/1` and
`…/pull/12`) can resolve to the wrong one — mildly ironic for a mode whose job is
disambiguating tabs. We accepted this for v1 because exact-prefix collisions are
uncommon and the simplicity is worth it. Chromium AppleScript tabs carry no
stable ID, so positional focus was the only unambiguous alternative, and it
brings its own staleness failure (an index pointing at a different tab after one
closes). If wrong-tab focusing ever bites in practice, revisit with positional
focus then.

## The list script is read-only and never activates or launches the browser

Listing tabs runs a *second* AppleScript distinct from the focus script. Unlike
the focus path it must **not** `activate` the browser — otherwise merely pressing
Tab would yank the browser to the foreground before the user has picked anything,
backwards for a TUI switcher. It also guards on `application "X" is running` and
returns an empty list when the browser is closed, so listing never *launches* a
browser that wasn't already open. The list is fetched asynchronously on mode
entry (a `tea.Cmd`, like `act`), with a brief `loading tabs…` state.
