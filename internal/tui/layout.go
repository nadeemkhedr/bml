package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// frame composes a fixed header, a body, and a fixed footer into exactly height
// rows and width columns. The body is clamped to the space left between header
// and footer (with a "… more" marker if it overflows) and padded with blank
// lines so the header stays pinned to the top and the footer to the bottom.
//
// If height <= 0 (size not known yet) it falls back to a natural join.
func frame(width, height int, head, body, foot []string) string {
	// Before the first WindowSizeMsg the size is unknown. Render only the header
	// (a couple of short lines) rather than dumping full-height content that
	// would overflow and scroll the header off the top of the alternate screen.
	if height <= 0 || width <= 0 {
		return strings.Join(head, "\n")
	}

	head = truncateAll(head, width)
	body = truncateAll(body, width)
	foot = truncateAll(foot, width)

	avail := height - len(head) - len(foot)
	if avail < 1 {
		avail = 1
	}

	shown := make([]string, avail)
	if len(body) > avail {
		copy(shown, body[:avail])
		shown[avail-1] = truncate(hintStyle.Render("  … more"), width)
	} else {
		copy(shown, body) // remaining entries stay "" (blank padding)
	}

	return strings.Join(concat(head, shown, foot), "\n")
}

func concat(parts ...[]string) []string {
	var out []string
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func truncateAll(lines []string, width int) []string {
	if width <= 0 {
		return lines
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = truncate(l, width)
	}
	return out
}

// truncate limits a (possibly styled) line to width display columns.
func truncate(s string, width int) string {
	if width <= 0 {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(s)
}
