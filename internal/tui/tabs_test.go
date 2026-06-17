package tui

import (
	"strings"
	"testing"

	"bml/internal/browser"
	"bml/internal/config"

	tea "github.com/charmbracelet/bubbletea"
)

func sampleTabs() []browser.Tab {
	return []browser.Tab{
		{Title: "GitHub", URL: "https://github.com/me/bml"},
		{Title: "Hacker News", URL: "https://news.ycombinator.com"},
		{Title: "Go docs", URL: "https://pkg.go.dev"},
	}
}

// newTabs builds a tab-mode model over a Fake preloaded with tabs, already past
// the async load (as if tabsLoadedMsg had arrived).
func newTabs(tabs []browser.Tab) (Tabs, *browser.Fake) {
	fake := &browser.Fake{Tabs: tabs}
	m := NewTabs(fake, fake, corpus(), nil, true, config.DefaultSearch())
	next, _ := m.Update(tabsLoadedMsg{tabs: tabs})
	return next.(Tabs), fake
}

// typeTabs feeds each rune of s to the model.
func typeTabs(t *testing.T, m Tabs, s string) Tabs {
	t.Helper()
	for _, r := range s {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(Tabs)
	}
	return m
}

func TestTabs_LoadPopulatesResults(t *testing.T) {
	m, _ := newTabs(sampleTabs())
	if len(m.results) != 3 {
		t.Fatalf("got %d results, want 3", len(m.results))
	}
}

func TestTabs_EnterFocusesSelectedTab(t *testing.T) {
	m, fake := newTabs(sampleTabs())
	// Move to the second tab and focus it.
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Tabs)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on a tab should produce an act command")
	}
	cmd()

	last, ok := fake.Last()
	if !ok || last.URL != "https://news.ycombinator.com" || last.ForceNew {
		t.Errorf("got %+v, want news focused (not force-new)", last)
	}
}

func TestTabs_FilterNarrowsAndFocuses(t *testing.T) {
	m, fake := newTabs(sampleTabs())
	m = typeTabs(t, m, "hacker")
	if len(m.results) != 1 {
		t.Fatalf("got %d results for 'hacker', want 1", len(m.results))
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should act on the single match")
	}
	cmd()
	if last, _ := fake.Last(); last.URL != "https://news.ycombinator.com" {
		t.Errorf("got %+v, want news", last)
	}
}

func TestTabs_FilterMatchesFriendlyURL(t *testing.T) {
	m, _ := newTabs(sampleTabs())
	// "pkg.go" only appears in the URL, not a title.
	m = typeTabs(t, m, "pkg.go")
	if len(m.results) != 1 || m.results[0].Tab.Title != "Go docs" {
		t.Fatalf("URL filter failed: %+v", m.results)
	}
}

func TestTabs_EnterWithNoMatchesIsNoOp(t *testing.T) {
	m, fake := newTabs(sampleTabs())
	m = typeTabs(t, m, "zzzzz")
	if len(m.results) != 0 {
		t.Fatalf("expected no matches, got %d", len(m.results))
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil || len(fake.Calls) != 0 {
		t.Errorf("Enter with no matches must not act, cmd=%v calls=%+v", cmd, fake.Calls)
	}
}

func TestTabs_EscReturnsToLeader(t *testing.T) {
	m, _ := newTabs(sampleTabs())
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if _, ok := next.(Leader); !ok {
		t.Errorf("Esc should return a Leader model, got %T", next)
	}
}

func TestTabs_CtrlCQuits(t *testing.T) {
	m, _ := newTabs(sampleTabs())
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil || !isQuit(cmd()) {
		t.Error("Ctrl-C should quit")
	}
}

func TestTabs_LoadingState(t *testing.T) {
	fake := &browser.Fake{Tabs: sampleTabs()}
	m := NewTabs(fake, fake, corpus(), nil, true, config.DefaultSearch())
	m.width, m.height = 80, 24
	if !strings.Contains(m.View(), "loading tabs…") {
		t.Error("before load, the view should show a loading state")
	}
}

func TestTabs_NotRunningState(t *testing.T) {
	fake := &browser.Fake{TabsErr: browser.ErrBrowserNotRunning}
	m := NewTabs(fake, fake, corpus(), nil, true, config.DefaultSearch())
	m.width, m.height = 80, 24
	next, _ := m.Update(tabsLoadedMsg{err: browser.ErrBrowserNotRunning})
	if !strings.Contains(next.(Tabs).View(), "isn't running") {
		t.Error("a not-running browser should show the not-running state")
	}
}

func TestTabs_ZeroTabsState(t *testing.T) {
	fake := &browser.Fake{}
	m := NewTabs(fake, fake, corpus(), nil, true, config.DefaultSearch())
	m.width, m.height = 80, 24
	next, _ := m.Update(tabsLoadedMsg{tabs: nil})
	if !strings.Contains(next.(Tabs).View(), "no open tabs") {
		t.Error("a running browser with no tabs should show 'no open tabs'")
	}
}

func TestTabs_AutomationDeniedState(t *testing.T) {
	fake := &browser.Fake{TabsErr: browser.ErrAutomationDenied}
	m := NewTabs(fake, fake, corpus(), nil, true, config.DefaultSearch())
	m.width, m.height = 80, 24
	next, _ := m.Update(tabsLoadedMsg{err: browser.ErrAutomationDenied})
	// The full guidance is long; on an 80-col terminal frame truncates the tail,
	// but the leading "permission denied" always shows.
	if !strings.Contains(next.(Tabs).View(), "permission denied") {
		t.Error("automation denial should surface the permission error")
	}
}

func TestTabs_BlankTitleFallsBackToURL(t *testing.T) {
	m, _ := newTabs([]browser.Tab{{Title: "", URL: "https://example.com/loading"}})
	m.width, m.height = 80, 24
	if !strings.Contains(m.View(), "example.com/loading") {
		t.Error("a blank-title tab should render its friendly URL")
	}
}

func TestFriendlyURL(t *testing.T) {
	cases := map[string]string{
		"https://www.github.com/me/bml/": "github.com/me/bml",
		"http://news.ycombinator.com":    "news.ycombinator.com",
		"https://pkg.go.dev/":            "pkg.go.dev",
	}
	for in, want := range cases {
		if got := friendlyURL(in); got != want {
			t.Errorf("friendlyURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLeader_TabEntersTabMode(t *testing.T) {
	m, _ := newModel()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if _, ok := next.(Tabs); !ok {
		t.Errorf("Tab at the top level should enter tab mode, got %T", next)
	}
}

func TestLeader_TabInsideGroupDoesNotEnterTabMode(t *testing.T) {
	m, _ := groupedModel()
	m = descend(t, m, "w") // enter the "w" group
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if _, ok := next.(Tabs); ok {
		t.Error("Tab inside a group should be ignored, not enter tab mode")
	}
}
