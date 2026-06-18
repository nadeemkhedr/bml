package tui

import (
	"strings"
	"testing"

	"bml/internal/browser"
	"bml/internal/config"

	tea "github.com/charmbracelet/bubbletea"
)

func keyDown() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyDown} }
func keyUp() tea.KeyMsg   { return tea.KeyMsg{Type: tea.KeyUp} }
func enter() tea.KeyMsg   { return tea.KeyMsg{Type: tea.KeyEnter} }

func TestLeader_EnterIsLatentUntilAnArrowPress(t *testing.T) {
	m, fake := newModel()
	_, cmd := m.Update(enter())
	if cmd != nil {
		t.Errorf("Enter with no selection should do nothing, got %v", cmd())
	}
	if len(fake.Calls) != 0 {
		t.Errorf("latent Enter must not act, got %+v", fake.Calls)
	}
}

func TestLeader_DownSelectsFirstLeafThenEnterFocuses(t *testing.T) {
	m, fake := newModel()
	next, _ := m.Update(keyDown())
	m = next.(Leader)
	if !m.cursorActive || m.cursor != 0 {
		t.Fatalf("first Down should reveal the first leaf, got active=%v cursor=%d", m.cursorActive, m.cursor)
	}

	_, msg := press(t, m, enter())
	if _, ok := msg.(actedMsg); !ok {
		t.Fatalf("Enter on a selection should act, got %#v", msg)
	}
	// leaves are sorted by key: g (GitHub), n (Hacker News).
	if last, _ := fake.Last(); last.URL != "https://github.com" || last.ForceNew {
		t.Errorf("got %+v, want {https://github.com false}", last)
	}
}

func TestLeader_UpRevealsLastLeaf(t *testing.T) {
	m, fake := newModel()
	next, _ := m.Update(keyUp())
	m = next.(Leader)
	if !m.cursorActive || m.cursor != 1 {
		t.Fatalf("first Up should reveal the last leaf, got active=%v cursor=%d", m.cursorActive, m.cursor)
	}
	_, _ = press(t, m, enter())
	if last, _ := fake.Last(); last.URL != "https://news.ycombinator.com" {
		t.Errorf("got %+v, want the Hacker News url", last)
	}
}

func TestLeader_CursorClampsWithoutWrapping(t *testing.T) {
	m, _ := newModel()
	// Down then Up past the top stays on the first leaf (no wrap to the end).
	next, _ := m.Update(keyDown())
	next, _ = next.(Leader).Update(keyUp())
	if c := next.(Leader).cursor; c != 0 {
		t.Errorf("Up at the top should clamp to 0, got %d", c)
	}
	// Stepping Down past the end stays on the last leaf.
	m2, _ := newModel()
	cur := m2
	for i := 0; i < 5; i++ {
		nx, _ := cur.Update(keyDown())
		cur = nx.(Leader)
	}
	if cur.cursor != 1 {
		t.Errorf("Down past the end should clamp to the last leaf (1), got %d", cur.cursor)
	}
}

func TestLeader_TypedKeyActsImmediatelyIgnoringCursor(t *testing.T) {
	m, fake := newModel()
	next, _ := m.Update(keyDown()) // cursor on g
	m = next.(Leader)

	_, msg := press(t, m, runes("n")) // typing n should fire Hacker News, not g
	if _, ok := msg.(actedMsg); !ok {
		t.Fatalf("a typed key should act immediately, got %#v", msg)
	}
	if last, _ := fake.Last(); last.URL != "https://news.ycombinator.com" {
		t.Errorf("typed key should act on its own bookmark, got %+v", last)
	}
}

func TestLeader_SelectionResetsWhenDescendingIntoGroup(t *testing.T) {
	m, _ := groupedModel()
	next, _ := m.Update(keyDown()) // reveal selection at top
	m = next.(Leader)
	if !m.cursorActive {
		t.Fatal("Down should reveal the selection")
	}
	m = descend(t, m, "w")
	if m.cursorActive || m.cursor != 0 || m.offset != 0 {
		t.Errorf("descending should reset to a latent selection, got active=%v cursor=%d offset=%d",
			m.cursorActive, m.cursor, m.offset)
	}
	if m.selectedKey() != "" {
		t.Errorf("latent selection should have no key, got %q", m.selectedKey())
	}
}

func TestLeader_SelectionResetsOnEscBackToTop(t *testing.T) {
	m, _ := groupedModel()
	m = descend(t, m, "w")
	next, _ := m.Update(keyDown()) // select a leaf inside the group
	m = next.(Leader)
	if !m.cursorActive {
		t.Fatal("Down inside a group should reveal the selection")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Leader)
	if m.prefix != "" || m.cursorActive {
		t.Errorf("Esc to top should reset the selection, prefix=%q active=%v", m.prefix, m.cursorActive)
	}
}

func TestLeader_BrowseReachesGroupedLeaf(t *testing.T) {
	m, fake := groupedModel()
	// Leaves in render order: g, wc, wt. Step to wc (index 1) and open it.
	next, _ := m.Update(keyDown())
	next, _ = next.(Leader).Update(keyDown())
	m = next.(Leader)
	if m.selectedKey() != "wc" {
		t.Fatalf("expected to land on wc, got %q", m.selectedKey())
	}
	_, _ = press(t, m, enter())
	if last, _ := fake.Last(); last.URL != "https://cal.example" || last.ForceNew {
		t.Errorf("got %+v, want {https://cal.example false}", last)
	}
}

func TestLeader_ScrollKeepsSelectionVisible(t *testing.T) {
	fake := &browser.Fake{}
	var bms []config.Bookmark
	for _, k := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		bms = append(bms, config.Bookmark{Key: k, Name: "Name " + k, URL: "https://" + k + ".example"})
	}
	m := NewLeader(fake, bms, nil, false, config.DefaultSearch())
	// A short screen: header(2)+footer(2) = 4 chrome, so only a few body lines fit.
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 9})
	m = next.(Leader)
	if vis := m.visibleCount(); vis >= len(bms) {
		t.Fatalf("test needs an overflowing menu: visible=%d leaves=%d", vis, len(bms))
	}

	// Walk to the last leaf; the window must scroll so it stays visible.
	cur := m
	for i := 0; i < len(bms); i++ {
		nx, _ := cur.Update(keyDown())
		cur = nx.(Leader)
	}
	if cur.cursor != len(bms)-1 {
		t.Fatalf("cursor should be on the last leaf, got %d", cur.cursor)
	}
	if cur.offset == 0 {
		t.Errorf("window should have scrolled (offset > 0), got 0")
	}
	if !strings.Contains(cur.View(), "Name h") {
		t.Error("the selected last leaf should be within the visible window")
	}
	if !strings.Contains(cur.View(), "▌") {
		t.Error("the cursor bar should be rendered on the selection")
	}
}
