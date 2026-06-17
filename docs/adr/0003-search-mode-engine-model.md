# Search mode: flat engine model and Tab as the secondary action

Search mode (entered with `s`) sends a free-text query to a web search engine.
We model a **search engine** as nothing more than a named URL template with a
`{{input}}` placeholder, and we bind two action keys to two engine slots: Enter →
the **primary engine** (configurable `default_engine`, default `google`) and Tab
→ the **secondary engine** (configurable `secondary_engine`, default
`duckduckgo_lucky`). Three engines ship built in — `google`, `duckduckgo`, and
`duckduckgo_lucky` — and users may define more under `[search.engines]`.

## "Lucky" is its own engine, not a per-engine variant

The original request framed "I'm feeling lucky" as a second *action* available on
the default engine (search vs. lucky). We rejected that because there is no
reliable URL-template "lucky" for Google: the old `&btnI` redirect has been
broken/removed and now hits a consent interstitial or plain results.
DuckDuckGo's `!ducky` bang, by contrast, is a real server-side redirect. So
"lucky" only meaningfully exists for DuckDuckGo, and modeling it as a per-engine
feature would bake in an asymmetry that doesn't hold. Instead `duckduckgo_lucky`
is simply its own engine (its own template), and the Enter/Tab keys are a generic
"primary/secondary engine" mechanism rather than a "search/lucky" one. This keeps
every search action a plain template substitution with no special cases.

## Tab triggers the secondary engine, not the requested Shift+Enter

The request was for Shift+Enter to invoke the secondary (lucky) search. Bubble
Tea v1.3.10 — the pinned version — cannot detect it: there is no `KeyShiftEnter`,
the `Key` struct carries only an `Alt` modifier (no `Shift`), and the library
does not enable the kitty keyboard protocol that would encode the distinction.
Shift+Enter therefore arrives as a plain Enter in every terminal, Ghostty
included. Getting literal Shift+Enter would require upgrading to the beta Bubble
Tea v2 and depending on terminal support — too much cost and risk for this
feature. We chose Tab: reliably distinct in every terminal, ergonomically
adjacent to Enter. If the project later moves to Bubble Tea v2, revisit whether
to offer Shift+Enter as well.

## Other settled points

- A leader **key** may not begin with `s` (reserved at the top level for entering
  search mode); enforced as a hard error at config load, since any `s…` key would
  be unreachable.
- The query is URL-escaped into the template and the result **always opens a new
  tab** (never focus-or-open) — a search is an action invoked now, and substring
  focus-matching on `?q=…` would surprise.
- Invalid search config (unknown engine name, or a custom template missing
  `{{input}}`) is a hard error at load, consistent with how bad/duplicate leader
  keys are handled.
