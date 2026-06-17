# Split settings and bookmarks into two files

bml's config directory now holds two files: `bookmarks.toml` (the `[[bookmark]]`
entries) and `config.toml` (settings — `browser`, `leader_tags`, `[search]`, and
`[[group]]` labels). This reverses ADR-era assumption of a single hand-edited
`bookmarks.toml` that carried both data and settings.

## Why

`bml import` rewrites the bookmarks file. When settings lived in that same file,
import had to faithfully round-trip them, and `import --replace` silently dropped
everything except the browser setting (groups, `[search]`, `leader_tags`).
Splitting the files makes import **settings-safe by construction**: it only ever
writes `bookmarks.toml`, and `config.toml` is never opened for writing by import —
so no settings can be lost, and `--replace` simply swaps the bookmark list.

Note this protects *settings*, not *keys*: a bookmark's `key` and `tags` are
intrinsic to its entry and stay in `bookmarks.toml`, so `--replace` still drops
them (by design — that's what replace means). The default merge still preserves
keyed favorites by URL.

## Decisions

- **`--config` / `$BML_CONFIG` now name a directory**, not a file. The two files
  are derived inside it (`config.toml`, `bookmarks.toml`). This breaks the prior
  `--config=<file>` form; we accepted that — see "No migration".
- **No migration.** Existing single-file users split their file by hand. There is
  no auto-migration, no legacy fallback that reads settings out of
  `bookmarks.toml`, and no `bml migrate`. Settings come from `config.toml` or
  default; a stray `[search]`/`browser` left in `bookmarks.toml` is simply ignored.
- **`config.toml` is hand-edited only.** bml never serializes settings
  programmatically — there is a `RenderBookmarks`/`SaveBookmarks` but no settings
  writer. `WriteStarter` lays down a commented `config.toml` on first run.
- **`bml edit`** opens `bookmarks.toml`; **`bml edit --settings`** opens
  `config.toml`.

## Consequences

- The web-search `[search]` config (ADR 0003) lives in `config.toml`. The
  round-trip-preservation machinery it originally needed is gone — import can't
  touch `config.toml`, so preservation is automatic.
- `config.Load` takes a directory and reads both files, validating keys (prefix-
  free, `s`-reserved) and group labels together across the two files.
