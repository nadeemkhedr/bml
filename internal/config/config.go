// Package config loads and validates bml's hand-edited files and resolves CLI
// arguments to a URL.
//
// Configuration lives in a directory holding two files: bookmarks.toml (the
// [[bookmark]] entries) and config.toml (settings — browser, leader_tags,
// [search], and [[group]] labels). Keeping them apart means `bml import` only
// ever rewrites bookmarks.toml and can never clobber hand-curated settings (see
// docs/adr/0004).
//
// A Bookmark is the single core entity (see CONTEXT.md). A "favorite" is just a
// bookmark carrying a Key.
package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/BurntSushi/toml"
)

// The two files that make up a config directory.
const (
	bookmarksFile = "bookmarks.toml"
	settingsFile  = "config.toml"
)

// Bookmark is one stored URL entry. Key is a 1–3 character leader sequence
// (e.g. "g" or "wt"); a multi-character key is reached by pressing each
// character in turn, navigating through groups.
type Bookmark struct {
	Name string   `toml:"name"`
	URL  string   `toml:"url"`
	Key  string   `toml:"key,omitempty"`
	Tags []string `toml:"tags,omitempty"`
}

// Group gives an optional friendly name to a key prefix (e.g. "w" → "Work"),
// shown in the leader menu while navigating.
type Group struct {
	Key  string `toml:"key"`
	Name string `toml:"name"`
}

// Config is the loaded, validated bookmark collection plus settings.
type Config struct {
	// Browser is the macOS application name to drive (empty = backend default).
	Browser string
	// LeaderTags controls whether tags are shown in leader mode (default true).
	LeaderTags bool
	// Search is the resolved search-mode configuration (primary/secondary engines).
	Search    Search
	Bookmarks []Bookmark
	Groups    []Group
	byKey     map[string]Bookmark
	groupName map[string]string
}

// tomlSettings mirrors config.toml: top-level settings, an optional [search]
// table, and optional [[group]] labels.
type tomlSettings struct {
	Browser    string      `toml:"browser,omitempty"`
	LeaderTags *bool       `toml:"leader_tags,omitempty"` // nil = default (show)
	Search     *tomlSearch `toml:"search,omitempty"`
	Group      []Group     `toml:"group,omitempty"`
}

// tomlBookmarks mirrors bookmarks.toml: just the array of [[bookmark]] tables.
type tomlBookmarks struct {
	Bookmark []Bookmark `toml:"bookmark"`
}

// BookmarksPath returns the bookmarks file inside a config directory.
func BookmarksPath(dir string) string { return filepath.Join(dir, bookmarksFile) }

// SettingsPath returns the settings file inside a config directory.
func SettingsPath(dir string) string { return filepath.Join(dir, settingsFile) }

// Load reads, parses, and validates the config directory: settings from
// config.toml (optional) and entries from bookmarks.toml. A missing bookmarks
// file returns an error wrapping os.ErrNotExist so callers can trigger first-run
// behavior.
func Load(dir string) (*Config, error) {
	settings, err := loadSettings(dir)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(BookmarksPath(dir))
	if err != nil {
		return nil, err // wraps os.ErrNotExist when absent
	}
	var b tomlBookmarks
	if _, err := toml.Decode(string(data), &b); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", BookmarksPath(dir), err)
	}

	cfg, err := newConfig(b.Bookmark, settings.Group)
	if err != nil {
		return nil, err
	}
	cfg.Browser = settings.Browser
	cfg.LeaderTags = settings.LeaderTags == nil || *settings.LeaderTags // default: show
	search, err := resolveSearch(settings.Search)
	if err != nil {
		return nil, err
	}
	cfg.Search = search
	return cfg, nil
}

// loadSettings reads and parses config.toml. A missing file is not an error —
// it yields zero-value settings (all defaults).
func loadSettings(dir string) (tomlSettings, error) {
	var s tomlSettings
	data, err := os.ReadFile(SettingsPath(dir))
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, err
	}
	if _, err := toml.Decode(string(data), &s); err != nil {
		return s, fmt.Errorf("parsing %s: %w", SettingsPath(dir), err)
	}
	return s, nil
}

// BrowserSetting reads only the browser setting from config.toml, tolerating a
// missing or unreadable file by returning "" (the backend then uses its
// default). Used by paths that act on a raw URL without requiring a full, valid
// bookmark config.
func BrowserSetting(dir string) string {
	s, err := loadSettings(dir)
	if err != nil {
		return ""
	}
	return s.Browser
}

// newConfig validates bookmarks and groups and builds the lookup indexes.
func newConfig(bookmarks []Bookmark, groups []Group) (*Config, error) {
	byKey := make(map[string]Bookmark)
	var keys []string
	for i, b := range bookmarks {
		switch {
		case b.Name == "":
			return nil, fmt.Errorf("bookmark #%d is missing a name", i+1)
		case b.URL == "":
			return nil, fmt.Errorf("bookmark %q is missing a url", b.Name)
		}
		if b.Key == "" {
			continue
		}
		if err := validateKey(b.Name, b.Key, 3); err != nil {
			return nil, err
		}
		k := strings.ToLower(b.Key)
		if prev, ok := byKey[k]; ok {
			return nil, fmt.Errorf("duplicate key %q used by both %q and %q", b.Key, prev.Name, b.Name)
		}
		byKey[k] = b
		keys = append(keys, k)
	}

	// Prefix-free: a key may not be a strict prefix of another, so navigation is
	// unambiguous (no key is both a bookmark and a group).
	for _, a := range keys {
		for _, c := range keys {
			if a != c && strings.HasPrefix(c, a) {
				return nil, fmt.Errorf("key %q is a prefix of %q; a key can't be both a bookmark and a group", a, c)
			}
		}
	}

	groupName := make(map[string]string)
	for _, g := range groups {
		if g.Name == "" {
			return nil, fmt.Errorf("group %q is missing a name", g.Key)
		}
		if err := validateKey("group "+g.Name, g.Key, 2); err != nil {
			return nil, err
		}
		groupName[strings.ToLower(g.Key)] = g.Name
	}

	return &Config{Bookmarks: bookmarks, Groups: groups, byKey: byKey, groupName: groupName}, nil
}

// validateKey checks a leader key/prefix: 1..max runes, no whitespace, and not
// starting with "s" (reserved at the top level for entering search mode, so any
// "s…" key would be unreachable).
func validateKey(owner, key string, max int) error {
	n := utf8.RuneCountInString(key)
	if n < 1 || n > max {
		return fmt.Errorf("%s: key %q must be 1–%d characters", owner, key, max)
	}
	if strings.ContainsFunc(key, unicode.IsSpace) {
		return fmt.Errorf("%s: key %q must not contain spaces", owner, key)
	}
	if strings.HasPrefix(strings.ToLower(key), "s") {
		return fmt.Errorf("%s: key %q may not start with \"s\" — that key is reserved for search mode", owner, key)
	}
	return nil
}

// GroupName returns the friendly label for a key prefix, if one is defined.
func (c *Config) GroupName(prefix string) (string, bool) {
	n, ok := c.groupName[strings.ToLower(prefix)]
	return n, ok
}

// Append adds bookmarks whose URL is not already present, preserving existing
// entries (and their keys). Returns the number actually added.
func (c *Config) Append(bms []Bookmark) int {
	have := make(map[string]bool, len(c.Bookmarks))
	for _, b := range c.Bookmarks {
		have[b.URL] = true
	}
	added := 0
	for _, b := range bms {
		if have[b.URL] {
			continue
		}
		have[b.URL] = true
		c.Bookmarks = append(c.Bookmarks, b)
		added++
	}
	return added
}

// bookmarksHeader tops every machine-written bookmarks file.
const bookmarksHeader = `# bml bookmarks — add ` + "`key = \"x\"`" + ` to any bookmark to pin it to the launcher.
# Updated by ` + "`bml import`" + `; safe to hand-edit. Settings live in config.toml.

`

// RenderBookmarks serializes a config's bookmark entries to bookmarks.toml text
// (with a header comment). Settings are not included — they live in config.toml.
func RenderBookmarks(c *Config) (string, error) {
	var buf bytes.Buffer
	buf.WriteString(bookmarksHeader)
	if err := toml.NewEncoder(&buf).Encode(tomlBookmarks{Bookmark: c.Bookmarks}); err != nil {
		return "", fmt.Errorf("encoding bookmarks: %w", err)
	}
	return buf.String(), nil
}

// SaveBookmarks writes the bookmark entries to bookmarks.toml in dir, backing up
// any existing file to "<path>.bak". config.toml is never touched. It returns
// the backup path ("" if there was nothing to back up).
func SaveBookmarks(dir string, c *Config) (backup string, err error) {
	text, err := RenderBookmarks(c)
	if err != nil {
		return "", err
	}
	path := BookmarksPath(dir)
	if existing, err := os.ReadFile(path); err == nil {
		backup = path + ".bak"
		if err := os.WriteFile(backup, existing, 0o644); err != nil {
			return "", fmt.Errorf("writing backup: %w", err)
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return "", err
	}
	return backup, nil
}

// URLForKey returns the URL bound to a complete key sequence (case-insensitive).
func (c *Config) URLForKey(key string) (string, bool) {
	b, ok := c.byKey[strings.ToLower(key)]
	if !ok {
		return "", false
	}
	return b.URL, true
}

// Dir resolves the config directory. An explicit flag value wins, then
// $BML_CONFIG, then $XDG_CONFIG_HOME/bml, then ~/.config/bml.
func Dir(flag string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	if env := os.Getenv("BML_CONFIG"); env != "" {
		return env, nil
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "bml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "bml"), nil
}

// starterBookmarks is the commented bookmarks.toml written on first run.
const starterBookmarks = `# bml bookmarks — edit this file, then run ` + "`bml`" + ` or ` + "`bml <key>`" + `.
#
# Each bookmark needs a name and url. An optional "key" (1–3 characters) binds it
# to leader mode. Press the key to focus-or-open; uppercase the LAST character to
# force a new tab. Optional "tags" help search find it.
#
# Multi-character keys form groups: with key = "wt", pressing "w" opens the group
# and "t" acts. A key can't be both a bookmark and a group prefix. A key may not
# start with "s" — that key is reserved for search mode (press "s" in the launcher).
#
# Settings (browser, search engines, group labels) live next to this file in
# config.toml.

[[bookmark]]
key = "g"
name = "GitHub"
url = "https://github.com"
tags = ["dev"]

[[bookmark]]
key = "n"
name = "Hacker News"
url = "https://news.ycombinator.com"
tags = ["news"]

# A bookmark without a key is reachable via search (/) but not bound to a letter.
[[bookmark]]
name = "Go docs"
url = "https://pkg.go.dev"
tags = ["dev", "reference"]
`

// starterSettings is the commented config.toml written on first run.
const starterSettings = `# bml settings — browser, search engines, and group labels. Hand-edit freely.
# Bookmarks themselves live next to this file in bookmarks.toml.

# Which macOS browser to drive. Any Chromium browser works:
# "Brave Browser" (default), "Google Chrome", "Arc", "Microsoft Edge".
# browser = "Brave Browser"

# Show tags next to bookmarks in leader mode (default true).
# leader_tags = false

# Search mode (press "s" in the launcher): type a query and Enter searches with
# the default engine, Tab searches with the secondary one. Built-in engines are
# "google", "duckduckgo", and "duckduckgo_lucky". Defaults shown below.
# [search]
# default_engine   = "google"
# secondary_engine = "duckduckgo_lucky"
#
# Define your own engines as name = URL template with "{{input}}":
# [search.engines]
# kagi = "https://kagi.com/search?q={{input}}"

# Optional: give a group prefix a friendly name in the menu.
# [[group]]
# key = "w"
# name = "Work"
`

// WriteStarter creates the config directory and writes any missing starter file
// (bookmarks.toml and config.toml). It reports whether it created the bookmarks
// file (the first-run signal).
func WriteStarter(dir string) (created bool, err error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	created, err = writeIfAbsent(BookmarksPath(dir), starterBookmarks)
	if err != nil {
		return false, err
	}
	if _, err := writeIfAbsent(SettingsPath(dir), starterSettings); err != nil {
		return false, err
	}
	return created, nil
}

// writeIfAbsent writes content to path only if no file exists there, reporting
// whether it created the file.
func writeIfAbsent(path, content string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return false, err
	}
	return true, nil
}
