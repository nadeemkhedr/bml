package importer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bml/internal/config"
)

// Source is a known Chromium-based browser and where it stores bookmarks on
// macOS. Adding a browser is a one-line registry entry.
type Source struct {
	Name    string   // canonical name, e.g. "brave"
	Aliases []string // alternative names accepted on the CLI
	App     string   // display name, e.g. "Brave Browser"
	dir     []string // path segments under ~/Library/Application Support
}

var sources = []Source{
	{Name: "brave", App: "Brave Browser", dir: []string{"BraveSoftware", "Brave-Browser"}},
	{Name: "chrome", Aliases: []string{"chromium", "google-chrome"}, App: "Google Chrome", dir: []string{"Google", "Chrome"}},
	{Name: "edge", App: "Microsoft Edge", dir: []string{"Microsoft Edge"}},
}

// Lookup finds a Source by canonical name or alias (case-insensitive).
func Lookup(name string) (Source, bool) {
	n := strings.ToLower(strings.TrimSpace(name))
	for _, s := range sources {
		if s.Name == n {
			return s, true
		}
		for _, a := range s.Aliases {
			if a == n {
				return s, true
			}
		}
	}
	return Source{}, false
}

// SupportedNames lists the canonical browser names for help/error messages.
func SupportedNames() []string {
	out := make([]string, len(sources))
	for i, s := range sources {
		out[i] = s.Name
	}
	return out
}

// BookmarksPath returns the bookmarks file for a profile (e.g. "Default").
func (s Source) BookmarksPath(home, profile string) string {
	parts := append([]string{home, "Library", "Application Support"}, s.dir...)
	parts = append(parts, profile, "Bookmarks")
	return filepath.Join(parts...)
}

// Read resolves, reads, and parses a browser's bookmarks for the given profile.
func Read(home, profile string, s Source) ([]config.Bookmark, error) {
	path := s.BookmarksPath(home, profile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no bookmarks found for %s at %s (is it installed, and is the profile %q correct?)", s.App, path, profile)
		}
		return nil, fmt.Errorf("reading %s bookmarks: %w", s.App, err)
	}
	return Parse(data)
}
