# Implementation Plan: bml v1

Source: [PRD 0001](../prd/0001-bml-v1.md) · Decisions: [ADR 0001](../adr/0001-reimplement-browser-automation-in-go.md) · Glossary: [CONTEXT.md](../../CONTEXT.md)

## Approach

Tracer-bullet vertical slices. Each phase is end-to-end runnable and
demonstrable, not a horizontal layer. The **riskiest integration — the
AppleScript/`osascript` backend and macOS Automation (TCC) permission — is built
first** (Phase 1) so the hardest unknown is proven before any UI is written.
Everything routes through the single `Browser.OpenOrFocus(url, forceNew)` seam,
which is injected as a fake in tests from Phase 1 onward.

Each phase lists: **Objective**, the **Slice** that works when it's done, what to
**Build**, the **Tests** (per the confirmed seams), and **Done when**.

---

## Phase 0 — Scaffold

**Objective:** A buildable Go binary with the Cobra skeleton.

**Slice:** `bml --help` and `bml --version` run.

**Build:**
- Go module `bml`; standard layout (`main.go` thin, logic under internal
  packages).
- Cobra root command wired; `--version`.
- `golangci-lint` / `go vet` / `go test` all green on an empty suite. Makefile or
  task targets for build/test/lint.

**Tests:** none beyond "it builds and the root command runs."

**Done when:** `go build` produces a `bml` binary; `bml --help` lists the command.

---

## Phase 1 — Tracer bullet: `bml <url>` acts on a real browser

**Objective:** Prove the full risky path — CLI → act-core → backend → real
Chromium — before any config or TUI exists.

**Slice:** `bml github.com` focuses an existing matching tab or opens a new one in
Brave; `bml -n github.com` always opens a new tab.

**Build:**
- `Browser` interface: `OpenOrFocus(url string, forceNew bool) error`.
- **macOS Chromium backend**: generates AppleScript and runs it via
  `os/exec` + `osascript`. Scheme-insensitive substring match inside the script;
  open the real URL with its scheme if no match (or `forceNew`). App name
  hardcoded to `Brave Browser` for now (config comes in Phase 5/2).
- Act-core function that takes `(url, forceNew)` and calls the injected `Browser`.
- Root command: a bare positional arg that contains `.` is treated as a URL and
  passed to act-core; `-n/--new-tab` global flag.
- **Fake `Browser`** test double that records `(url, forceNew)`.

**Tests:**
- CLI seam: `bml github.com` → fake records `("github.com", false)`; `-n` →
  `forceNew=true`.
- Backend: light test of the AppleScript *string generation* (URL substituted,
  `forceNew` toggles the match branch) — not execution.
- **Manual on macOS:** real focus vs. new-tab against Brave; confirm the TCC
  prompt appears and, once granted, focus works. (Matching is manual-only by
  design — ADR 0001.)

**Done when:** the real binary focuses and opens tabs in Brave, and the fake-based
CLI tests pass.

---

## Phase 2 — Config + full CLI resolution

**Objective:** Bookmarks come from a real TOML file and `bml <arg>` resolves keys
as well as URLs.

**Slice:** `bml g` acts on the bookmark bound to `g`; `bml github` errors; first
run writes a starter config; `bml edit` opens it.

**Build:**
- `Bookmark` model: `name` (req), `url` (req), `key` (optional single char),
  `tags` (optional). One flat list.
- TOML `Load`: parse `[[bookmark]]` array-of-tables. Validate required fields;
  **duplicate `key` → hard error naming the key**; invalid TOML → hard parse
  error. Default path `~/.config/bml/bookmarks.toml`; `--config` / `$BML_CONFIG`
  override.
- First-run behavior: write a commented starter config when none exists.
- Argument `Resolve`: 1 char → key lookup (error if unbound); else if contains
  `.` → URL; else → error.
- `bml edit`: open config in `$EDITOR`.

**Tests:**
- Pure units: `Load` (valid, missing-required, duplicate-key error, optional
  fields), `Resolve` (one-char hit/miss, dotted URL, no-dot error) — table-driven.
- CLI seam: `bml g` with a temp config + fake backend records the bound URL;
  `bml github` errors; unbound `bml z` errors.

**Done when:** the CLI works end-to-end off a TOML file, with all resolution and
validation cases covered.

---

## Phase 3 — Leader mode (Bubble Tea)

**Objective:** Bare `bml` opens the which-key launcher.

**Slice:** `bml` shows keyed bookmarks; lowercase key focuses-or-opens, uppercase
forces new; bml fires and exits; Esc/`q`/Ctrl-C quit.

**Build:**
- Bubble Tea leader model rendering a `key → name` list of keyed bookmarks plus
  hints (`/` search, Esc/`q` quit).
- Key handling: lowercase letter → act `forceNew=false`; uppercase → `forceNew=
  true`; act via the injected `Browser`, then quit (fire and exit).
- Quit on Esc/`q`/Ctrl-C.
- Root command with no resolvable arg launches this model.

**Tests:**
- TUI seam: feed `tea.KeyMsg` into `Update`; assert lowercase → act with
  `forceNew=false`, uppercase → `forceNew=true` (against the fake backend), and
  Esc/`q`/Ctrl-C transition to quit. Unbound key → no-op.

**Done when:** the launcher visibly lists favorites and acting on one focuses/opens
in the browser, then exits.

---

## Phase 4 — Search mode

**Objective:** `/` reaches any bookmark, keyed or not, via fuzzy search.

**Slice:** From leader mode, `/` opens a fuzzy finder over name+URL+tags; typing
filters live; Enter acts (focus-or-open); Esc returns to leader.

**Build:**
- `Filter`: fuzzy match a query over name+URL+tags, ranked.
- Bubble Tea search model: query input + filtered list; Enter acts via `Browser`
  then exits; Esc returns to leader model.
- Wire `/` in leader mode to enter search.

**Tests:**
- Pure unit: `Filter` (matches by name, by URL, by tag; ranking/empty query).
- TUI seam: `/` enters search; typing narrows results; Enter → act on the
  selected bookmark (fake backend); Esc → back to leader.

**Done when:** any bookmark is reachable by fuzzy search and selecting one acts and
exits.

---

## Phase 5 — Polish & hardening

**Objective:** Production-quality edges.

**Slice:** Configurable browser, graceful permission/error UX, docs.

**Build:**
- `browser` config setting (default `Brave Browser`) threaded into the backend;
  covers Brave/Chrome/Arc/Edge.
- macOS Automation (TCC) error detection: surface a clear "automation permission
  denied / not yet granted" message instead of an opaque osascript failure.
- Friendly errors for missing config dir, unwritable starter file, empty
  bookmark list.
- README + usage docs; starter-config content with helpful comments.

**Tests:**
- Backend: app name from config is substituted into the script.
- Error-path tests for the permission/known-failure mapping (against a fake
  `osascript` runner if the exec call is itself behind a small seam).
- Manual on macOS: switch browser via config; revoke automation permission and
  confirm the friendly message.

**Done when:** bml drives a configured browser, fails legibly when it can't, and is
documented.

---

## Sequencing rationale

- **Phase 1 first** retires the biggest risk (osascript + TCC) end-to-end before
  investing in UI.
- **Phase 2 before 3/4** so the TUI has real data and the shared act-core/backend
  are already proven and injectable.
- **Leader (3) before search (4)** — search reuses the act-core, the backend
  injection, and the model patterns established by leader mode.
- Every phase ships something runnable; no phase is a pure horizontal layer.

## Cross-cutting testing notes

- The injected `Browser` fake is the spine of the suite — introduced in Phase 1,
  reused everywhere.
- Prefer the highest seam: Cobra command tests for CLI behavior, Bubble Tea
  `Update` tests for interactive behavior, pure table-driven tests for `Load` /
  `Resolve` / `Filter`.
- AppleScript tab-matching is **out of automated scope** (manual on macOS) per
  ADR 0001's coarse seam; assert only the generated-script shape, never execution.
