# Context: bml (bookmark launcher)

A macOS terminal tool for launching bookmarks. Drives a Chromium browser
(Brave by default, configurable) via AppleScript automation.

## Glossary

### Leader mode
The default interactive mode entered when `bml` is launched with no arguments.
Presents **favorites** navigable by **key sequence** (which-key style): pressing
characters drills through **groups** until a bookmark is reached, which acts
immediately (no Enter). Modeled on "leader key" launchers. Curated by hand.

Favorites can alternatively be **browsed**: arrow keys move a selection over the
reachable bookmarks and Enter acts on the highlighted one. Browsing is a
discovery aid layered on top of — and subordinate to — the **key sequence**
path: the keys remain authoritative (typing one always acts immediately,
ignoring any selection), the selection is latent until an arrow is pressed, and
it never offers "force a new tab" (that stays on the keyed path). It exists so a
favorite can be reached without knowing its **key**.

### Bookmark
A stored URL entry — the single core entity. All bookmarks live in one flat
list, in `bookmarks.toml`. A bookmark may optionally carry a **key**, which
promotes it into **leader mode**. Every bookmark (keyed or not) is reachable via
**bookmarks mode**.

### Config directory
The directory (default `~/.config/bml`, overridable with `--config` or
`$BML_CONFIG`) holding two files: `bookmarks.toml` (the **bookmark** entries) and
`config.toml` (settings — browser, `leader_tags`, the **search engine** config,
and **group** labels). They are split so `bml import` only ever rewrites
`bookmarks.toml` and can never clobber hand-curated settings.

### Key (key sequence)
A bookmark's optional leader binding: 1–3 characters typed in turn (e.g. `g` or
`wt`). A multi-character key navigates through **groups**. Keys are
**prefix-free** — no key may be a strict prefix of another — so navigation is
unambiguous and a key is never both a bookmark and a group. A key may not begin
with `s`: that character is **reserved** at the top level for entering **search
mode**, so any `s…` key would be unreachable.

### Group
A key **prefix** that holds further keys (e.g. `w` for the keys `wt`, `wc`).
Groups are implied by prefixes; an optional **group label** gives the prefix a
friendly name in the menu (e.g. `w` → "Work"). Not a stored entity itself.

### Favorite
Not a separate entity — a **favorite** is simply a bookmark that carries a
**key**. Used informally to mean "the bookmarks that appear in leader mode."

### Bookmarks mode
A mode entered from leader mode (via `/`) that fuzzy-filters the full set of
bookmarks (matching name, URL, and tags) and acts on the chosen one. Named for
what it browses, not the **bookmark** entity itself.
_Avoid_: Search mode (now reserved for web search), filter mode, find mode.

### Search mode
A mode entered from leader mode (via `s`) that sends a free-text **query** to a
web **search engine** rather than touching stored bookmarks. Pressing Enter
dispatches to the **primary engine**; Tab dispatches to the **secondary engine**
(Tab, not Shift+Enter, because Bubble Tea v1 can't distinguish Shift+Enter from
Enter). The dispatched URL always opens a new tab.

### Search engine
A named URL **template** containing a `{{input}}` placeholder (e.g. `google` →
`https://www.google.com/search?q={{input}}`). The single configurable unit of
search: every search action is just "fill this engine's template with the query
and act on the result." Three are built in — `google`, `duckduckgo`, and
`duckduckgo_lucky` (DuckDuckGo's `!ducky` bang, which redirects to the first
result). Users may define more in config. There is deliberately no per-engine
"I'm feeling lucky" variant — "lucky" is simply its own engine, because no
reliable URL-template lucky exists for Google.
_Avoid_: provider, site, bang.

### Query
The free-text string the user types in **search mode**, substituted into a
**search engine** template (URL-escaped) to form the URL that is acted upon.
Distinct from a leader **key sequence** and from a **bookmarks mode** filter
string.

### Primary / secondary engine
The two **search engine** slots bound in **search mode**: the **primary engine**
(Enter — the configurable "default search engine") and the **secondary engine**
(Tab — defaults to `duckduckgo_lucky`). Both are configurable by name.

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
backends without touching leader/search/CLI code. A backend may additionally
implement **tab listing** to power **tab mode**.

### Open tab
A live tab currently open in the configured browser, captured as a `{title, url}`
pair. Distinct from a **bookmark** (a stored, hand-curated URL): an open tab is
ephemeral browser state, enumerated on demand, never persisted. The entity
**tab mode** lists and focuses. _Avoid_: calling it a bookmark, or a "session".

### Tab mode
A mode entered from leader mode via the **Tab key** that lists the **open tabs**
of the configured browser and fuzzy-filters them (over title + URL, like
**bookmarks mode**); selecting one **focuses** that tab via the existing
**act on a URL** path (its full URL passed to `OpenOrFocus`, which substring-
matches it back to the live tab), then exits. Read-only: it never opens, closes,
or rearranges tabs in v1 — a pure switcher. Because focusing re-matches by URL
substring, two open tabs sharing a URL prefix may resolve to the wrong one; this
is an accepted v1 limitation, not a bug.

The trigger is the **Tab key** (not a printable letter), so unlike `s` and `/`
it reserves no character from the bookmark **keyspace**. Note "Tab" is overloaded
deliberately: the **Tab key** enters tab mode at the leader level, while in
**search mode** Tab dispatches the **secondary engine** — different modes, no
collision.

### Tab listing
The capability of enumerating the configured browser's **open tabs**, exposed as
a separate optional interface (`TabLister`) rather than widening the core
**browser backend** seam — a backend that cannot (or chooses not to) enumerate
tabs simply does not implement it, and **tab mode** is unavailable on it. The
macOS Chromium backend implements it with a **read-only** AppleScript that does
*not* `activate` the browser (listing must never steal focus) and does not launch
the browser when it is not running (guarded by `is running`, yielding an empty
list instead).
