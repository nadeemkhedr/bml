// Package tui holds bml's interactive Bubble Tea models: leader mode, bookmarks
// mode (the "/" fuzzy finder), and search mode (the "s" web search).
package tui

import (
	"sort"
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
	search        config.Search // resolved engines, handed to search mode
	favorites     []favorite
	byKey         map[string]favorite
	groupName     map[string]string
	prefix        string // characters typed so far at the current depth
	width, height int
	err           error
	quitting      bool
}

// NewLeader builds the leader model from the bookmark list (keyed bookmarks
// become navigable favorites), the optional group labels, whether to show tags,
// and the resolved search-engine config.
func NewLeader(b browser.Browser, bookmarks []config.Bookmark, groups []config.Group, showTags bool, search config.Search) Leader {
	m := Leader{
		browser:   b,
		all:       bookmarks,
		groups:    groups,
		showTags:  showTags,
		search:    search,
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
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEsc:
			if m.prefix != "" {
				m.prefix = "" // back to the top level
				return m, nil
			}
			m.quitting = true
			return m, tea.Quit
		case tea.KeyBackspace:
			if r := []rune(m.prefix); len(r) > 0 {
				m.prefix = string(r[:len(r)-1])
			}
			return m, nil
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
		return m, nil
	}

	if m.prefix == "" {
		switch strings.ToLower(s) {
		case "q":
			m.quitting = true
			return m, tea.Quit
		case "/":
			search := NewSearch(m.browser, m.all, m.groups, m.showTags, m.search)
			search.width, search.height = m.width, m.height // bubbletea won't resend size on a model swap
			search.clamp()
			return search, search.Init()
		case "s":
			web := NewWebSearch(m.browser, m.all, m.groups, m.showTags, m.search)
			web.width, web.height = m.width, m.height // bubbletea won't resend size on a model swap
			return web, web.Init()
		}
	}
	return m, nil // stray key inside a group — ignore
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

	body := m.treeLines(m.prefix, 0)
	if len(body) == 0 {
		body = []string{hintStyle.Render("  no favorites yet — add a key to a bookmark in `bml edit`")}
	}
	return frame(m.width, m.height, head, body, foot)
}

// treeLines renders the menu rooted at prefix as a slice of lines, expanding
// groups inline with their children indented one level deeper.
func (m Leader) treeLines(prefix string, depth int) []string {
	indent := strings.Repeat("  ", depth*2+1)
	var lines []string
	for _, c := range m.childrenOf(prefix) {
		if c.leaf {
			row := indent + keyBadge.Render(c.ch) + "  " + nameStyle.Render(c.name)
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
		lines = append(lines, m.treeLines(prefix+c.ch, depth+1)...)
	}
	return lines
}

func (m Leader) footer() string {
	if m.prefix == "" {
		return "  Shift+key  new tab   ·   /  bookmarks   ·   s  search   ·   q  quit"
	}
	return "  Shift+key  new tab   ·   ⌫  back   ·   esc  top"
}

// errer is implemented by both leader and search models so the runner can
// surface an act error regardless of which mode the program ended in.
type errer interface{ Err() error }

// RunLeader runs the interactive program (starting in leader mode) and returns
// any error from acting on a bookmark.
func RunLeader(b browser.Browser, bookmarks []config.Bookmark, groups []config.Group, showTags bool, search config.Search) error {
	final, err := tea.NewProgram(NewLeader(b, bookmarks, groups, showTags, search), tea.WithAltScreen()).Run()
	if err != nil {
		return err
	}
	if e, ok := final.(errer); ok {
		return e.Err()
	}
	return nil
}
