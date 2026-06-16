// Package tui holds bml's interactive Bubble Tea models: leader mode and (later)
// search mode.
package tui

import (
	"strings"
	"unicode"

	"bml/internal/browser"
	"bml/internal/config"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// actedMsg reports the result of acting on a URL; receiving it ends the program
// (fire and exit).
type actedMsg struct{ err error }

// act performs the side effect through the injected Browser as a Bubble Tea
// command, so Update stays a pure transition.
func act(b browser.Browser, url string, forceNew bool) tea.Cmd {
	return func() tea.Msg {
		return actedMsg{err: b.OpenOrFocus(url, forceNew)}
	}
}

type favorite struct {
	key, name, url string
}

// Leader is the which-key launcher shown when bml starts with no argument.
type Leader struct {
	browser   browser.Browser
	favorites []favorite
	byKey     map[string]favorite
	err       error // set when an act fails; surfaced by the caller after exit
	quitting  bool
}

// NewLeader builds the leader model from the bookmark list, keeping only the
// keyed bookmarks (favorites) in config order.
func NewLeader(b browser.Browser, bookmarks []config.Bookmark) Leader {
	m := Leader{browser: b, byKey: make(map[string]favorite)}
	for _, bm := range bookmarks {
		if bm.Key == "" {
			continue
		}
		f := favorite{key: bm.Key, name: bm.Name, url: bm.URL}
		m.favorites = append(m.favorites, f)
		m.byKey[bm.Key] = f
	}
	return m
}

// Err returns any error from acting on a bookmark.
func (m Leader) Err() error { return m.err }

func (m Leader) Init() tea.Cmd { return nil }

// Update handles key input. Resolution order on a rune: exact key match (focus),
// then uppercase-of-a-key (force new tab), then the quit/search shortcuts. Exact
// matches win so a bookmark bound to "q" or "/" still works.
func (m Leader) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case actedMsg:
		m.err = msg.err
		m.quitting = true
		return m, tea.Quit

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyRunes:
			return m.handleRune(string(msg.Runes))
		}
	}
	return m, nil
}

func (m Leader) handleRune(s string) (tea.Model, tea.Cmd) {
	// Exact key → focus-or-open.
	if f, ok := m.byKey[s]; ok {
		return m, act(m.browser, f.url, false)
	}
	// Uppercase of a bound letter → force a new tab.
	if isUpper(s) {
		if f, ok := m.byKey[strings.ToLower(s)]; ok {
			return m, act(m.browser, f.url, true)
		}
	}
	switch s {
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "/":
		// Search mode arrives in Phase 4.
		return m, nil
	}
	return m, nil
}

func isUpper(s string) bool {
	r := []rune(s)
	return len(r) == 1 && unicode.IsUpper(r[0])
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true)
	keyStyle   = lipgloss.NewStyle().Bold(true)
	hintStyle  = lipgloss.NewStyle().Faint(true)
)

func (m Leader) View() string {
	if m.quitting {
		return ""
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("bml") + "\n\n")
	if len(m.favorites) == 0 {
		b.WriteString(hintStyle.Render("  no favorites yet — add a key to a bookmark in `bml edit`") + "\n\n")
	}
	for _, f := range m.favorites {
		b.WriteString("  " + keyStyle.Render(f.key) + "   " + f.name + "\n")
	}
	b.WriteString("\n" + hintStyle.Render("  Shift+key new tab   /  search   q  quit") + "\n")
	return b.String()
}

// RunLeader runs the leader-mode program and returns any error from acting on a
// bookmark.
func RunLeader(b browser.Browser, bookmarks []config.Bookmark) error {
	final, err := tea.NewProgram(NewLeader(b, bookmarks)).Run()
	if err != nil {
		return err
	}
	if lm, ok := final.(Leader); ok {
		return lm.Err()
	}
	return nil
}
