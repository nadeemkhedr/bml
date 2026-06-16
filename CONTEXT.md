# Context: bml (bookmark launcher)

A macOS terminal tool for launching bookmarks. Drives a Chromium browser
(Brave by default, configurable) via AppleScript automation.

## Glossary

### Leader mode
The default interactive mode entered when `bml` is launched with no arguments.
Presents **favorites** navigable by **key sequence** (which-key style): pressing
characters drills through **groups** until a bookmark is reached, which acts
immediately (no Enter). Modeled on "leader key" launchers. Curated by hand.

### Bookmark
A stored URL entry — the single core entity. All bookmarks live in one flat
list. A bookmark may optionally carry a **key**, which promotes it into
**leader mode**. Every bookmark (keyed or not) is reachable via **search mode**.

### Key (key sequence)
A bookmark's optional leader binding: 1–3 characters typed in turn (e.g. `g` or
`wt`). A multi-character key navigates through **groups**. Keys are
**prefix-free** — no key may be a strict prefix of another — so navigation is
unambiguous and a key is never both a bookmark and a group.

### Group
A key **prefix** that holds further keys (e.g. `w` for the keys `wt`, `wc`).
Groups are implied by prefixes; an optional **group label** gives the prefix a
friendly name in the menu (e.g. `w` → "Work"). Not a stored entity itself.

### Favorite
Not a separate entity — a **favorite** is simply a bookmark that carries a
**key**. Used informally to mean "the bookmarks that appear in leader mode."

### Search mode
A mode entered from leader mode (via `/`) that fuzzy-searches the full set of
bookmarks (matching name, URL, and tags) and acts on the chosen one.

### Act on a URL (open vs focus)
Taking action on a URL either **focuses** an already-open browser tab or
**opens** a new tab. "Focus" finds an existing tab by **substring match**
(scheme-insensitive) on the tab's URL; if none matches, a new tab opens.
A "force new tab" path (uppercase final key, or `-n/--new-tab`) skips the match
and always opens. This single routine backs leader mode, search mode, and the
`bml <arg>` CLI path. The actual automation is delegated to a **browser
backend**.

### Browser backend
A pluggable implementation of "act on a URL" for a specific platform + browser,
exposed through one coarse interface (`OpenOrFocus(url, forceNew)`). The backend
owns its own focus-or-open mechanism and matching. v1 ships a single backend —
**macOS Chromium** (runs AppleScript via `osascript`), which covers all
Chromium browsers (Brave/Chrome/Arc/Edge) by parameterizing the app name.
Additional platforms/browsers (Safari, Firefox, Linux, Windows) are added as new
backends without touching leader/search/CLI code.
