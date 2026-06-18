package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fixedClock returns a History whose clock is pinned to base, so decay and
// pruning are deterministic.
func newAt(t *testing.T, base time.Time) *History {
	t.Helper()
	h := Load(t.TempDir())
	h.now = func() time.Time { return base }
	return h
}

var base = time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)

func TestRecord_WritesQueryAndGlobalEntries(t *testing.T) {
	h := newAt(t, base)
	h.Record("en", "https://example.com")

	if got := len(h.entries); got != 2 {
		t.Fatalf("expected a (query,url) and a global entry, got %d entries", got)
	}
	if h.entries[key{"en", "https://example.com"}] == nil {
		t.Error("missing query-keyed entry")
	}
	if h.entries[key{"", "https://example.com"}] == nil {
		t.Error("missing global entry")
	}
}

func TestRecord_EmptyQueryReinforcesGlobalOnly(t *testing.T) {
	h := newAt(t, base)
	h.Record("   ", "https://example.com") // whitespace normalizes to empty

	if got := len(h.entries); got != 1 {
		t.Fatalf("empty query should write only the global entry, got %d", got)
	}
	if h.entries[key{"", "https://example.com"}] == nil {
		t.Error("missing global entry")
	}
}

func TestScores_QueryKeyedDominatesGlobal(t *testing.T) {
	h := newAt(t, base)
	// "habit" was chosen once for "en"; "popular" chosen 3x but never for "en".
	h.Record("en", "https://habit.com")
	for i := 0; i < 3; i++ {
		h.Record("docs", "https://popular.com")
	}

	scores := h.Scores("en")
	if scores["https://habit.com"] <= scores["https://popular.com"] {
		t.Fatalf("query-keyed habit should outrank a globally popular but unrelated pick: habit=%g popular=%g",
			scores["https://habit.com"], scores["https://popular.com"])
	}
}

func TestScores_PrefixCompatible(t *testing.T) {
	h := newAt(t, base)
	h.Record("ent", "https://example.com") // committed at "ent"

	// Typing fewer chars ("en") picks up the query-keyed signal on top of the
	// global one; a divergent query ("entz") gets only the global signal. So the
	// prefix-compatible query must score strictly higher.
	global := h.Scores("entz")["https://example.com"]
	if global == 0 {
		t.Fatal("global signal should always contribute")
	}
	if h.Scores("en")["https://example.com"] <= global {
		t.Error("typing a prefix of a committed query should add the query-keyed boost")
	}
}

func TestScores_EmptyQueryUsesGlobalOnly(t *testing.T) {
	h := newAt(t, base)
	h.Record("en", "https://example.com")

	// Empty query must ignore the query-keyed signal (else it double-counts);
	// only the global entry contributes.
	want := weightGlobal * h.frecency(h.entries[key{"", "https://example.com"}], base)
	if got := h.Scores("")["https://example.com"]; got != want {
		t.Fatalf("empty-query score = %g, want global-only %g", got, want)
	}
}

func TestScores_NilSafe(t *testing.T) {
	var h *History
	if got := h.Scores("en"); len(got) != 0 {
		t.Errorf("nil History should score nothing, got %v", got)
	}
}

func TestDecay_Buckets(t *testing.T) {
	cases := []struct {
		age  time.Duration
		want float64
	}{
		{30 * time.Minute, 4},
		{5 * time.Hour, 2},
		{3 * 24 * time.Hour, 0.5},
		{30 * 24 * time.Hour, 0.25},
	}
	for _, c := range cases {
		if got := decay(c.age); got != c.want {
			t.Errorf("decay(%v) = %g, want %g", c.age, got, c.want)
		}
	}
}

func TestPrune_DropsStaleSinglePicks(t *testing.T) {
	h := newAt(t, base)
	h.Record("old", "https://stale.com")
	// Age the stale pick past a week so its decayed score (1 * 0.25) < pruneFloor.
	for k := range h.entries {
		h.entries[k].Last = base.Add(-14 * 24 * time.Hour)
	}
	h.Record("fresh", "https://kept.com") // recent, must survive

	h.prune()

	if h.entries[key{"old", "https://stale.com"}] != nil {
		t.Error("a stale single pick should be pruned")
	}
	if h.entries[key{"fresh", "https://kept.com"}] == nil {
		t.Error("a fresh pick should be kept")
	}
}

func TestPrune_ScalesDownPastCeiling(t *testing.T) {
	h := newAt(t, base)
	h.entries[key{"q", "https://a.com"}] = &entry{Query: "q", URL: "https://a.com", Rank: maxTotalRank * 2, Last: base}

	h.prune()

	if got := h.entries[key{"q", "https://a.com"}].Rank; got > maxTotalRank {
		t.Errorf("rank %g should have been scaled to within the ceiling %d", got, maxTotalRank)
	}
}

func TestSaveLoad_RoundTrips(t *testing.T) {
	dir := t.TempDir()
	h := Load(dir)
	h.now = func() time.Time { return base }
	h.Record("en", "https://example.com")
	if err := h.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, fileName)); err != nil {
		t.Fatalf("history file not written: %v", err)
	}

	reloaded := Load(dir)
	reloaded.now = func() time.Time { return base }
	if got := reloaded.Scores("en")["https://example.com"]; got == 0 {
		t.Error("reloaded history lost its learned score")
	}
}

func TestLoad_MissingFileIsEmptyNotError(t *testing.T) {
	h := Load(t.TempDir())
	if h == nil || len(h.entries) != 0 {
		t.Fatal("missing history should load as an empty, usable table")
	}
}

func TestLoad_CorruptFileIsDiscarded(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(Path(dir), []byte("this is not valid toml = ="), 0o644); err != nil {
		t.Fatal(err)
	}
	h := Load(dir)
	if h == nil || len(h.entries) != 0 {
		t.Error("corrupt history should be discarded, yielding an empty table")
	}
}
