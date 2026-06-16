package tui

import (
	"fmt"
	"strings"

	"bml/internal/browser"
	"bml/internal/config"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// maxVisible caps how many results render at once; the window scrolls to keep
// the selection in view.
const maxVisible = 10

// Search is the fuzzy finder entered from leader mode with "/". It matches over
// name, url, and tags, and acts on the selected bookmark with Enter.
type Search struct {
	browser  browser.Browser
	all      []config.Bookmark
	input    textinput.Model
	results  []Result
	cursor   int
	offset   int
	err      error
	quitting bool
}

// NewSearch builds the search model over the full bookmark list.
func NewSearch(b browser.Browser, bookmarks []config.Bookmark) Search {
	in := textinput.New()
	in.Placeholder = "search bookmarks…"
	in.Prompt = ""
	in.Focus()

	m := Search{browser: b, all: bookmarks, input: in}
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

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEsc:
			// Back to leader mode.
			return NewLeader(m.browser, m.all), nil
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

// clamp keeps the cursor in range and the scroll window around it.
func (m *Search) clamp() {
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
	if m.cursor >= m.offset+maxVisible {
		m.offset = m.cursor - maxVisible + 1
	}
}

func (m Search) View() string {
	if m.quitting {
		return ""
	}
	var b strings.Builder
	b.WriteString(header("search") + "\n\n")

	// Query line: a prompt glyph + the live input.
	b.WriteString("  " + promptStr.Render("/ ") + m.input.View() + "\n")
	b.WriteString("  " + hintStyle.Render(fmt.Sprintf("%d result%s", len(m.results), plural(len(m.results)))) + "\n\n")

	if len(m.results) == 0 {
		b.WriteString("  " + hintStyle.Render("no matches") + "\n")
	}

	end := m.offset + maxVisible
	if end > len(m.results) {
		end = len(m.results)
	}
	for i := m.offset; i < end; i++ {
		b.WriteString(m.renderRow(m.results[i], i == m.cursor))
	}
	if len(m.results) > end {
		b.WriteString("  " + hintStyle.Render(fmt.Sprintf("… %d more", len(m.results)-end)) + "\n")
	}

	b.WriteString("\n" + hintStyle.Render("  ↑↓  move   ·   ↵  open   ·   esc  back   ·   ^c  quit") + "\n")
	return b.String()
}

func (m Search) renderRow(r Result, selected bool) string {
	name := highlight(r.Bookmark.Name, r.NameMatch, nameStyle, matchStyle)
	if selected {
		name = highlight(r.Bookmark.Name, r.NameMatch, selName, matchStyle)
	}

	marker := "  "
	if selected {
		marker = cursorBar.Render("▌ ")
	}

	line := marker + name + renderTags(r.Bookmark.Tags) + "\n"
	// URL on a dimmed second line, indented under the name.
	line += "    " + urlStyle.Render(r.Bookmark.URL) + "\n"
	return line
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
