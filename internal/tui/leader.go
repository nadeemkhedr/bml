// Package tui holds bml's interactive Bubble Tea models: leader mode, bookmarks
// mode (the "/" fuzzy finder), search mode (the "s" web search), and tab mode
// (the Tab-key switcher over the browser's open tabs).
package tui

import (
	"sort"
	"strings"
	"unicode"

	"bml/internal/browser"
	"bml/internal/config"
	"bml/internal/history"

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
	key, name, url string // key is the full lowercased sequence, e.g. "wt"
	tags           []string
}

// Leader is the which-key launcher shown when bml starts with no argument. Keys
// are 1–3 character sequences; pressing characters navigates through groups
// until a bookmark is reached.
type Leader struct {
	browser       browser.Browser
	all           []config.Bookmark // full list, handed to bookmarks/search mode
	groups        []config.Group
	showTags      bool
	search        config.Search    // resolved engines, handed to search mode
	history       *history.History // learned ranking, handed to bookmarks mode
	favorites     []favorite
	byKey         map[string]favorite
	groupName     map[string]string
	prefix        string // characters typed so far at the current depth
	cursor        int    // index into the reachable leaves, when browsing
	offset        int    // first rendered line shown (scroll window)
	cursorActive  bool   // a leaf is selected (latent until the first arrow press)
	width, height int
	err           error
	quitting      bool
}

// NewLeader builds the leader model from the bookmark list (keyed bookmarks
// become navigable favorites), the optional group labels, whether to show tags,
// and the resolved search-engine config.
func NewLeader(b browser.Browser, bookmarks []config.Bookmark, groups []config.Group, showTags bool, search config.Search, hist *history.History) Leader {
	m := Leader{
		browser:   b,
		all:       bookmarks,
		groups:    groups,
		showTags:  showTags,
		search:    search,
		history:   hist,
		byKey:     make(map[string]favorite),
		groupName: make(map[string]string),
	}
	for _, bm := range bookmarks {
		if bm.Key == "" {
			continue
		}
		f := favorite{key: strings.ToLower(bm.Key), name: bm.Name, url: bm.URL, tags: bm.Tags}
		m.favorites = append(m.favorites, f)
		m.byKey[f.key] = f
	}
	for _, g := range groups {
		m.groupName[strings.ToLower(g.Key)] = g.Name
	}
	return m
}

// Err returns any error from acting on a bookmark.
func (m Leader) Err() error { return m.err }

func (m Leader) Init() tea.Cmd { return nil }

func (m Leader) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if m.prefix != "" {
				m.prefix = "" // back to the top level
				m.resetCursor()
				return m, nil
			}
			m.quitting = true
			return m, tea.Quit
		case tea.KeyBackspace:
			if r := []rune(m.prefix); len(r) > 0 {
				m.prefix = string(r[:len(r)-1])
				m.resetCursor()
			}
			return m, nil
		case tea.KeyUp, tea.KeyCtrlP:
			m.moveCursor(-1)
			return m, nil
		case tea.KeyDown, tea.KeyCtrlN:
			m.moveCursor(1)
			return m, nil
		case tea.KeyEnter:
			// Acts only once an arrow has revealed a selection; latent Enter is
			// a no-op so a stray press on the opening screen does nothing.
			if f, ok := m.byKey[m.selectedKey()]; ok {
				return m, act(m.browser, f.url, false)
			}
			return m, nil
		case tea.KeyTab:
			return m.enterTabs()
		case tea.KeyRunes:
			return m.handleRune(string(msg.Runes))
		}
	}
	return m, nil
}

// handleRune advances navigation by one character. A completed key sequence acts
// (uppercase last char forces a new tab); a group prefix descends; an unmatched
// character only triggers quit/search at the top level.
func (m Leader) handleRune(s string) (tea.Model, tea.Cmd) {
	if len([]rune(s)) != 1 {
		return m, nil
	}
	next := m.prefix + strings.ToLower(s)

	if f, ok := m.byKey[next]; ok {
		// Prefix-free config guarantees this is the unique completion.
		return m, act(m.browser, f.url, isUpper(s))
	}
	if m.hasPrefix(next) {
		m.prefix = next // descend into the group
		m.resetCursor() // a fresh view starts with no selection
		return m, nil
	}

	if m.prefix == "" {
		switch strings.ToLower(s) {
		case "q":
			m.quitting = true
			return m, tea.Quit
		case "/":
			search := NewSearch(m.browser, m.all, m.groups, m.showTags, m.search, m.history)
			search.width, search.height = m.width, m.height // bubbletea won't resend size on a model swap
			search.clamp()
			return search, search.Init()
		case "s":
			web := NewWebSearch(m.browser, m.all, m.groups, m.showTags, m.search, m.history)
			web.width, web.height = m.width, m.height // bubbletea won't resend size on a model swap
			return web, web.Init()
		}
	}
	return m, nil // stray key inside a group — ignore
}

// enterTabs switches to tab mode. It only fires at the top level (the Tab key is
// ignored while navigating a group), and only if the backend can enumerate tabs —
// otherwise it is a silent no-op.
func (m Leader) enterTabs() (tea.Model, tea.Cmd) {
	if m.prefix != "" {
		return m, nil
	}
	lister, ok := m.browser.(browser.TabLister)
	if !ok {
		return m, nil
	}
	tabs := NewTabs(m.browser, lister, m.all, m.groups, m.showTags, m.search, m.history)
	tabs.width, tabs.height = m.width, m.height // bubbletea won't resend size on a model swap
	tabs.clamp()
	return tabs, tabs.Init()
}

// resetCursor drops back to no selection. Called whenever the view re-roots
// (descending via a typed key, or backing out with Esc/Backspace), since the
// reachable leaves change and a carried-over index would be meaningless.
func (m *Leader) resetCursor() {
	m.cursorActive = false
	m.cursor = 0
	m.offset = 0
}

// moveCursor browses the reachable leaves. The first press reveals the latent
// selection (Down lands on the first leaf, Up on the last); later presses step
// and clamp. Typed keys remain the authoritative path — this only adds an
// alternative way to reach a favorite without knowing its key.
func (m *Leader) moveCursor(delta int) {
	leaves := m.leaves()
	if len(leaves) == 0 {
		return
	}
	if !m.cursorActive {
		m.cursorActive = true
		if delta < 0 {
			m.cursor = len(leaves) - 1
		} else {
			m.cursor = 0
		}
	} else {
		m.cursor += delta
	}
	m.clamp()
}

// leaves returns the full keys of every reachable bookmark under the current
// prefix, in the same depth-first order renderBody draws them. The browse
// cursor is an index into this slice; group headers are skipped.
func (m Leader) leaves() []string {
	var out []string
	var walk func(prefix string)
	walk = func(prefix string) {
		for _, c := range m.childrenOf(prefix) {
			full := prefix + c.ch
			if c.leaf {
				out = append(out, full)
			} else {
				walk(full)
			}
		}
	}
	walk(m.prefix)
	return out
}

// selectedKey is the full key of the leaf under the cursor, or "" when the
// selection is latent or out of range (so Enter is a no-op).
func (m Leader) selectedKey() string {
	if !m.cursorActive {
		return ""
	}
	leaves := m.leaves()
	if m.cursor < 0 || m.cursor >= len(leaves) {
		return ""
	}
	return leaves[m.cursor]
}

// leaderChrome is the number of non-body rows (header + blank, then blank +
// footer) — matching frame's reserved space so the scroll window fills the rest.
const leaderChrome = 4

// visibleCount is how many rendered body lines fit between header and footer.
func (m Leader) visibleCount() int {
	if m.height <= 0 {
		return defaultVisible
	}
	n := m.height - leaderChrome
	if n < 1 {
		n = 1
	}
	return n
}

// clamp keeps the cursor in range and scrolls the window so the selected leaf's
// rendered line stays visible. With no selection the window rests at the top.
func (m *Leader) clamp() {
	leaves := m.leaves()
	if len(leaves) == 0 {
		m.resetCursor()
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor > len(leaves)-1 {
		m.cursor = len(leaves) - 1
	}

	lines, selLine := m.renderBody()
	vis := m.visibleCount()
	if selLine >= 0 {
		if selLine < m.offset {
			m.offset = selLine
		}
		if selLine >= m.offset+vis {
			m.offset = selLine - vis + 1
		}
	}
	if maxOff := len(lines) - vis; m.offset > maxOff {
		m.offset = maxOff
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

// hasPrefix reports whether any key starts with p (and is longer than it).
func (m Leader) hasPrefix(p string) bool {
	for _, f := range m.favorites {
		if len(f.key) > len(p) && strings.HasPrefix(f.key, p) {
			return true
		}
	}
	return false
}

func isUpper(s string) bool {
	r := []rune(s)
	return len(r) == 1 && unicode.IsUpper(r[0])
}

// child is one entry in the menu at the current depth: either a bookmark (leaf)
// or a group to descend into.
type child struct {
	ch   string
	leaf bool
	name string
	tags []string
}

// children returns the menu items reachable by one more keystroke from the
// current prefix.
func (m Leader) children() []child { return m.childrenOf(m.prefix) }

// childrenOf returns the menu items reachable by one more keystroke from prefix.
func (m Leader) childrenOf(prefix string) []child {
	plen := len([]rune(prefix))
	seen := make(map[string]bool)
	var out []child
	for _, f := range m.favorites {
		kr := []rune(f.key)
		if !strings.HasPrefix(f.key, prefix) || len(kr) <= plen {
			continue
		}
		ch := string(kr[plen])
		if seen[ch] {
			continue
		}
		seen[ch] = true
		full := prefix + ch
		if bm, ok := m.byKey[full]; ok {
			out = append(out, child{ch: ch, leaf: true, name: bm.name, tags: bm.tags})
		} else {
			out = append(out, child{ch: ch, name: m.groupName[full]})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ch < out[j].ch })
	return out
}

// breadcrumb describes the current group path for the header.
func (m Leader) breadcrumb() string {
	if m.prefix == "" {
		return "launcher"
	}
	var parts []string
	r := []rune(m.prefix)
	for i := 1; i <= len(r); i++ {
		p := string(r[:i])
		if name, ok := m.groupName[p]; ok {
			parts = append(parts, name)
		} else {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, " / ")
}

func (m Leader) View() string {
	if m.quitting {
		return ""
	}
	head := []string{header(m.breadcrumb()), ""}
	foot := []string{"", hintStyle.Render(m.footer())}

	lines, _ := m.renderBody()
	if len(lines) == 0 {
		body := []string{hintStyle.Render("  no favorites yet — add a key to a bookmark in `bml edit`")}
		return frame(m.width, m.height, head, body, foot)
	}
	return frame(m.width, m.height, head, m.scrollWindow(lines), foot)
}

// scrollWindow slices the rendered lines to the visible window starting at the
// scroll offset. When everything fits it returns the lines unchanged.
func (m Leader) scrollWindow(lines []string) []string {
	vis := m.visibleCount()
	if len(lines) <= vis {
		return lines
	}
	end := m.offset + vis
	if end > len(lines) {
		end = len(lines)
	}
	return lines[m.offset:end]
}

// renderBody renders the menu rooted at the current prefix, expanding groups
// inline with their children indented one level deeper. It marks the selected
// leaf with the cursor bar and reports that leaf's rendered line index (-1 when
// the selection is latent), which the scroll window keeps in view.
func (m Leader) renderBody() (lines []string, selLine int) {
	sel := m.selectedKey()
	selLine = -1
	var walk func(prefix string, depth int)
	walk = func(prefix string, depth int) {
		indent := strings.Repeat("  ", depth*2+1)
		for _, c := range m.childrenOf(prefix) {
			full := prefix + c.ch
			if c.leaf {
				marker, name := indent, nameStyle
				if full == sel {
					// Swap the two trailing indent spaces for the cursor bar so
					// columns stay aligned with the unselected rows.
					marker = indent[:len(indent)-2] + cursorBar.Render("▌ ")
					name = selName
					selLine = len(lines)
				}
				row := marker + keyBadge.Render(c.ch) + "  " + name.Render(c.name)
				if m.showTags {
					row += renderTags(c.tags)
				}
				lines = append(lines, row)
				continue
			}
			label := c.name
			if label == "" {
				label = "group"
			}
			lines = append(lines, indent+keyBadge.Render(c.ch)+"  "+groupHeader.Render("["+label+"]"))
			walk(full, depth+1)
		}
	}
	walk(m.prefix, 0)
	return lines, selLine
}

func (m Leader) footer() string {
	if m.prefix == "" {
		return "  ↑↓  browse   ·   ↵  open   ·   Shift+key  new tab   ·   /  bookmarks   ·   s  search   ·   ⇥  tabs   ·   q  quit"
	}
	return "  ↑↓  browse   ·   ↵  open   ·   Shift+key  new tab   ·   ⌫  back   ·   esc  top"
}

// errer is implemented by both leader and search models so the runner can
// surface an act error regardless of which mode the program ended in.
type errer interface{ Err() error }

// RunLeader runs the interactive program (starting in leader mode) and returns
// any error from acting on a bookmark.
func RunLeader(b browser.Browser, bookmarks []config.Bookmark, groups []config.Group, showTags bool, search config.Search, hist *history.History) error {
	final, err := tea.NewProgram(NewLeader(b, bookmarks, groups, showTags, search, hist), tea.WithAltScreen()).Run()
	if err != nil {
		return err
	}
	if e, ok := final.(errer); ok {
		return e.Err()
	}
	return nil
}
