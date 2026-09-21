package tui

import (
	"strings"
	"testing"

	"bml/internal/browser"
	"bml/internal/config"

	tea "github.com/charmbracelet/bubbletea"
)

func browseModel() (Browse, *browser.Fake) {
	fake := &browser.Fake{}
	bms := []config.Bookmark{
		{Name: "GitHub", URL: "https://github.com"},
		{Name: "Gitlab", URL: "https://gitlab.com"},
		{Name: "Hacker News", URL: "https://news.ycombinator.com"},
	}
	m := NewBrowse(fake, bms, nil, true, config.DefaultSearch(), nil)
	return m, fake
}

// typeBrowse feeds a string of runes one message at a time, returning the model.
func typeBrowse(m Browse, s string) Browse {
	for _, r := range s {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(Browse)
	}
	return m
}

// loadHistoryInto runs the async history-load command and delivers its message,
// simulating the read landing after browse mode opens.
func loadHistoryInto(m Browse, fake *browser.Fake) Browse {
	cmd := loadHistory(fake)
	if cmd == nil {
		return m
	}
	next, _ := m.Update(cmd())
	return next.(Browse)
}

var errTest = errString("boom")

type errString string

func (e errString) Error() string { return string(e) }

func TestLeader_DotEntersBrowse(t *testing.T) {
	m, _ := newModel()
	next, _ := m.Update(runes("."))
	if _, ok := next.(Browse); !ok {
		t.Fatalf("\".\" should enter browse mode, got %T", next)
	}
}

func TestBrowse_EnterOnURLOpensIt(t *testing.T) {
	m, fake := browseModel()
	m = typeBrowse(m, "example.com")
	if m.cursor != 0 {
		t.Fatalf("cursor should rest on the primary action, got %d", m.cursor)
	}
	next, cmd := m.Update(enter())
	if _, ok := next.(Browse); !ok || cmd == nil {
		t.Fatal("Enter should produce an act command")
	}
	cmd()
	last, _ := fake.Last()
	if last.URL != "https://example.com" {
		t.Errorf("got %+v, want https://example.com (scheme defaulted)", last)
	}
	if last.ForceNew {
		t.Error("opening an address should focus-or-open, not force a new tab")
	}
}

func TestBrowse_EnterOnTermSearches(t *testing.T) {
	m, fake := browseModel()
	m = typeBrowse(m, "golang generics")
	next, cmd := m.Update(enter())
	if cmd == nil {
		t.Fatal("Enter on a search term should act")
	}
	_ = next
	cmd()
	last, _ := fake.Last()
	if !strings.Contains(last.URL, "google.com/search") || !strings.Contains(last.URL, "golang+generics") {
		t.Errorf("got %q, want a Google search URL for the query", last.URL)
	}
	if !last.ForceNew {
		t.Error("a web search should open in a new tab")
	}
}

func TestBrowse_BookmarkPrefixSuggestionsRankFirst(t *testing.T) {
	m, _ := browseModel()
	m = typeBrowse(m, "git")
	if len(m.suggestions) < 2 {
		t.Fatalf("expected bookmark suggestions for \"git\", got %+v", m.suggestions)
	}
	// The two git* bookmarks must come before any popular-domain rows.
	if !m.suggestions[0].isBookmark || !m.suggestions[1].isBookmark {
		t.Errorf("bookmark matches should rank ahead of popular domains, got %+v", m.suggestions)
	}
	for _, s := range m.suggestions[:2] {
		if !strings.HasPrefix(s.display, "git") {
			t.Errorf("suggestion %q is not a prefix match for \"git\"", s.display)
		}
	}
}

func TestBrowse_EnterOnSuggestionOpensThatURL(t *testing.T) {
	m, fake := browseModel()
	m = typeBrowse(m, "git")
	next, _ := m.Update(keyDown()) // move to the first suggestion
	m = next.(Browse)
	if m.cursor != 1 {
		t.Fatalf("Down should select the first suggestion, got cursor %d", m.cursor)
	}
	want := m.suggestions[0].url
	_, cmd := m.Update(enter())
	if cmd == nil {
		t.Fatal("Enter on a suggestion should act")
	}
	cmd()
	if last, _ := fake.Last(); last.URL != want || last.ForceNew {
		t.Errorf("got %+v, want {%s false}", last, want)
	}
}

func TestBrowse_HistorySuggestion(t *testing.T) {
	fake := &browser.Fake{Visits: []browser.Visit{
		{URL: "https://youtube.com", TypedCount: 236, VisitCount: 1342},
		{URL: "https://youtube.com/feed/subscriptions", TypedCount: 1, VisitCount: 40},
	}}
	m := NewBrowse(fake, nil, nil, false, config.DefaultSearch(), nil) // no bookmarks
	m = loadHistoryInto(m, fake)                                       // simulate the async read landing
	m = typeBrowse(m, "youtu")
	if len(m.suggestions) == 0 || m.suggestions[0].url != "https://youtube.com" {
		t.Fatalf("expected youtube.com from history first, got %+v", m.suggestions)
	}
	if m.suggestions[0].display != "youtube.com" {
		t.Errorf("history suggestion display = %q, want normalized host", m.suggestions[0].display)
	}
}

func TestBrowse_HistoryShowsTitle(t *testing.T) {
	fake := &browser.Fake{Visits: []browser.Visit{
		{URL: "https://youtube.com", Title: "YouTube", TypedCount: 236, VisitCount: 1342},
	}}
	m := NewBrowse(fake, nil, nil, false, config.DefaultSearch(), nil)
	m = loadHistoryInto(m, fake)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m = next.(Browse)
	m = typeBrowse(m, "youtu")
	view := m.View()
	if !strings.Contains(view, "YouTube") {
		t.Errorf("history suggestion should show its page title; view:\n%s", view)
	}
	if !strings.Contains(view, "youtube.com") {
		t.Errorf("history suggestion should still show its URL; view:\n%s", view)
	}
}

func TestBrowse_LongTitleClipped(t *testing.T) {
	long := strings.Repeat("x", 200)
	if got := clip(long, labelWidth); len([]rune(got)) != labelWidth {
		t.Errorf("clip should cap at %d runes, got %d", labelWidth, len([]rune(got)))
	}
	if got := clip("short", labelWidth); got != "short" {
		t.Errorf("clip should leave a short string untouched, got %q", got)
	}
}

func TestBrowse_BookmarkOutranksHistory(t *testing.T) {
	fake := &browser.Fake{Visits: []browser.Visit{
		{URL: "https://github.com/explore", TypedCount: 99, VisitCount: 99},
	}}
	bms := []config.Bookmark{{Name: "GitHub", URL: "https://github.com"}}
	m := NewBrowse(fake, bms, nil, true, config.DefaultSearch(), nil)
	m = loadHistoryInto(m, fake)
	m = typeBrowse(m, "git")
	if len(m.suggestions) == 0 || !m.suggestions[0].isBookmark {
		t.Fatalf("a bookmark must outrank even a heavily-typed history entry, got %+v", m.suggestions)
	}
}

func TestBrowse_FallsBackWhenHistoryUnavailable(t *testing.T) {
	fake := &browser.Fake{HistoryErr: errTest} // backend can't read history
	bms := []config.Bookmark{{Name: "GitHub", URL: "https://github.com"}}
	m := NewBrowse(fake, bms, nil, false, config.DefaultSearch(), nil)
	m = loadHistoryInto(m, fake)
	m = typeBrowse(m, "git")
	if len(m.suggestions) != 1 || !m.suggestions[0].isBookmark {
		t.Errorf("with no history, only the bookmark match should show, got %+v", m.suggestions)
	}
}

func TestBrowse_TabCompletesInput(t *testing.T) {
	m, _ := browseModel()
	m = typeBrowse(m, "git")
	want := m.suggestions[0].display
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Browse)
	if m.input.Value() != want {
		t.Errorf("Tab should complete the input to %q, got %q", want, m.input.Value())
	}
}

func TestBrowse_EscReturnsToLeader(t *testing.T) {
	m, _ := browseModel()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if _, ok := next.(Leader); !ok {
		t.Errorf("Esc should return to leader mode, got %T", next)
	}
}

func TestBrowse_EmptyEnterIsNoOp(t *testing.T) {
	m, fake := browseModel()
	_, cmd := m.Update(enter())
	if cmd != nil {
		t.Errorf("Enter on an empty input should do nothing, got %v", cmd())
	}
	if len(fake.Calls) != 0 {
		t.Errorf("empty Enter must not act, got %+v", fake.Calls)
	}
}

func TestLooksLikeURL(t *testing.T) {
	cases := map[string]bool{
		"example.com":         true,
		"https://example.com": true,
		"localhost":           true,
		"localhost:3000":      true,
		"github.com/foo/bar":  true,
		"hello world":         false,
		"golang generics":     false,
		"justaword":           false,
	}
	for in, want := range cases {
		if got := looksLikeURL(in); got != want {
			t.Errorf("looksLikeURL(%q) = %v, want %v", in, got, want)
		}
	}
}
