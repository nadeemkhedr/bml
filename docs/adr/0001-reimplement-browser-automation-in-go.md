# Reimplement browser open/focus automation in Go instead of shelling out

## Context

A working bash script already existed that drives the browser via AppleScript
(`osascript`): it focuses an existing tab by substring-matching the tab URL, or
opens a new tab, and accepts `-n/--new-tab` to force a new tab. bml's core "act
on a URL" routine needs exactly this behavior, backing leader mode, search mode,
and the `bml <arg>` CLI path.

## Decision

bml generates and runs the AppleScript itself (via `os/exec` + `osascript`)
rather than shelling out to the existing script. Matching is **scheme-insensitive
substring** match on open tab URLs; the real stored URL (with its scheme) is
opened, not an `http://`-prefixed fragment. The target browser is a TOML setting
(`browser`, default `Brave Browser`).

The automation sits behind a **pluggable backend interface** so other
platforms/browsers can be added without touching leader/search/CLI code. The
seam is deliberately **coarse** — a single method,
`Browser.OpenOrFocus(url string, forceNew bool) error` — because AppleScript
finds-and-focuses a tab in one round-trip and gives no stable cross-call tab
handle, so a finer `ListTabs`/`Focus`/`OpenNew` split would fight the platform.
Each backend owns its own focus-or-open behavior and matching. v1 ships one
backend, **macOS Chromium**, which covers Brave/Chrome/Arc/Edge by
parameterizing the app name; the backend is inferred from the OS for now and can
gain an explicit selector when a second backend exists.

## Considered Options

- **Shell out to the existing script** — reuse proven code, stay portable and
  tiny, swap browser by editing one file. Rejected in favor of a self-contained
  binary.
- **Embed the script in the binary** — single binary that reuses exact logic,
  but awkward to customize.

## Consequences

- bml is a single self-contained binary with no external script/PATH dependency,
  and handles full URLs cleanly (no `http://`-prefix hack).
- bml now **owns** browser automation: v1 is macOS-only with AppleScript
  embedded in Go, and the browser choice lives in bml (mitigated by making it
  configurable).
- Cross-platform/cross-browser support is a deferred-but-designed-for goal: new
  backends implement one method and register themselves; the rest of bml is
  unaffected.
- The proven bash logic is duplicated; the two can drift. bml is the source of
  truth going forward.
