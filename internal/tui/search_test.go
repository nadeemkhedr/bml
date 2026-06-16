package tui

import (
	"testing"

	"bml/internal/browser"

	tea "github.com/charmbracelet/bubbletea"
)

func newSearch() (Search, *browser.Fake) {
	fake := &browser.Fake{}
	return NewSearch(fake, corpus(), nil, true), fake
}

// typeQuery feeds each rune of s to the model, returning the updated model.
func typeQuery(t *testing.T, m Search, s string) Search {
	t.Helper()
	for _, r := range s {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(Search)
	}
	return m
}

func TestSearch_StartsWithAllResults(t *testing.T) {
	m, _ := newSearch()
	if len(m.results) != 3 {
		t.Fatalf("empty query should show all 3, got %d", len(m.results))
	}
}

func TestSearch_TypingFilters(t *testing.T) {
	m, _ := newSearch()
	m = typeQuery(t, m, "hub")
	if len(m.results) != 1 || m.results[0].Bookmark.Name != "GitHub" {
		t.Fatalf("typing 'hub' should narrow to GitHub, got %d results", len(m.results))
	}
}

func TestSearch_EnterActsOnSelection(t *testing.T) {
	m, fake := newSearch()
	m = typeQuery(t, m, "hub")

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on a result should produce an act command")
	}
	cmd() // execute the act
	_ = next

	last, ok := fake.Last()
	if !ok || last.URL != "https://github.com" || last.ForceNew {
		t.Errorf("got %+v, want {https://github.com false}", last)
	}
}

func TestSearch_EnterWithNoResultsDoesNothing(t *testing.T) {
	m, fake := newSearch()
	m = typeQuery(t, m, "zzzz")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("Enter with no results should do nothing")
	}
	if len(fake.Calls) != 0 {
		t.Errorf("no act expected, got %+v", fake.Calls)
	}
}

func TestSearch_EscReturnsToLeader(t *testing.T) {
	m, _ := newSearch()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if _, ok := next.(Leader); !ok {
		t.Errorf("Esc should return a Leader model, got %T", next)
	}
}

func TestSearch_DownMovesCursor(t *testing.T) {
	m, _ := newSearch()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Search)
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1 after Down", m.cursor)
	}
}

func TestSearch_DownActsOnSecondResult(t *testing.T) {
	m, fake := newSearch()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Search)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	cmd()
	if last, _ := fake.Last(); last.URL != "https://news.ycombinator.com" {
		t.Errorf("expected second result acted on, got %+v", last)
	}
}

func TestSearch_CtrlCQuits(t *testing.T) {
	m, _ := newSearch()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("Ctrl-C should quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("Ctrl-C should produce QuitMsg")
	}
}
