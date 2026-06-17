package tui

import (
	"testing"

	"bml/internal/browser"
	"bml/internal/config"

	tea "github.com/charmbracelet/bubbletea"
)

func newWebSearch() (WebSearch, *browser.Fake) {
	fake := &browser.Fake{}
	return NewWebSearch(fake, corpus(), nil, true, config.DefaultSearch()), fake
}

// typeWeb feeds each rune of s to the model, returning the updated model.
func typeWeb(t *testing.T, m WebSearch, s string) WebSearch {
	t.Helper()
	for _, r := range s {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(WebSearch)
	}
	return m
}

func TestWebSearch_EnterUsesPrimaryEngine(t *testing.T) {
	m, fake := newWebSearch()
	m = typeWeb(t, m, "go context")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on a query should produce an act command")
	}
	cmd()

	last, ok := fake.Last()
	if !ok || last.URL != "https://www.google.com/search?q=go+context" || !last.ForceNew {
		t.Errorf("got %+v, want google search in a new tab", last)
	}
}

func TestWebSearch_TabUsesSecondaryEngine(t *testing.T) {
	m, fake := newWebSearch()
	m = typeWeb(t, m, "go context")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if cmd == nil {
		t.Fatal("Tab on a query should produce an act command")
	}
	cmd()

	last, ok := fake.Last()
	if !ok || last.URL != "https://duckduckgo.com/?q=!ducky+go+context" || !last.ForceNew {
		t.Errorf("got %+v, want duckduckgo_lucky search in a new tab", last)
	}
}

func TestWebSearch_EmptyQueryIsNoOp(t *testing.T) {
	for _, key := range []tea.KeyType{tea.KeyEnter, tea.KeyTab} {
		m, fake := newWebSearch()
		_, cmd := m.Update(tea.KeyMsg{Type: key})
		if cmd != nil {
			t.Errorf("key %v: empty query should produce no command, got %v", key, cmd())
		}
		if len(fake.Calls) != 0 {
			t.Errorf("key %v: empty query must not act, got %+v", key, fake.Calls)
		}
	}
}

func TestWebSearch_WhitespaceQueryIsNoOp(t *testing.T) {
	m, fake := newWebSearch()
	m = typeWeb(t, m, "   ")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil || len(fake.Calls) != 0 {
		t.Errorf("whitespace-only query should not act, got cmd=%v calls=%+v", cmd, fake.Calls)
	}
}

func TestWebSearch_EscReturnsToLeader(t *testing.T) {
	m, _ := newWebSearch()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if _, ok := next.(Leader); !ok {
		t.Errorf("Esc should return a Leader model, got %T", next)
	}
}

func TestWebSearch_CtrlCQuits(t *testing.T) {
	m, _ := newWebSearch()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil || !isQuit(cmd()) {
		t.Error("Ctrl-C should quit")
	}
}

func TestLeader_SEntersWebSearch(t *testing.T) {
	for _, key := range []string{"s", "S"} {
		m, _ := newModel()
		next, _ := m.Update(runes(key))
		if _, ok := next.(WebSearch); !ok {
			t.Errorf("%q at the top level should enter web search, got %T", key, next)
		}
	}
}

func TestLeader_SInsideGroupDoesNotEnterWebSearch(t *testing.T) {
	m, _ := groupedModel()
	m = descend(t, m, "w") // enter the "w" group
	next, _ := m.Update(runes("s"))
	if _, ok := next.(WebSearch); ok {
		t.Error("s inside a group should be ignored, not enter web search")
	}
}
