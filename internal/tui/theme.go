package tui

import "github.com/charmbracelet/lipgloss"

// Palette — adaptive so it reads well on both dark and light terminals.
var (
	accent = lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A78BFA"} // violet
	match  = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"} // amber
	subtle = lipgloss.AdaptiveColor{Light: "#9CA3AF", Dark: "#6B7280"} // grey
	fg     = lipgloss.AdaptiveColor{Light: "#1F2937", Dark: "#E5E7EB"}
	onAcc  = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#1E1B2E"}
)

var (
	// Header: "▌ bml" with an accent bar.
	logoBar = lipgloss.NewStyle().Foreground(accent).Bold(true)
	logo    = lipgloss.NewStyle().Foreground(fg).Bold(true)

	// A key shown as a filled badge.
	keyBadge = lipgloss.NewStyle().
			Foreground(onAcc).Background(accent).Bold(true).
			Padding(0, 1)

	nameStyle  = lipgloss.NewStyle().Foreground(fg)
	matchStyle = lipgloss.NewStyle().Foreground(match).Bold(true)
	urlStyle   = lipgloss.NewStyle().Foreground(subtle)
	hintStyle  = lipgloss.NewStyle().Foreground(subtle)

	// A tag pill.
	pillStyle = lipgloss.NewStyle().
			Foreground(accent).
			Background(lipgloss.AdaptiveColor{Light: "#EDE9FE", Dark: "#2A2540"}).
			Padding(0, 1)

	// Group arrow marker in the leader menu.
	groupArrow = lipgloss.NewStyle().Foreground(accent).Bold(true)

	// Selected row marker and prompt.
	cursorBar = lipgloss.NewStyle().Foreground(accent).Bold(true)
	selName   = lipgloss.NewStyle().Foreground(fg).Bold(true)
	promptStr = lipgloss.NewStyle().Foreground(accent).Bold(true)
)

// header renders the "▌ bml" logo line with an optional subtitle.
func header(subtitle string) string {
	line := logoBar.Render("▌ ") + logo.Render("bml")
	if subtitle != "" {
		line += "  " + hintStyle.Render(subtitle)
	}
	return line
}

// renderTags renders tags as pills.
func renderTags(tags []string) string {
	out := ""
	for _, t := range tags {
		out += " " + pillStyle.Render(t)
	}
	return out
}

// highlight renders text, emphasizing the rune indexes in idx.
func highlight(text string, idx []int, base, hi lipgloss.Style) string {
	if len(idx) == 0 {
		return base.Render(text)
	}
	set := make(map[int]bool, len(idx))
	for _, i := range idx {
		set[i] = true
	}
	var out string
	for i, r := range []rune(text) {
		if set[i] {
			out += hi.Render(string(r))
		} else {
			out += base.Render(string(r))
		}
	}
	return out
}
