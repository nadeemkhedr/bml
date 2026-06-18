# bml — bookmark launcher

A fast, keyboard-driven bookmark launcher for the terminal (macOS). Bare `bml`
opens a which-key launcher; press a letter to **focus an already-open tab** for
that site (or open it), Shift+letter to force a new tab. Don't know the key? Use
`↑`/`↓` to browse and `Enter` to open the highlighted one. Press `/` to
fuzzy-search your whole bookmark list, `s` to search the web, or `Tab` to switch
between your browser's currently open tabs.

```
▌ bml  launcher

   g   GitHub        dev
   n   Hacker News   news
   m   Gmail         work

  ↑↓  browse   ·   ↵  open   ·   Shift+key  new tab   ·   /  bookmarks   ·   s  search   ·   ⇥  tabs   ·   q  quit
```

## Tab mode

Press `Tab` in the launcher to list the open tabs of your configured browser and
fuzzy-filter them by title or URL — type a few characters, then `Enter` to jump
to (focus) that tab. It's a pure switcher: it only ever focuses an existing tab,
never opening, closing, or rearranging anything. Opening tab mode never steals
focus or launches a closed browser. `Esc` returns to the launcher.

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
bml            # opens the launcher (creates ~/.config/bml/{bookmarks,config}.toml on first run)
bml edit       # edit your bookmarks in $EDITOR
```

The first time bml controls your browser, macOS asks for **Automation**
permission — approve it (System Settings → Privacy & Security → Automation).

## Config files

bml uses a config **directory** (default `~/.config/bml`, override with `--config`
or `$BML_CONFIG`; `$XDG_CONFIG_HOME` is honored) holding two hand-edited TOML files:

- **`bookmarks.toml`** — your bookmark entries. `bml import` only ever rewrites
  this file.
- **`config.toml`** — settings: browser, `leader_tags`, search engines, and group
  labels. Import never touches it.

Edit them with `bml edit` (bookmarks) and `bml edit --settings` (config.toml).

A third file, **`history.toml`**, is written automatically to record which
bookmarks you pick in bookmarks mode (see "Bookmarks mode" below). It is
machine-maintained — never hand-edit it; reset it with `bml history clear`.

`bookmarks.toml`:

```toml
[[bookmark]]
key = "g"            # optional 1–3 char leader key
name = "GitHub"
url = "https://github.com"
tags = ["dev"]       # optional; searchable

[[bookmark]]
key = "wt"           # grouped: press "w" then "t"
name = "Work Tasks"
url = "https://app.clickup.com"
tags = ["work"]

[[bookmark]]
name = "Go docs"     # no key → not in the launcher, but still searchable
url = "https://pkg.go.dev"
tags = ["dev", "reference"]
```

`config.toml`:

```toml
# Which macOS browser to drive (default "Brave Browser").
# browser = "Google Chrome"

# Show tags next to bookmarks in leader mode (default true).
# leader_tags = false

# Search engines (see "Search mode" below).
# [search]
# default_engine = "google"

# Give a key-group prefix a friendly name in the menu.
[[group]]
key = "w"
name = "Work"
```

- A bookmark needs a **name** and **url**.
- An optional **key** (1–3 characters) binds it to the launcher. Multi-character
  keys form **groups**: `wt` means press `w` (opens the Work group) then `t`. A
  key may not start with `s` (reserved for search mode).
- Keys are **prefix-free** — a key can't be both a bookmark and a group prefix
  (no `w` *and* `wt`). Duplicate or conflicting keys are rejected on load.

## Usage

### Launcher (leader mode) — `bml`

| Key            | Action                                            |
| -------------- | ------------------------------------------------- |
| `g` (a key)    | Focus an existing tab for that bookmark, else open |
| `w` then `t`   | Navigate a group, then act (`wt`)                 |
| Uppercase last | Force a new tab (`G`, or `wT`)                    |
| `Backspace`    | Go up one group level                             |
| `/`            | Enter bookmarks mode (fuzzy-find your bookmarks)  |
| `s`            | Enter search mode (search the web)                |
| `q` / `Esc` / `Ctrl-C` | Quit (`Esc` first leaves the current group) |

Because `s` enters search mode, no bookmark or group key may begin with `s` —
such a key would be unreachable, and is rejected when the config loads.

### Bookmarks mode — `/`

Fuzzy-matches **name + url + tags**. Type to filter, `↑`/`↓` (or `Ctrl-n`/`Ctrl-p`)
to move, `Enter` to focus/open, `Esc` to go back.

Results learn from use. Each pick is remembered against the query you typed, so
the bookmarks you habitually choose float toward the top — pick the 4th result
for `en` a few times and it becomes the first result for `en`. With no query, the
list is ordered by your most-used bookmarks overall. Recent picks weigh more than
old ones, so habits adapt over time. Run `bml history clear` to forget everything
learned.

### Search mode — `s`

Type a free-text query and search the web. `Enter` searches with the **primary
engine** (default `google`); `Tab` searches with the **secondary engine** (default
`duckduckgo_lucky`, DuckDuckGo's "I'm feeling lucky" `!ducky` jump). Results always
open in a new tab; `Esc` goes back.

Engines are URL templates with an `{{input}}` placeholder. Built-ins are `google`,
`duckduckgo`, and `duckduckgo_lucky`; choose which back the two keys, or define your
own, in `config.toml`:

```toml
[search]
default_engine   = "google"            # Enter
secondary_engine = "duckduckgo_lucky"  # Tab

[search.engines]
kagi = "https://kagi.com/search?q={{input}}"
```

### Command line

```sh
bml g                 # act on the bookmark bound to key "g"
bml wt                # act on a grouped key sequence
bml github.com        # act on a URL (scheme optional)
bml -n github.com     # force a new tab
bml -n wt             # force a new tab for a keyed bookmark
bml edit              # open bookmarks.toml in $EDITOR
bml edit --settings   # open config.toml (browser, search engines, groups)
bml history clear     # forget learned bookmark ranking
```

A 1–3 character argument with no `.` is a **key sequence**; an argument with a
`.` is a **URL**; anything else errors.

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
`docs/epics/<epic>/` (per-feature `prd.md` requirements and `plan.md`
implementation plan).
