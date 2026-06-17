package tui

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"bml/internal/browser"
	"bml/internal/config"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// tabsChrome is the number of non-result rows in tab mode (header, query, count,
// blank, and two footer lines). Tab rows are a single line each.
const tabsChrome = 6

// tabsLoadedMsg delivers the result of listing the browser's open tabs. Listing
// is async (it shells out to osascript) so the model shows a loading state until
// this arrives.
type tabsLoadedMsg struct {
	tabs []browser.Tab
	err  error
}

// Tabs is tab mode: the switcher entered from leader mode with the Tab key. It
// lists the configured browser's open tabs, fuzzy-filters them over title and
// friendly URL, and focuses the selected one with Enter (reusing the act-on-a-URL
// path), then exits. A pure switcher — it never opens, closes, or rearranges tabs.
type Tabs struct {
	browser browser.Browser
	lister  browser.TabLister
	// Carried so returning to leader (Esc) rebuilds it identically.
	all      []config.Bookmark
	groups   []config.Group
	showTags bool
	search   config.Search

	input         textinput.Model
	tabs          []browser.Tab
	results       []TabResult
	cursor        int
	offset        int
	loading       bool
	loadErr       error
	width, height int
	err           error
	quitting      bool
}

// NewTabs builds the tab-mode model. The lister enumerates open tabs; the browser
// focuses the chosen one. Both are carried because focusing reuses OpenOrFocus.
func NewTabs(b browser.Browser, lister browser.TabLister, bookmarks []config.Bookmark, groups []config.Group, showTags bool, search config.Search) Tabs {
	in := textinput.New()
	in.Placeholder = "filter tabs…"
	in.Prompt = ""
	in.Focus()

	return Tabs{
		browser:  b,
		lister:   lister,
		all:      bookmarks,
		groups:   groups,
		showTags: showTags,
		search:   search,
		input:    in,
		loading:  true,
	}
}

// Err returns any error from acting on a tab.
func (m Tabs) Err() error { return m.err }

func (m Tabs) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, loadTabs(m.lister))
}

// loadTabs enumerates open tabs off the Update loop and reports the result.
func loadTabs(l browser.TabLister) tea.Cmd {
	return func() tea.Msg {
		tabs, err := l.ListTabs()
		return tabsLoadedMsg{tabs: tabs, err: err}
	}
}

func (m Tabs) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case actedMsg:
		m.err = msg.err
		m.quitting = true
		return m, tea.Quit

	case tabsLoadedMsg:
		m.loading = false
		m.loadErr = msg.err
		m.tabs = msg.tabs
		m.results = filterTabs(m.tabs, m.input.Value())
		m.clamp()
		return m, nil

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
			leader := NewLeader(m.browser, m.all, m.groups, m.showTags, m.search)
			leader.width, leader.height = m.width, m.height
			return leader, nil
		case tea.KeyEnter:
			if len(m.results) > 0 {
				return m, act(m.browser, m.results[m.cursor].Tab.URL, false)
			}
			return m, nil
		case tea.KeyUp, tea.KeyCtrlP:
			m.move(-1)
			return m, nil
		case tea.KeyDown, tea.KeyCtrlN:
			m.move(1)
			return m, nil
		}

		// Anything else edits the filter query.
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.results = filterTabs(m.tabs, m.input.Value())
		m.clamp()
		return m, cmd
	}
	return m, nil
}

func (m *Tabs) move(delta int) {
	m.cursor += delta
	m.clamp()
}

// visibleCount is how many single-line tab rows fit in the body.
func (m Tabs) visibleCount() int {
	if m.height <= 0 {
		return defaultVisible
	}
	n := m.height - tabsChrome
	if n < 1 {
		n = 1
	}
	return n
}

// clamp keeps the cursor in range and the scroll window around it.
func (m *Tabs) clamp() {
	vis := m.visibleCount()
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

func (m Tabs) View() string {
	if m.quitting {
		return ""
	}
	head := []string{
		header("tabs"),
		"  " + promptStr.Render("⇥ ") + m.input.View(),
		"  " + hintStyle.Render(fmt.Sprintf("%d tab%s", len(m.results), plural(len(m.results)))),
		"",
	}
	foot := []string{"", hintStyle.Render("  ↑↓  move   ·   ↵  focus   ·   esc  back   ·   ^c  quit")}

	body := m.bodyLines()
	return frame(m.width, m.height, head, body, foot)
}

// bodyLines renders the result rows, or the appropriate status message for the
// loading / error / empty states.
func (m Tabs) bodyLines() []string {
	switch {
	case m.loading:
		return []string{"  " + hintStyle.Render("loading tabs…")}
	case m.loadErr != nil:
		// ErrBrowserNotRunning and ErrAutomationDenied both carry user-facing text.
		return []string{"  " + hintStyle.Render(loadErrText(m.loadErr))}
	case len(m.tabs) == 0:
		return []string{"  " + hintStyle.Render("no open tabs")}
	case len(m.results) == 0:
		return []string{"  " + hintStyle.Render("no matches")}
	}

	var body []string
	end := m.offset + m.visibleCount()
	if end > len(m.results) {
		end = len(m.results)
	}
	for i := m.offset; i < end; i++ {
		body = append(body, m.renderRow(m.results[i], i == m.cursor))
	}
	return body
}

// loadErrText turns a listing error into the line shown in the body.
func loadErrText(err error) string {
	if errors.Is(err, browser.ErrAutomationDenied) || errors.Is(err, browser.ErrBrowserNotRunning) {
		return err.Error()
	}
	return "couldn't list tabs: " + err.Error()
}

// renderRow renders one tab as a single line: a bold title followed by its faint
// "(friendly url)". A blank title falls back to the friendly URL alone.
func (m Tabs) renderRow(r TabResult, selected bool) string {
	marker := "  "
	if selected {
		marker = cursorBar.Render("▌ ")
	}
	fu := friendlyURL(r.Tab.URL)
	if strings.TrimSpace(r.Tab.Title) == "" {
		return marker + highlight(fu, r.URLMatch, tabURL, matchStyle)
	}
	title := highlight(r.Tab.Title, r.TitleMatch, tabTitle, matchStyle)
	url := highlight(fu, r.URLMatch, tabURL, matchStyle)
	return marker + title + " " + tabURL.Render("(") + url + tabURL.Render(")")
}

// TabResult is one ranked tab hit. TitleMatch / URLMatch hold the matched rune
// indexes in the tab's title / friendly URL, driving highlighting.
type TabResult struct {
	Tab        browser.Tab
	TitleMatch []int
	URLMatch   []int
	score      int
}

// filterTabs ranks open tabs against a query over title and friendly URL, reusing
// the same matcher and tiers as bookmarks mode. An empty query returns every tab
// in original (browser) order with no highlight.
func filterTabs(tabs []browser.Tab, query string) []TabResult {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		out := make([]TabResult, len(tabs))
		for i, t := range tabs {
			out[i] = TabResult{Tab: t}
		}
		return out
	}
	qr := []rune(q)

	var out []TabResult
	for _, t := range tabs {
		titleR := []rune(strings.ToLower(t.Title))
		titleTier, titleIdx, titleOK := fieldMatch(qr, titleR)

		urlR := []rune(strings.ToLower(friendlyURL(t.URL)))
		urlTier, urlIdx, urlOK := fieldMatch(qr, urlR)

		if !titleOK && !urlOK {
			continue
		}

		// Pick the winning field by tier; on ties, title beats url.
		bestTier, fieldBonus, winPos, winLen := 0, 0, 0, 0
		if titleOK && titleTier > bestTier {
			bestTier, fieldBonus, winPos, winLen = titleTier, bonusName, titleIdx[0], len(titleR)
		}
		if urlOK && urlTier > bestTier {
			bestTier, fieldBonus, winPos, winLen = urlTier, bonusURL, urlIdx[0], len(urlR)
		}

		fine := 900 - winPos*10 - winLen
		if fine < 0 {
			fine = 0
		}
		score := bestTier*10000 + fieldBonus + fine

		var tm []int
		if titleOK {
			tm = titleIdx
		}
		var um []int
		if urlOK {
			um = urlIdx
		}
		out = append(out, TabResult{Tab: t, TitleMatch: tm, URLMatch: um, score: score})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return out[i].Tab.Title < out[j].Tab.Title
	})
	return out
}

// friendlyURL strips the scheme, a leading "www.", and a trailing "/" for display
// and matching. The full URL is kept on the Tab for focusing.
func friendlyURL(u string) string {
	if i := strings.Index(u, "://"); i >= 0 {
		u = u[i+len("://"):]
	}
	u = strings.TrimPrefix(u, "www.")
	u = strings.TrimSuffix(u, "/")
	return u
}
