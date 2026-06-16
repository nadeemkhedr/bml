package tui

import (
	"testing"

	"bml/internal/browser"
	"bml/internal/config"

	tea "github.com/charmbracelet/bubbletea"
)

func groupedModel() (Leader, *browser.Fake) {
	fake := &browser.Fake{}
	bms := []config.Bookmark{
		{Key: "g", Name: "GitHub", URL: "https://github.com"},
		{Key: "wt", Name: "Work Tasks", URL: "https://tasks.example"},
		{Key: "wc", Name: "Work Calendar", URL: "https://cal.example"},
	}
	groups := []config.Group{{Key: "w", Name: "Work"}}
	return NewLeader(fake, bms, groups, true), fake
}

func descend(t *testing.T, m Leader, s string) Leader {
	t.Helper()
	next, cmd := m.Update(runes(s))
	if cmd != nil {
		t.Fatalf("pressing %q should descend, not act", s)
	}
	return next.(Leader)
}

func TestLeader_TopLevelShowsLeafAndGroup(t *testing.T) {
	m, _ := groupedModel()
	kids := m.children()
	if len(kids) != 2 {
		t.Fatalf("top level should have g (leaf) and w (group), got %d", len(kids))
	}
	byCh := map[string]child{}
	for _, c := range kids {
		byCh[c.ch] = c
	}
	if !byCh["g"].leaf {
		t.Error("g should be a leaf")
	}
	if byCh["w"].leaf || byCh["w"].name != "Work" {
		t.Errorf("w should be a group labeled Work, got %+v", byCh["w"])
	}
}

func TestLeader_DescendIntoGroupThenAct(t *testing.T) {
	m, fake := groupedModel()
	m = descend(t, m, "w")
	if m.prefix != "w" {
		t.Fatalf("prefix = %q, want w", m.prefix)
	}
	if m.breadcrumb() != "Work" {
		t.Errorf("breadcrumb = %q, want Work", m.breadcrumb())
	}
	if len(m.children()) != 2 { // t and c
		t.Fatalf("group should show 2 children, got %d", len(m.children()))
	}

	_, msg := press(t, m, runes("t"))
	if _, ok := msg.(actedMsg); !ok {
		t.Fatalf("completing the sequence should act, got %#v", msg)
	}
	if last, _ := fake.Last(); last.URL != "https://tasks.example" || last.ForceNew {
		t.Errorf("got %+v, want {https://tasks.example false}", last)
	}
}

func TestLeader_UppercaseFinalForcesNewTab(t *testing.T) {
	m, fake := groupedModel()
	m = descend(t, m, "w")
	press(t, m, runes("T"))
	if last, _ := fake.Last(); last.URL != "https://tasks.example" || !last.ForceNew {
		t.Errorf("got %+v, want {https://tasks.example true}", last)
	}
}

func TestLeader_EscLeavesGroupThenQuits(t *testing.T) {
	m, _ := groupedModel()
	m = descend(t, m, "w")

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Leader)
	if m.prefix != "" {
		t.Errorf("Esc in a group should return to top, prefix = %q", m.prefix)
	}
	if cmd != nil {
		t.Error("Esc in a group should not quit")
	}

	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil || !isQuit(cmd()) {
		t.Error("Esc at top should quit")
	}
}

func TestLeader_BackspacePopsOneChar(t *testing.T) {
	m, _ := groupedModel()
	m = descend(t, m, "w")
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if next.(Leader).prefix != "" {
		t.Errorf("Backspace should pop the group prefix")
	}
}

func TestLeader_StrayKeyInGroupIsIgnored(t *testing.T) {
	m, fake := groupedModel()
	m = descend(t, m, "w")
	next, cmd := m.Update(runes("x"))
	if cmd != nil {
		t.Error("an unmatched key inside a group should do nothing")
	}
	if next.(Leader).prefix != "w" {
		t.Error("prefix should stay in the group after a stray key")
	}
	if len(fake.Calls) != 0 {
		t.Errorf("no act expected, got %+v", fake.Calls)
	}
}

func TestLeader_GroupPrefixDoesNotActAtTop(t *testing.T) {
	m, fake := groupedModel()
	next, cmd := m.Update(runes("w"))
	if cmd != nil {
		t.Error("pressing a group prefix should not act")
	}
	_ = next
	if len(fake.Calls) != 0 {
		t.Errorf("no act expected, got %+v", fake.Calls)
	}
}
