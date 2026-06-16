# Leader keys are prefix-free 1–3 character sequences with groups

## Context

Single-character leader keys cap the launcher at ~36 bindings and offer no
organization. We want grouping — e.g. a "Work" group `w` where `wt` opens work
tasks — without introducing timing-based disambiguation (which-key timeouts feel
bad in a terminal).

## Decision

A bookmark key is a **1–3 character sequence**; multi-character keys navigate
through **groups** (a group is just a key prefix). Pressing characters in leader
mode descends until a complete key is reached, which acts immediately. The last
character's case decides focus vs. force-new (`wt` focuses, `wT` opens new).
Group prefixes may carry an optional friendly label via `[[group]]`.

Keys are **prefix-free**: no key may be a strict prefix of another. The config
loader rejects violations.

## Considered Options

- **Timeout-based which-key** (allow `w` to be both a bookmark and a group,
  acting after a pause). Rejected — timing-dependent behavior is unpredictable
  and untestable.
- **Unlimited depth.** Rejected — capped at 3 (two groups + leaf) to keep the
  menu shallow and the CLI rule simple.

## Consequences

- Navigation is fully deterministic: an exact key match is, by construction, the
  unique completion, so it can act with no lookahead or timer.
- A key can never be both a bookmark and a group — occasionally requires
  renaming a single-char favorite when adding a group under the same letter.
- The CLI argument rule widens accordingly: 1–3 characters with no `.` is a key
  sequence, a `.` means a URL, anything longer with no `.` is an error.
