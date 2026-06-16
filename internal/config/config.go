// Package config loads and validates the hand-edited bookmark file and resolves
// CLI arguments to a URL.
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
	Bookmarks  []Bookmark
	Groups     []Group
	leaderTags *bool // raw value as written, preserved on round-trip
	byKey      map[string]Bookmark
	groupName  map[string]string
}

// tomlFile mirrors the on-disk layout: top-level settings, optional [[group]]
// labels, and an array of [[bookmark]] tables.
type tomlFile struct {
	Browser    string     `toml:"browser,omitempty"`
	LeaderTags *bool      `toml:"leader_tags,omitempty"` // nil = default (show)
	Group      []Group    `toml:"group,omitempty"`
	Bookmark   []Bookmark `toml:"bookmark"`
}

// Load reads, parses, and validates the bookmark file at path. A missing file
// returns an error wrapping os.ErrNotExist so callers can trigger first-run
// behavior.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err // wraps os.ErrNotExist when absent
	}
	var f tomlFile
	if _, err := toml.Decode(string(data), &f); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	cfg, err := newConfig(f.Bookmark, f.Group)
	if err != nil {
		return nil, err
	}
	cfg.Browser = f.Browser
	cfg.leaderTags = f.LeaderTags
	cfg.LeaderTags = f.LeaderTags == nil || *f.LeaderTags // default: show
	return cfg, nil
}

// BrowserSetting reads only the browser setting from the file, tolerating a
// missing or unreadable file by returning "" (the backend then uses its
// default). Used by paths that act on a raw URL without requiring a full,
// valid bookmark config.
func BrowserSetting(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var f tomlFile
	if _, err := toml.Decode(string(data), &f); err != nil {
		return ""
	}
	return f.Browser
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

// validateKey checks a leader key/prefix: 1..max runes, no whitespace.
func validateKey(owner, key string, max int) error {
	n := utf8.RuneCountInString(key)
	if n < 1 || n > max {
		return fmt.Errorf("%s: key %q must be 1–%d characters", owner, key, max)
	}
	if strings.ContainsFunc(key, unicode.IsSpace) {
		return fmt.Errorf("%s: key %q must not contain spaces", owner, key)
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

// renderHeader tops every machine-written config.
const renderHeader = `# bml bookmarks — add ` + "`key = \"x\"`" + ` to any bookmark to pin it to the launcher.
# Updated by ` + "`bml import`" + `; safe to hand-edit.

`

// Render serializes a config to TOML text (with a header comment).
func Render(c *Config) (string, error) {
	var buf bytes.Buffer
	buf.WriteString(renderHeader)
	if err := toml.NewEncoder(&buf).Encode(tomlFile{Browser: c.Browser, LeaderTags: c.leaderTags, Group: c.Groups, Bookmark: c.Bookmarks}); err != nil {
		return "", fmt.Errorf("encoding config: %w", err)
	}
	return buf.String(), nil
}

// Save writes the config to path, backing up any existing file to "<path>.bak".
// It returns the backup path ("" if there was nothing to back up).
func Save(path string, c *Config) (backup string, err error) {
	text, err := Render(c)
	if err != nil {
		return "", err
	}
	if existing, err := os.ReadFile(path); err == nil {
		backup = path + ".bak"
		if err := os.WriteFile(backup, existing, 0o644); err != nil {
			return "", fmt.Errorf("writing backup: %w", err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
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

// Path resolves the bookmark file location. An explicit flag value wins, then
// $BML_CONFIG, then $XDG_CONFIG_HOME/bml, then ~/.config/bml.
func Path(flag string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	if env := os.Getenv("BML_CONFIG"); env != "" {
		return env, nil
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "bml", "bookmarks.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "bml", "bookmarks.toml"), nil
}

// starter is the commented bookmark file written on first run.
const starter = `# bml bookmarks — edit this file, then run ` + "`bml`" + ` or ` + "`bml <key>`" + `.
#
# Each bookmark needs a name and url. An optional "key" (1–3 characters) binds it
# to leader mode. Press the key to focus-or-open; uppercase the LAST character to
# force a new tab. Optional "tags" help search find it.
#
# Multi-character keys form groups: with key = "wt", pressing "w" opens the group
# and "t" acts. A key can't be both a bookmark and a group prefix.

# Which macOS browser to drive. Any Chromium browser works:
# "Brave Browser" (default), "Google Chrome", "Arc", "Microsoft Edge".
# browser = "Brave Browser"

# Show tags next to bookmarks in leader mode (default true).
# leader_tags = false

# Optional: give a group prefix a friendly name in the menu.
# [[group]]
# key = "w"
# name = "Work"

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

# A grouped key: press "w" then "t".
# [[bookmark]]
# key = "wt"
# name = "Work Tasks"
# url = "https://app.clickup.com"
# tags = ["work"]

# A bookmark without a key is reachable via search (/) but not bound to a letter.
[[bookmark]]
name = "Go docs"
url = "https://pkg.go.dev"
tags = ["dev", "reference"]
`

// WriteStarter creates the parent directory and writes the starter file if no
// file exists at path. It reports whether it created the file.
func WriteStarter(path string) (created bool, err error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(path, []byte(starter), 0o644); err != nil {
		return false, err
	}
	return true, nil
}
