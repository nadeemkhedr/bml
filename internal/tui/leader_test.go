package tui

import (
	"strings"
	"testing"

	"bml/internal/browser"
	"bml/internal/config"

	tea "github.com/charmbracelet/bubbletea"
)

func bookmarks() []config.Bookmark {
	return []config.Bookmark{
		{Key: "g", Name: "GitHub", URL: "https://github.com"},
		{Key: "n", Name: "Hacker News", URL: "https://news.ycombinator.com"},
		{Name: "Go docs", URL: "https://pkg.go.dev"}, // unkeyed
	}
}

func newModel() (Leader, *browser.Fake) {
	fake := &browser.Fake{}
	return NewLeader(fake, bookmarks(), nil, true, config.DefaultSearch(), nil), fake
}

// runes builds a KeyMsg for typed characters.
func runes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// press feeds a message and, if a command is returned, runs it and returns its
// message (for asserting acts/quits).
func press(t *testing.T, m Leader, msg tea.Msg) (Leader, tea.Msg) {
	t.Helper()
	next, cmd := m.Update(msg)
	lm := next.(Leader)
	if cmd == nil {
		return lm, nil
	}
	return lm, cmd()
}

func isQuit(msg tea.Msg) bool {
	_, ok := msg.(tea.QuitMsg)
	return ok
}

func TestNewLeader_KeepsOnlyKeyedBookmarks(t *testing.T) {
	m, _ := newModel()
	if len(m.favorites) != 2 {
		t.Fatalf("got %d favorites, want 2 (unkeyed excluded)", len(m.favorites))
	}
}

func TestLeader_LowercaseKeyFocuses(t *testing.T) {
	m, fake := newModel()
	_, msg := press(t, m, runes("g"))

	acted, ok := msg.(actedMsg)
	if !ok || acted.err != nil {
		t.Fatalf("expected a successful actedMsg, got %#v", msg)
	}
	last, _ := fake.Last()
	if last.URL != "https://github.com" || last.ForceNew {
		t.Errorf("got %+v, want {https://github.com false}", last)
	}
}

func TestLeader_UppercaseKeyForcesNewTab(t *testing.T) {
	m, fake := newModel()
	_, _ = press(t, m, runes("G"))

	last, ok := fake.Last()
	if !ok || last.URL != "https://github.com" || !last.ForceNew {
		t.Errorf("got %+v, want {https://github.com true}", last)
	}
}

func TestLeader_ActedMsgQuits(t *testing.T) {
	m, _ := newModel()
	_, cmd := m.Update(actedMsg{})
	if cmd == nil || !isQuit(cmd()) {
		t.Error("actedMsg should trigger quit (fire and exit)")
	}
}

func TestLeader_UnboundKeyDoesNothing(t *testing.T) {
	m, fake := newModel()
	_, cmd := m.Update(runes("x"))
	if cmd != nil {
		t.Errorf("unbound key should produce no command, got %v", cmd())
	}
	if len(fake.Calls) != 0 {
		t.Errorf("unbound key must not act, got %+v", fake.Calls)
	}
}

func TestLeader_QuitKeys(t *testing.T) {
	for _, msg := range []tea.Msg{
		runes("q"),
		tea.KeyMsg{Type: tea.KeyEsc},
		tea.KeyMsg{Type: tea.KeyCtrlC},
	} {
		m, fake := newModel()
		_, cmd := m.Update(msg)
		if cmd == nil || !isQuit(cmd()) {
			t.Errorf("%v should quit", msg)
		}
		if len(fake.Calls) != 0 {
			t.Errorf("quit must not act, got %+v", fake.Calls)
		}
	}
}

func TestLeader_ShowTagsTogglesTagDisplay(t *testing.T) {
	bms := []config.Bookmark{{Key: "g", Name: "GitHub", URL: "https://github.com", Tags: []string{"devtag"}}}
	sized := func(showTags bool) string {
		m := NewLeader(&browser.Fake{}, bms, nil, showTags, config.DefaultSearch(), nil)
		next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		return next.(Leader).View()
	}
	with := sized(true)
	without := sized(false)
	if !strings.Contains(with, "devtag") {
		t.Error("tags should appear in leader mode when showTags is true")
	}
	if strings.Contains(without, "devtag") {
		t.Error("tags should be hidden in leader mode when showTags is false")
	}
}

func TestLeader_BoundQuitKeyActsInsteadOfQuitting(t *testing.T) {
	fake := &browser.Fake{}
	m := NewLeader(fake, []config.Bookmark{{Key: "q", Name: "Queue", URL: "https://q.example"}}, nil, true, config.DefaultSearch(), nil)
	_, msg := press(t, m, runes("q"))
	if _, ok := msg.(actedMsg); !ok {
		t.Errorf("a bookmark bound to q should act, not quit; got %#v", msg)
	}
	if last, _ := fake.Last(); last.URL != "https://q.example" {
		t.Errorf("got %+v", last)
	}
}
