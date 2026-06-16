# bml — bookmark launcher

A fast, keyboard-driven bookmark launcher for the terminal (macOS). Bare `bml`
opens a which-key launcher; press a letter to **focus an already-open tab** for
that site (or open it), Shift+letter to force a new tab. Press `/` to fuzzy-search
your whole bookmark list.

```
▌ bml  launcher

   g   GitHub        dev
   n   Hacker News   news
   m   Gmail         work

  Shift+key  new tab   ·   /  search   ·   q  quit
```

## Install

Install globally to `/usr/local/bin` (the usual place for CLI tools):

```sh
make install          # add `sudo` if /usr/local/bin isn't writable: sudo make install
```

Pick a different location with `PREFIX` (no sudo needed if it's writable):

```sh
make install PREFIX=~/.local      # installs to ~/.local/bin
```

Remove it with `make uninstall` (pass the same `PREFIX` you installed with).

Other options:

```sh
go install .          # installs to $(go env GOPATH)/bin
make build            # just build ./bml in the repo
```

Requires macOS and a Chromium-based browser (Brave, Chrome, Arc, or Edge).

## Quick start

The first run writes a starter config and tells you where:

```sh
bml            # opens the launcher (creates ~/.config/bml/bookmarks.toml on first run)
bml edit       # edit your bookmarks in $EDITOR
```

The first time bml controls your browser, macOS asks for **Automation**
permission — approve it (System Settings → Privacy & Security → Automation).

## Bookmarks file

TOML, hand-edited, at `~/.config/bml/bookmarks.toml` (override with `--config` or
`$BML_CONFIG`; `$XDG_CONFIG_HOME` is honored).

```toml
# Optional: which macOS browser to drive (default "Brave Browser").
# browser = "Google Chrome"

[[bookmark]]
key = "g"            # optional single-character leader key
name = "GitHub"
url = "https://github.com"
tags = ["dev"]       # optional; searchable

[[bookmark]]
name = "Go docs"     # no key → not in the launcher, but still searchable
url = "https://pkg.go.dev"
tags = ["dev", "reference"]
```

- A bookmark needs a **name** and **url**.
- An optional single-character **key** binds it to the launcher (leader mode).
- Duplicate keys are rejected with an error — bml won't start with an ambiguous
  config.

## Usage

### Launcher (leader mode) — `bml`

| Key            | Action                                            |
| -------------- | ------------------------------------------------- |
| `g` (a letter) | Focus an existing tab for that bookmark, else open |
| `G` (Shift)    | Force a new tab                                   |
| `/`            | Enter search                                      |
| `q` / `Esc` / `Ctrl-C` | Quit                                      |

### Search — `/`

Fuzzy-matches **name + url + tags**. Type to filter, `↑`/`↓` (or `Ctrl-n`/`Ctrl-p`)
to move, `Enter` to focus/open, `Esc` to go back.

### Command line

```sh
bml g                 # act on the bookmark bound to key "g"
bml github.com        # act on a URL (scheme optional)
bml -n github.com     # force a new tab
bml -n g              # force a new tab for a keyed bookmark
bml edit              # open the bookmarks file in $EDITOR
```

### Importing bookmarks

Import from a Chromium-based browser (`brave`, `chrome`, `edge`). Folder names
become tags. New bookmarks are **merged** in — existing entries and their keys
are kept — so it's safe to re-run.

```sh
bml import brave                 # merge Brave's Default profile into your config
bml import chrome --profile "Profile 1"
bml import brave --dry-run       # preview the result without writing
bml import brave --replace       # overwrite instead of merging
```

Imported bookmarks have no leader key — add `key = "x"` to the ones you want in
the launcher. The previous config is backed up to `<file>.bak` on each write.

A single-character argument is treated as a **key**; a multi-character argument
must contain a `.` to be treated as a **URL** (otherwise it's an error).

## How "focus existing tab" works

bml drives the browser with AppleScript. It looks for an open tab whose URL
contains the bookmark's URL (scheme-insensitive) and activates it; if none
matches — or you forced a new tab — it opens the URL. The browser and matching
live behind a single backend interface, so other platforms/browsers can be added
later. See `docs/adr/0001-reimplement-browser-automation-in-go.md`.

## Development

```sh
go test ./...
go build -o bml .
```

Design notes: `CONTEXT.md` (glossary), `docs/adr/` (decisions),
`docs/prd/` (requirements), `docs/plans/` (implementation plan).
