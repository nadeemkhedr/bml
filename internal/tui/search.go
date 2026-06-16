package tui

import (
	"fmt"

	"bml/internal/browser"
	"bml/internal/config"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// defaultVisible is used before the window size is known.
const defaultVisible = 10

// searchChrome is the number of non-result rows in the view (header lines +
// footer lines): logo, query, count, blank, blank, hint.
const searchChrome = 6

// Search is the fuzzy finder entered from leader mode with "/". It matches over
// name, url, and tags, and acts on the selected bookmark with Enter.
type Search struct {
	browser       browser.Browser
	all           []config.Bookmark
	groups        []config.Group
	showTags      bool // carried so returning to leader preserves the setting
	input         textinput.Model
	results       []Result
	cursor        int
	offset        int
	width, height int
	err           error
	quitting      bool
}

// NewSearch builds the search model over the full bookmark list.
func NewSearch(b browser.Browser, bookmarks []config.Bookmark, groups []config.Group, showTags bool) Search {
	in := textinput.New()
	in.Placeholder = "search bookmarks…"
	in.Prompt = ""
	in.Focus()

	m := Search{browser: b, all: bookmarks, groups: groups, showTags: showTags, input: in}
	m.results = Filter(bookmarks, "")
	return m
}

// Err returns any error from acting on a bookmark.
func (m Search) Err() error { return m.err }

func (m Search) Init() tea.Cmd { return textinput.Blink }

func (m Search) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case actedMsg:
		m.err = msg.err
		m.quitting = true
		return m, tea.Quit

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clamp()
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEsc:
			// Back to leader mode, carrying the known size (no resize needed).
			leader := NewLeader(m.browser, m.all, m.groups, m.showTags)
			leader.width, leader.height = m.width, m.height
			return leader, nil
		case tea.KeyEnter:
			if len(m.results) > 0 {
				return m, act(m.browser, m.results[m.cursor].Bookmark.URL, false)
			}
			return m, nil
		case tea.KeyUp, tea.KeyCtrlP:
			m.move(-1)
			return m, nil
		case tea.KeyDown, tea.KeyCtrlN:
			m.move(1)
			return m, nil
		}

		// Anything else edits the query.
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.results = Filter(m.all, m.input.Value())
		m.clamp()
		return m, cmd
	}
	return m, nil
}

func (m *Search) move(delta int) {
	m.cursor += delta
	m.clamp()
}

// visibleCount is how many results fit in the body (each result is 2 rows).
func (m Search) visibleCount() int {
	if m.height <= 0 {
		return defaultVisible
	}
	n := (m.height - searchChrome) / 2
	if n < 1 {
		n = 1
	}
	return n
}

// clamp keeps the cursor in range and the scroll window around it.
func (m *Search) clamp() {
	vis := m.visibleCount()
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor > len(m.results)-1 {
		m.cursor = len(m.results) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+vis {
		m.offset = m.cursor - vis + 1
	}
}

func (m Search) View() string {
	if m.quitting {
		return ""
	}
	head := []string{
		header("search"),
		"  " + promptStr.Render("/ ") + m.input.View(),
		"  " + hintStyle.Render(fmt.Sprintf("%d result%s", len(m.results), plural(len(m.results)))),
		"",
	}
	foot := []string{"", hintStyle.Render("  ↑↓  move   ·   ↵  open   ·   esc  back   ·   ^c  quit")}

	var body []string
	if len(m.results) == 0 {
		body = []string{"  " + hintStyle.Render("no matches")}
	} else {
		end := m.offset + m.visibleCount()
		if end > len(m.results) {
			end = len(m.results)
		}
		for i := m.offset; i < end; i++ {
			body = append(body, m.renderRowLines(m.results[i], i == m.cursor)...)
		}
	}
	return frame(m.width, m.height, head, body, foot)
}

// renderRowLines returns a result as two lines: name (with tags) and a dimmed
// URL indented underneath.
func (m Search) renderRowLines(r Result, selected bool) []string {
	base := nameStyle
	if selected {
		base = selName
	}
	name := highlight(r.Bookmark.Name, r.NameMatch, base, matchStyle)

	marker := "  "
	if selected {
		marker = cursorBar.Render("▌ ")
	}
	return []string{
		marker + name + renderTagsMatch(r.Bookmark.Tags, r.TagMatch),
		"    " + highlight(r.Bookmark.URL, r.URLMatch, urlStyle, matchStyle),
	}
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
