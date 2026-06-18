# Learned ranking via usage history

Bookmarks mode now reorders matches by **learned ranking** — an adaptive boost
derived from past selections, persisted to a new `history.toml` in the config
directory. The bookmarks a user habitually picks for a given query rise toward
the top, Raycast/Spotlight-style. This is layered *on top of* the existing
tier-based match scorer (`internal/tui/filter.go`), which is unchanged: the raw
match still gates which bookmarks appear and supplies base relevance; learned
ranking only reorders within the matched set.

## Why this, and not a library

The triggering request was "add scoring," but bml already scores (the 4-tier
prefix/word/substring/scatter model). What was actually missing is *adaptive*
ranking from usage. We surveyed the Go landscape:

- **Fuzzy-matching libraries** (`sahilm/fuzzy`, `lithammer/fuzzysearch`, fzf's
  `algo`) solve a different problem — single-field match scoring. None does
  multi-field (name/url/tags) ranking with per-field highlight offsets, which is
  why our matcher stays hand-rolled. `sahilm/fuzzy` is the only one worth
  adopting and is **orthogonal** to this feature; we did not adopt it here.
- **Frecency** has no canonical Go library. zoxide (the reference design) is
  Rust and its aging logic is ~40 lines. So this is a deliberate hand-roll.

## Considered options

- **Global frecency only (zoxide):** one score per bookmark, query-independent.
  Rejected as the *primary* signal because it can't satisfy the motivating case
  — "for query `en`, float the item I always pick for `en`." A globally popular
  bookmark would win `en` even if it's never chosen for `en`.
- **Query-keyed only (Raycast):** boost lives on `(query, bookmark)` pairs. Best
  for the `en` case but gives nothing for a query never typed before.
- **Hybrid, query-keyed dominant (chosen):** store both. The `(query, url)`
  signal dominates; a query-independent `("", url)` global signal is the
  fallback for cold queries and orders the empty-query list.

## Decisions

- **Identity is the URL.** No bookmark has a stable ID; URLs survive `bml import`
  and name edits. Editing a bookmark's URL orphans its history — accepted, same
  trade-off zoxide makes when a directory moves.
- **One unified table.** Each selection writes/updates two `[[entry]]` rows:
  the `(query, url)` row and the `("", url)` global row. `query` is the committed
  query at action time, lowercased and trimmed (matching the matcher's
  normalization). An empty-query selection simply bumps the global row.
- **Aging follows zoxide.** `rank += 1` per selection; effective score =
  `rank × decay(now − last)` with zoxide's buckets (×4 within the hour, ×2 within
  a day, ×0.5 within a week, ×0.25 older). Habits fade rather than ossify.
- **Blend.** Among gated matches, `final = baseMatchScore + Kq·queryFrecency +
  Kg·globalFrecency`, with `Kq ≫ Kg` and `Kq` large enough to jump one match
  tier, so a strong habit can leap a fuzzier-but-unused result to the top.
- **Query lookup is prefix-compatible:** a stored query is consulted when it
  *starts with* what the user has typed so far, so typing `e` already resurfaces
  the bookmark habitually chosen at `en`. Fewer keystrokes, same habit.
- **Empty query sorts by global frecency**, with original `bookmarks.toml` order
  as the tiebreak among never-selected bookmarks (and on a fresh install).
- **Self-pruning.** When summed `rank` crosses a ceiling, scale all ranks down
  proportionally; drop entries whose decayed score falls below an epsilon. The
  query-keyed table cannot grow without bound.
- **Scope: bookmarks mode only.** Tab mode stays a pure switcher (ADR 0005) — its
  tabs are ephemeral, so frecency keyed by URL would pollute the table.
- **`bml history clear`** wipes `history.toml` — the reset/privacy valve for a
  feature that silently mutates ordering. No `history` list/show in v1.

## Consequences

- **bml now writes a file programmatically.** ADR 0004 established that
  `config.toml` is hand-edited and "bml never serializes settings
  programmatically." That invariant is intact: hand-curated files
  (`config.toml`, `bookmarks.toml`) stay hand-edited; `history.toml` is a
  separate, machine-owned file. The split rationale of ADR 0004 extends cleanly —
  usage history is isolated from both curated files, and `bml import` never
  touches it.
- Writes happen on each selection (atomic temp-file rename). Two concurrent bml
  instances are last-write-wins; acceptable for a single-user launcher.
