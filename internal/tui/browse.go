package tui

import (
	"strings"

	"bml/internal/browser"
	"bml/internal/config"
	"bml/internal/history"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// maxSuggestions caps the autocomplete list (bookmark + popular-domain rows).
const maxSuggestions = 8

// browseChrome is the number of non-suggestion rows in the view: logo, prompt,
// blank, blank, footer.
const browseChrome = 5

// suggestion is one autocomplete row: a URL to act on, with the leading portion
// of the displayed text that matched the query (for highlighting).
type suggestion struct {
	url        string   // the URL to focus-or-open
	display    string   // normalized url (bookmark) or domain (popular)
	name       string   // bookmark name; "" for a popular domain
	tags       []string // bookmark tags
	matchLen   int      // leading runes of display that matched the query
	isBookmark bool
}

// Browse is browse mode: entered from leader mode with ".", it takes a typed
// address or search term. Enter opens a URL directly (or searches the primary
// engine when the input isn't a URL); an autocomplete list offers matching
// bookmark URLs (prefix match) first, then entries from the browser's own
// history. It is fire-and-exit like the other acting modes.
//
// The cursor selects a row: row 0 is the primary action (act on exactly what
// was typed); rows 1.. are the suggestions. Editing the query resets the cursor
// to the primary action, mirroring a browser's address bar.
type Browse struct {
	browser browser.Browser
	// Carried so returning to leader (Esc) rebuilds it identically.
	all      []config.Bookmark
	groups   []config.Group
	showTags bool
	search   config.Search
	history  *history.History

	visits        []browser.Visit // browser history, loaded asynchronously on entry
	input         textinput.Model
	suggestions   []suggestion
	cursor        int // 0 = primary action; 1.. = suggestions[cursor-1]
	offset        int
	width, height int
	err           error
	quitting      bool
}

// historyLoadedMsg delivers the browser history read asynchronously on entry. An
// empty/failed read leaves browse mode with bookmark-only suggestions.
type historyLoadedMsg struct{ visits []browser.Visit }

// loadHistory reads the browser's visit history off the main loop, if the
// backend can. It never surfaces an error — a failed read just yields no history
// suggestions — so reading the user's history can't break opening a URL.
func loadHistory(b browser.Browser) tea.Cmd {
	lister, ok := b.(browser.HistoryLister)
	if !ok {
		return nil
	}
	return func() tea.Msg {
		visits, _ := lister.ListHistory()
		return historyLoadedMsg{visits: visits}
	}
}

// NewBrowse builds the browse-mode model. It carries the same state as the other
// modes so Esc can rebuild leader mode unchanged.
func NewBrowse(b browser.Browser, bookmarks []config.Bookmark, groups []config.Group, showTags bool, search config.Search, hist *history.History) Browse {
	in := textinput.New()
	in.Placeholder = "enter a URL or search…"
	in.Prompt = ""
	in.Focus()

	return Browse{
		browser:  b,
		all:      bookmarks,
		groups:   groups,
		showTags: showTags,
		search:   search,
		history:  hist,
		input:    in,
	}
}

// Err returns any error from acting on a URL.
func (m Browse) Err() error { return m.err }

// Init starts the cursor blink and kicks off the asynchronous history read, so
// mode entry is instant and suggestions populate a few milliseconds later.
func (m Browse) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, loadHistory(m.browser))
}

func (m Browse) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case actedMsg:
		m.err = msg.err
		m.quitting = true
		return m, tea.Quit

	case historyLoadedMsg:
		m.visits = msg.visits
		// Rebuild for the current query now that history is available, keeping
		// the cursor where it is (the read finishes before any real interaction).
		m.suggestions = buildSuggestions(m.all, m.visits, m.input.Value())
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
			leader := NewLeader(m.browser, m.all, m.groups, m.showTags, m.search, m.history)
			leader.width, leader.height = m.width, m.height
			return leader, nil
		case tea.KeyEnter:
			return m.act()
		case tea.KeyTab:
			m.complete()
			return m, nil
		case tea.KeyUp, tea.KeyCtrlP:
			m.move(-1)
			return m, nil
		case tea.KeyDown, tea.KeyCtrlN:
			m.move(1)
			return m, nil
		}

		// Anything else edits the query and rebuilds the suggestions.
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.refresh()
		return m, cmd
	}
	return m, nil
}

// act resolves the selected row. The primary action (row 0) opens the raw input
// as a URL or, when it isn't one, searches the primary engine; a suggestion row
// focus-or-opens its URL. A blank primary action is a no-op.
func (m Browse) act() (tea.Model, tea.Cmd) {
	if m.cursor > 0 {
		s := m.suggestions[m.cursor-1]
		return m, act(m.browser, s.url, false)
	}
	q := strings.TrimSpace(m.input.Value())
	if q == "" {
		return m, nil
	}
	if looksLikeURL(q) {
		return m, act(m.browser, toURL(q), false)
	}
	return m, act(m.browser, m.search.Primary.URL(q), true)
}

// complete fills the input from the selected suggestion (or the top one when the
// primary action is selected), the way Tab completes in a browser's address bar.
func (m *Browse) complete() {
	idx := m.cursor
	if idx == 0 {
		idx = 1 // complete to the best suggestion
	}
	if idx-1 < 0 || idx-1 >= len(m.suggestions) {
		return
	}
	m.input.SetValue(m.suggestions[idx-1].display)
	m.input.CursorEnd()
	m.refresh()
}

// refresh rebuilds the suggestion list for the current query and returns the
// cursor to the primary action (the freshly typed input).
func (m *Browse) refresh() {
	m.suggestions = buildSuggestions(m.all, m.visits, m.input.Value())
	m.cursor = 0
	m.offset = 0
}

// buildSuggestions returns the autocomplete rows for a query: bookmark URLs
// whose normalized form starts with the query (highest priority), then browser-
// history URLs by the same prefix (already ranked most-typed/visited first),
// skipping any already shown. The whole list is capped at maxSuggestions.
func buildSuggestions(bms []config.Bookmark, visits []browser.Visit, query string) []suggestion {
	q := normalizeHost(query)
	if q == "" {
		return nil
	}
	qlen := len([]rune(q))

	var out []suggestion
	seen := make(map[string]bool)

	// add appends a prefix-matching, not-yet-seen candidate and reports whether
	// the list is now full.
	add := func(rawURL, name string, tags []string, isBookmark bool) bool {
		nu := normalizeHost(rawURL)
		if !strings.HasPrefix(nu, q) || seen[nu] {
			return false
		}
		seen[nu] = true
		out = append(out, suggestion{
			url: rawURL, display: nu, name: name, tags: tags,
			matchLen: qlen, isBookmark: isBookmark,
		})
		return len(out) >= maxSuggestions
	}

	for _, b := range bms {
		if add(b.URL, b.Name, b.Tags, true) {
			return out
		}
	}
	for _, v := range visits {
		if add(v.URL, v.Title, nil, false) {
			break
		}
	}
	return out
}

// normalizeHost reduces an address to the bare host+path used for prefix
// matching: lowercased, with any scheme and a leading "www." stripped.
func normalizeHost(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	return strings.TrimPrefix(s, "www.")
}

// looksLikeURL reports whether the input should be navigated to as an address
// rather than sent to a search engine: no spaces, and either an explicit scheme,
// a dot (a host), or the localhost special case.
func looksLikeURL(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s, " \t") {
		return false
	}
	if strings.Contains(s, "://") {
		return true
	}
	return strings.Contains(s, ".") || s == "localhost" || strings.HasPrefix(s, "localhost:") || strings.HasPrefix(s, "localhost/")
}

// toURL ensures an address has a scheme, defaulting to https.
func toURL(s string) string {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "://") {
		return s
	}
	return "https://" + s
}

func (m *Browse) move(delta int) {
	m.cursor += delta
	m.clamp()
}

// rowCount is the primary action plus every suggestion.
func (m Browse) rowCount() int { return 1 + len(m.suggestions) }

// visibleCount is how many rows fit between header and footer (one row each).
func (m Browse) visibleCount() int {
	if m.height <= 0 {
		return defaultVisible
	}
	n := m.height - browseChrome
	if n < 1 {
		n = 1
	}
	return n
}

// clamp keeps the cursor in range and the scroll window around it.
func (m *Browse) clamp() {
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor > m.rowCount()-1 {
		m.cursor = m.rowCount() - 1
	}
	vis := m.visibleCount()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+vis {
		m.offset = m.cursor - vis + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m Browse) View() string {
	if m.quitting {
		return ""
	}
	head := []string{
		header("browse"),
		"  " + promptStr.Render(". ") + m.input.View(),
		"",
	}
	foot := []string{"", hintStyle.Render(m.footer())}

	rows := make([]string, 0, m.rowCount())
	rows = append(rows, m.renderPrimary(m.cursor == 0))
	for i, s := range m.suggestions {
		rows = append(rows, m.renderSuggestion(s, m.cursor == i+1))
	}

	if vis := m.visibleCount(); len(rows) > vis {
		end := m.offset + vis
		if end > len(rows) {
			end = len(rows)
		}
		rows = rows[m.offset:end]
	}
	return frame(m.width, m.height, head, rows, foot)
}

// renderPrimary renders row 0: a description of what Enter does with exactly
// what was typed — open an address, or search for a term.
func (m Browse) renderPrimary(selected bool) string {
	marker := "  "
	if selected {
		marker = cursorBar.Render("▌ ")
	}
	q := strings.TrimSpace(m.input.Value())
	if q == "" {
		return marker + hintStyle.Render("type a URL or a search term")
	}
	if looksLikeURL(q) {
		return marker + nameStyle.Render("open  ") + urlStyle.Render(toURL(q))
	}
	return marker + nameStyle.Render("search "+m.search.Primary.Name+" for  ") + selName.Render(q)
}

// labelWidth caps a suggestion's title/name so a long page title can't push the
// (prefix-highlighted) URL off the right edge.
const labelWidth = 48

// renderSuggestion renders one autocomplete row: the label (bookmark name or
// page title) followed by the dimmed, prefix-highlighted URL. A history entry
// with no title shows the URL alone.
func (m Browse) renderSuggestion(s suggestion, selected bool) string {
	marker := "  "
	if selected {
		marker = cursorBar.Render("▌ ")
	}
	idx := prefixRange(s.matchLen)

	if s.name == "" {
		base := urlStyle
		if selected {
			base = selName
		}
		return marker + highlight(s.display, idx, base, matchStyle)
	}

	label := nameStyle
	if selected {
		label = selName
	}
	row := marker + label.Render(clip(s.name, labelWidth)) + "  " + highlight(s.display, idx, urlStyle, matchStyle)
	if s.isBookmark && m.showTags {
		row += renderTags(s.tags)
	}
	return row
}

// clip shortens s to at most max runes, marking truncation with an ellipsis.
func clip(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

func (m Browse) footer() string {
	return "  ↑↓  move   ·   ↵  open/search   ·   ⇥  complete   ·   esc  back   ·   ^c  quit"
}

// prefixRange returns the rune indexes 0..n-1, for highlighting a matched prefix.
func prefixRange(n int) []int {
	if n <= 0 {
		return nil
	}
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	return idx
}
