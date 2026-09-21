# Browse mode and history-personalized autocomplete

Browse mode (entered with `.` from leader mode) is an address bar: the user types
a URL or a search term, and Enter navigates to the URL or searches the primary
engine. While typing, an autocomplete list suggests matching bookmark URLs first,
then entries from the user's own browser history. Three sub-decisions took a
non-obvious path and are worth recording.

## URL-vs-search is decided by a shape heuristic, not parsing

The input is treated as a URL when it has no whitespace and either contains a
scheme (`://`), contains a `.`, or is the `localhost` special case; otherwise it
is a search query. This mirrors a browser's omnibox and deliberately avoids a
full URL parser: `golang generics` searches (has a space), `example.com` and
`localhost:3000` navigate. A URL missing a scheme gets `https://` prepended. The
same heuristic chooses what the row-0 "primary action" label says, so the user
sees up front whether Enter will open or search.

Navigating focus-or-opens (consistent with leader mode and the `bml <url>` CLI —
typing a site you already have open should focus it), while a search always opens
a new tab (consistent with search mode).

## Autocomplete is two tiers: bookmark **prefix** match, then browser history

Browse mode does **not** reuse bookmarks mode's fuzzy matcher. The requirement is
address-bar behavior: typing `git` should complete to `github.com`, not fuzzily
match `gist` or a bookmark whose name merely contains those letters. So tier 1 is
a literal **prefix** match over each bookmark's normalized host (scheme and
`www.` stripped), ranked ahead of everything. Tier 2 fills the remaining slots
from the browser's own history by the same prefix, skipping any URL already
shown. Editing the query resets the cursor to the primary action, so Enter
without arrowing always acts on exactly what was typed.

## The popular-sites source is the user's history, not a global ranking

We first shipped a generic top-sites list (Majestic Million, embedded and
binary-searched). It was the wrong data: a global popularity ranking is mostly
CDN, tracking, regional, and adult domains that a given person never types, so
its suggestions felt irrelevant. The signal that matters for *an* address bar is
*that user's* behavior. So tier 2 reads the browser's own history and ranks by
`typed_count` (how often the user has typed an address — exactly what a browser
omnibox weights) then `visit_count`. With no global list there is also nothing to
bundle, attribute, or periodically regenerate.

## History is read via the system `sqlite3`, immutable, async — no CGO

Chromium stores history in a SQLite database, but bml is a single pure-Go,
zero-CGO static binary (ADR 0001). Linking a SQLite driver would mean either CGO
(breaking the static build) or a multi-megabyte pure-Go driver — for a read-only
query we run once. Instead we shell out to the system `sqlite3` (present on
macOS), the same "delegate to an OS tool" move as the AppleScript backend.

Two reads were rejected before landing on the right one. A plain read-only open
fails — Chromium holds a lock on the live file while running (`database is locked
(5)`). Copying the 22 MB DB to a temp file first works but costs ~100 ms of disk
churn on every entry. The chosen path opens the live file with a `file:…?immutable=1`
URI: SQLite then assumes the file never changes, takes **no lock** and makes **no
copy**, reading in ~10 ms while the browser is running. The tradeoff is that
immutable mode ignores the write-ahead log, so the current session's newest
visits may be absent — irrelevant when ranking by accumulated counts. The path is
percent-encoded into the URI (the macOS profile path contains a space) since the
read is exec'd without a shell.

Reading is modeled as an optional backend capability (`HistoryLister`) exactly
like `TabLister` (ADR 0005): the macOS Chromium backend implements it, browse
mode type-asserts for it, and a backend that can't read history just doesn't
provide it. The read runs in a `tea.Cmd` kicked off on mode entry (like tab
listing), so entry is instant and suggestions populate a few milliseconds later —
well before the user finishes typing. It reads the `Default` profile and degrades
silently to bookmark-only suggestions if `sqlite3` is missing, the profile path
is unknown, or the query fails.
