// Package history records bookmark selections and turns them into a learned
// ranking signal for bookmarks mode (see CONTEXT.md "Learned ranking" and
// docs/adr/0006). Each selection credits the chosen bookmark — identified by its
// URL — under the query that was typed at the time, plus a query-independent
// global entry. At lookup these decay with age (zoxide-style) and combine into a
// per-URL boost: a query-keyed habit dominates, with global frecency as the
// fallback for never-before-typed queries.
//
// The table is persisted to history.toml in the config directory. It is
// machine-owned: a missing or corrupt file is treated as empty rather than
// fatal, because learned ranking must never block the launcher.
package history

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const fileName = "history.toml"

// Tunable weights and aging parameters (see docs/adr/0006). These are the knobs
// most likely to want adjustment after real-world use.
const (
	// weightQuery scales the query-keyed signal; weightGlobal the query-
	// independent one. weightQuery dominates so an established habit can leap a
	// match tier (tiers are spaced 10000 apart in tui.Filter), while a single
	// recent pick (frecency ~4) only nudges and a much stronger lexical match
	// still wins until the habit is reinforced.
	weightQuery  = 8000
	weightGlobal = 600

	// maxTotalRank caps the summed raw rank: once exceeded, every rank is scaled
	// down proportionally so the table ages as a whole and never grows without
	// bound. pruneFloor drops entries whose decayed score falls below it, so
	// stale one-off picks are forgotten while frequent ones survive.
	maxTotalRank = 5000
	pruneFloor   = 1.0
)

// entry is one (query, url) selection record. An empty Query marks the global,
// query-independent frecency entry for that URL.
type entry struct {
	Query string    `toml:"query"`
	URL   string    `toml:"url"`
	Rank  float64   `toml:"rank"`
	Last  time.Time `toml:"last"`
}

type key struct{ query, url string }

// History is the in-memory learned-ranking table loaded from history.toml.
type History struct {
	path    string
	entries map[key]*entry
	now     func() time.Time // injectable clock; tests override it
}

// tomlFile mirrors history.toml: just the array of [[entry]] tables.
type tomlFile struct {
	Entry []entry `toml:"entry"`
}

// Path returns the history file inside a config directory.
func Path(dir string) string { return filepath.Join(dir, fileName) }

// Load reads the history table from dir. A missing or unparseable file yields an
// empty (but usable) table — corrupt history is discarded, never fatal.
func Load(dir string) *History {
	h := &History{path: Path(dir), entries: map[key]*entry{}, now: time.Now}
	data, err := os.ReadFile(h.path)
	if err != nil {
		return h
	}
	var f tomlFile
	if _, err := toml.Decode(string(data), &f); err != nil {
		return h
	}
	for i := range f.Entry {
		e := f.Entry[i]
		h.entries[key{e.Query, e.URL}] = &e
	}
	return h
}

// Record credits url under the committed query (lowercased and trimmed): it
// bumps both the (query, url) entry and the query-independent global entry. A
// selection made with no typed query reinforces only the global entry.
func (h *History) Record(query, url string) {
	if h == nil || url == "" {
		return
	}
	q := normalize(query)
	now := h.now()
	h.bump(q, url, now)
	if q != "" {
		h.bump("", url, now)
	}
}

func (h *History) bump(q, url string, now time.Time) {
	k := key{q, url}
	e := h.entries[k]
	if e == nil {
		e = &entry{Query: q, URL: url}
		h.entries[k] = e
	}
	e.Rank++
	e.Last = now
}

// Scores returns, for the typed query, a url→boost map blending the query-keyed
// signal (stored queries that the typed query is a prefix of — so typing "e"
// already surfaces a habit committed at "en") with the global signal. An empty
// query uses only the global signal. nil-safe: a nil History scores nothing.
func (h *History) Scores(query string) map[string]float64 {
	out := map[string]float64{}
	if h == nil {
		return out
	}
	q := normalize(query)
	now := h.now()
	for _, e := range h.entries {
		f := h.frecency(e, now)
		switch {
		case e.Query == "":
			out[e.URL] += weightGlobal * f
		case q != "" && strings.HasPrefix(e.Query, q):
			out[e.URL] += weightQuery * f
		}
	}
	return out
}

// Save writes the table to history.toml atomically (temp file + rename), after
// an aging pass that prunes stale entries and scales ranks down past the ceiling.
func (h *History) Save() error {
	if h == nil {
		return nil
	}
	h.prune()

	var f tomlFile
	for _, e := range h.entries {
		f.Entry = append(f.Entry, *e)
	}
	// Deterministic on-disk order: by url, then query (global "" first).
	sort.Slice(f.Entry, func(i, j int) bool {
		if f.Entry[i].URL != f.Entry[j].URL {
			return f.Entry[i].URL < f.Entry[j].URL
		}
		return f.Entry[i].Query < f.Entry[j].Query
	})

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(f); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(h.path), 0o755); err != nil {
		return err
	}
	tmp := h.path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, h.path)
}

// prune drops entries whose decayed score has fallen below pruneFloor, then
// scales all ranks down proportionally if their sum exceeds maxTotalRank.
func (h *History) prune() {
	now := h.now()
	var total float64
	for k, e := range h.entries {
		if h.frecency(e, now) < pruneFloor {
			delete(h.entries, k)
			continue
		}
		total += e.Rank
	}
	if total > maxTotalRank {
		scale := maxTotalRank / total
		for _, e := range h.entries {
			e.Rank *= scale
		}
	}
}

// frecency is an entry's raw rank decayed by its age, zoxide-style.
func (h *History) frecency(e *entry, now time.Time) float64 {
	return e.Rank * decay(now.Sub(e.Last))
}

// decay is zoxide's bucketed recency multiplier.
func decay(age time.Duration) float64 {
	switch {
	case age < time.Hour:
		return 4
	case age < 24*time.Hour:
		return 2
	case age < 7*24*time.Hour:
		return 0.5
	default:
		return 0.25
	}
}

func normalize(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
