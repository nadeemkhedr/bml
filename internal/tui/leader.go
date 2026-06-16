// Package tui holds bml's interactive Bubble Tea models: leader mode and (later)
// search mode.
package tui

import (
	"strings"
	"unicode"

	"bml/internal/browser"
	"bml/internal/config"

	tea "github.com/charmbracelet/bubbletea"
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
	tags           []string
}

// Leader is the which-key launcher shown when bml starts with no argument.
type Leader struct {
	browser   browser.Browser
	all       []config.Bookmark // full list, handed to search mode
	favorites []favorite
	byKey     map[string]favorite
	err       error // set when an act fails; surfaced by the caller after exit
	quitting  bool
}

// NewLeader builds the leader model from the bookmark list, keeping only the
// keyed bookmarks (favorites) in config order for display while retaining the
// full list for search mode.
func NewLeader(b browser.Browser, bookmarks []config.Bookmark) Leader {
	m := Leader{browser: b, all: bookmarks, byKey: make(map[string]favorite)}
	for _, bm := range bookmarks {
		if bm.Key == "" {
			continue
		}
		f := favorite{key: bm.Key, name: bm.Name, url: bm.URL, tags: bm.Tags}
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
		search := NewSearch(m.browser, m.all)
		return search, search.Init()
	}
	return m, nil
}

func isUpper(s string) bool {
	r := []rune(s)
	return len(r) == 1 && unicode.IsUpper(r[0])
}

func (m Leader) View() string {
	if m.quitting {
		return ""
	}
	var b strings.Builder
	b.WriteString(header("launcher") + "\n\n")
	if len(m.favorites) == 0 {
		b.WriteString(hintStyle.Render("  no favorites yet — add a key to a bookmark in `bml edit`") + "\n\n")
	}
	for _, f := range m.favorites {
		b.WriteString("  " + keyBadge.Render(f.key) + "  " + nameStyle.Render(f.name))
		b.WriteString(renderTags(f.tags) + "\n")
	}
	b.WriteString("\n" + hintStyle.Render("  Shift+key  new tab   ·   /  search   ·   q  quit") + "\n")
	return b.String()
}

// errer is implemented by both leader and search models so the runner can
// surface an act error regardless of which mode the program ended in.
type errer interface{ Err() error }

// RunLeader runs the interactive program (starting in leader mode) and returns
// any error from acting on a bookmark.
func RunLeader(b browser.Browser, bookmarks []config.Bookmark) error {
	final, err := tea.NewProgram(NewLeader(b, bookmarks)).Run()
	if err != nil {
		return err
	}
	if e, ok := final.(errer); ok {
		return e.Err()
	}
	return nil
}
