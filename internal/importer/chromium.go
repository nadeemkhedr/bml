// Package importer reads bookmarks from Chromium-based browsers (Brave, Chrome,
// Edge) and converts them into bml bookmarks. The on-disk "Bookmarks" JSON
// format is shared across Chromium browsers, so a single parser serves them all;
// only the file location differs per browser (see sources.go).
package importer

import (
	"encoding/json"
	"fmt"
	"strings"

	"bml/internal/config"
)

// chromeNode is one node in the Chromium bookmarks tree.
type chromeNode struct {
	Type     string       `json:"type"` // "url" or "folder"
	Name     string       `json:"name"`
	URL      string       `json:"url"`
	Children []chromeNode `json:"children"`
}

type chromeFile struct {
	Roots map[string]chromeNode `json:"roots"`
}

// rootOrder fixes a deterministic traversal across the standard roots.
var rootOrder = []string{"bookmark_bar", "other", "synced"}

// Parse converts a Chromium "Bookmarks" JSON document into bml bookmarks. Folder
// names along the path become tags; entries are deduplicated by URL (first wins)
// and non-web schemes are skipped. Keys are never assigned — that's the user's
// to do.
func Parse(data []byte) ([]config.Bookmark, error) {
	var f chromeFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing chromium bookmarks: %w", err)
	}

	var out []config.Bookmark
	seen := make(map[string]bool)

	var walk func(n chromeNode, trail []string)
	walk = func(n chromeNode, trail []string) {
		switch n.Type {
		case "url":
			if skipURL(n.URL) || seen[n.URL] {
				return
			}
			seen[n.URL] = true
			name := n.Name
			if name == "" {
				name = host(n.URL)
			}
			out = append(out, config.Bookmark{Name: name, URL: n.URL, Tags: tagsFrom(trail)})
		case "folder":
			t := trail
			if n.Name != "" {
				t = append(append([]string{}, trail...), n.Name)
			}
			for _, c := range n.Children {
				walk(c, t)
			}
		}
	}

	for _, key := range rootOrder {
		if root, ok := f.Roots[key]; ok {
			for _, c := range root.Children {
				walk(c, nil)
			}
		}
	}
	return out, nil
}

// tagsFrom turns a folder trail into tags: trimmed, empties dropped, deduped
// case-insensitively while preserving order and original casing.
func tagsFrom(trail []string) []string {
	var tags []string
	seen := make(map[string]bool)
	for _, f := range trail {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		k := strings.ToLower(f)
		if seen[k] {
			continue
		}
		seen[k] = true
		tags = append(tags, f)
	}
	return tags
}

// skipURL reports whether a URL is not a normal web bookmark worth importing.
func skipURL(u string) bool {
	if u == "" {
		return true
	}
	for _, p := range []string{"javascript:", "about:", "chrome://", "brave://", "edge://", "chrome-extension://", "file://"} {
		if strings.HasPrefix(u, p) {
			return true
		}
	}
	return false
}

// host extracts the host portion of a URL for use as a fallback name.
func host(u string) string {
	if i := strings.Index(u, "://"); i >= 0 {
		u = u[i+3:]
	}
	if i := strings.IndexByte(u, '/'); i >= 0 {
		u = u[:i]
	}
	return u
}
