package tui

import (
	"fmt"
	"strings"

	"bml/internal/browser"
	"bml/internal/config"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// WebSearch is search mode: entered from leader mode with "s", it sends a
// free-text query to a web search engine rather than touching stored bookmarks.
// Enter dispatches the query to the primary engine, Tab to the secondary engine;
// either always opens a new tab and then exits (fire and exit).
//
// Tab — not Shift+Enter — drives the secondary engine because Bubble Tea v1
// cannot distinguish Shift+Enter from Enter (see ADR 0003).
type WebSearch struct {
	browser browser.Browser
	// Carried so returning to leader (Esc) rebuilds it identically.
	all      []config.Bookmark
	groups   []config.Group
	showTags bool
	search   config.Search

	input         textinput.Model
	width, height int
	err           error
	quitting      bool
}

// NewWebSearch builds the search-mode model from the resolved engine config.
func NewWebSearch(b browser.Browser, bookmarks []config.Bookmark, groups []config.Group, showTags bool, search config.Search) WebSearch {
	in := textinput.New()
	in.Placeholder = "search the web…"
	in.Prompt = ""
	in.Focus()

	return WebSearch{
		browser:  b,
		all:      bookmarks,
		groups:   groups,
		showTags: showTags,
		search:   search,
		input:    in,
	}
}

// Err returns any error from acting on a search URL.
func (m WebSearch) Err() error { return m.err }

func (m WebSearch) Init() tea.Cmd { return textinput.Blink }

func (m WebSearch) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case actedMsg:
		m.err = msg.err
		m.quitting = true
		return m, tea.Quit

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEsc:
			// Back to leader mode, carrying the known size (no resize needed).
			leader := NewLeader(m.browser, m.all, m.groups, m.showTags, m.search)
			leader.width, leader.height = m.width, m.height
			return leader, nil
		case tea.KeyEnter:
			return m.dispatch(m.search.Primary)
		case tea.KeyTab:
			return m.dispatch(m.search.Secondary)
		}

		// Anything else edits the query.
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

// dispatch acts on the query through the given engine, always in a new tab. An
// empty (or whitespace-only) query is a no-op.
func (m WebSearch) dispatch(e config.Engine) (tea.Model, tea.Cmd) {
	q := strings.TrimSpace(m.input.Value())
	if q == "" {
		return m, nil
	}
	return m, act(m.browser, e.URL(q), true)
}

func (m WebSearch) View() string {
	if m.quitting {
		return ""
	}
	head := []string{
		header("search"),
		"  " + promptStr.Render("s ") + m.input.View(),
		"",
	}
	body := []string{
		"  " + keyBadge.Render("↵") + "  " + nameStyle.Render("search with "+m.search.Primary.Name),
		"  " + keyBadge.Render("⇥") + "  " + nameStyle.Render("search with "+m.search.Secondary.Name),
	}
	foot := []string{"", hintStyle.Render(m.footer())}
	return frame(m.width, m.height, head, body, foot)
}

func (m WebSearch) footer() string {
	return fmt.Sprintf("  ↵  %s   ·   ⇥  %s   ·   esc  back   ·   ^c  quit",
		m.search.Primary.Name, m.search.Secondary.Name)
}
