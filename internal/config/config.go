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
	"unicode/utf8"

	"github.com/BurntSushi/toml"
)

// Bookmark is one stored URL entry.
type Bookmark struct {
	Name string   `toml:"name"`
	URL  string   `toml:"url"`
	Key  string   `toml:"key,omitempty"`
	Tags []string `toml:"tags,omitempty"`
}

// Config is the loaded, validated bookmark collection plus settings.
type Config struct {
	// Browser is the macOS application name to drive (empty = backend default).
	Browser   string
	Bookmarks []Bookmark
	byKey     map[string]Bookmark
}

// tomlFile mirrors the on-disk layout: a top-level browser setting and an array
// of [[bookmark]] tables.
type tomlFile struct {
	Browser  string     `toml:"browser,omitempty"`
	Bookmark []Bookmark `toml:"bookmark"`
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
	cfg, err := newConfig(f.Bookmark)
	if err != nil {
		return nil, err
	}
	cfg.Browser = f.Browser
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

// newConfig validates bookmarks and builds the key index.
func newConfig(bookmarks []Bookmark) (*Config, error) {
	byKey := make(map[string]Bookmark)
	for i, b := range bookmarks {
		switch {
		case b.Name == "":
			return nil, fmt.Errorf("bookmark #%d is missing a name", i+1)
		case b.URL == "":
			return nil, fmt.Errorf("bookmark %q is missing a url", b.Name)
		}
		if b.Key != "" {
			if utf8.RuneCountInString(b.Key) != 1 {
				return nil, fmt.Errorf("bookmark %q: key %q must be a single character", b.Name, b.Key)
			}
			if prev, ok := byKey[b.Key]; ok {
				return nil, fmt.Errorf("duplicate key %q used by both %q and %q", b.Key, prev.Name, b.Name)
			}
			byKey[b.Key] = b
		}
	}
	return &Config{Bookmarks: bookmarks, byKey: byKey}, nil
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
	if err := toml.NewEncoder(&buf).Encode(tomlFile{Browser: c.Browser, Bookmark: c.Bookmarks}); err != nil {
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

// URLForKey returns the URL bound to a single-character key.
func (c *Config) URLForKey(key string) (string, bool) {
	b, ok := c.byKey[key]
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
# Each bookmark needs a name and url. An optional single-character "key" binds it
# to leader mode (press the key to focus-or-open; press the uppercase to force a
# new tab). Optional "tags" help search find it.

# Which macOS browser to drive. Any Chromium browser works:
# "Brave Browser" (default), "Google Chrome", "Arc", "Microsoft Edge".
# browser = "Brave Browser"

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
