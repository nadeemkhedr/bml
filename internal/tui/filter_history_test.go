package tui

import (
	"testing"

	"bml/internal/history"
)

// Without history, the query "g" ranks GitHub above Go docs (shorter name wins
// the tie); a learned habit for "g" → Go docs must flip that.
func TestFilter_QueryHabitFloatsToTop(t *testing.T) {
	if got := names(Filter(corpus(), "g")); got[0] != "GitHub" {
		t.Fatalf("baseline: expected GitHub first for 'g', got %v", got)
	}

	h := history.Load(t.TempDir())
	for i := 0; i < 3; i++ {
		h.Record("g", "https://pkg.go.dev") // habitually pick Go docs for "g"
	}

	got := names(Filter(corpus(), "g", h))
	if got[0] != "Go docs" {
		t.Fatalf("a learned habit for 'g' should float Go docs to the top, got %v", got)
	}
}

// A bookmark that does not match the query must not appear, however strong its
// learned ranking — the match still gates.
func TestFilter_HabitDoesNotBypassMatchGate(t *testing.T) {
	h := history.Load(t.TempDir())
	for i := 0; i < 5; i++ {
		h.Record("zzz", "https://pkg.go.dev")
	}
	for _, r := range Filter(corpus(), "zzz", h) {
		if r.Bookmark.URL == "https://pkg.go.dev" {
			t.Fatal("a non-matching bookmark must not surface on learned ranking alone")
		}
	}
}

// Empty query orders by global frecency (most-used first) instead of file order.
func TestFilter_EmptyQuerySortsByGlobalFrecency(t *testing.T) {
	h := history.Load(t.TempDir())
	h.Record("", "https://pkg.go.dev") // Go docs is the most-used overall

	got := names(Filter(corpus(), "", h))
	if got[0] != "Go docs" {
		t.Fatalf("empty query should put the most-used bookmark first, got %v", got)
	}
}
