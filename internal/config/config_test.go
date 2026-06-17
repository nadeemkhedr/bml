package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bookmarksDir writes a config dir containing only bookmarks.toml and returns it.
func bookmarksDir(t *testing.T, bookmarks string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(BookmarksPath(dir), []byte(bookmarks), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// dirWith writes a config dir containing both config.toml (settings) and
// bookmarks.toml, and returns it.
func dirWith(t *testing.T, settings, bookmarks string) string {
	t.Helper()
	dir := bookmarksDir(t, bookmarks)
	if err := os.WriteFile(SettingsPath(dir), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoad_Valid(t *testing.T) {
	cfg, err := Load(bookmarksDir(t, `
[[bookmark]]
key = "g"
name = "GitHub"
url = "https://github.com"
tags = ["dev"]

[[bookmark]]
name = "Go docs"
url = "https://pkg.go.dev"
`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Bookmarks) != 2 {
		t.Fatalf("got %d bookmarks, want 2", len(cfg.Bookmarks))
	}
	if url, ok := cfg.URLForKey("g"); !ok || url != "https://github.com" {
		t.Errorf("URLForKey(g) = %q, %v", url, ok)
	}
	if _, ok := cfg.URLForKey("z"); ok {
		t.Errorf("URLForKey(z) should be absent")
	}
}

func TestLoad_ReadsBrowserSetting(t *testing.T) {
	cfg, err := Load(dirWith(t, `browser = "Google Chrome"`, `
[[bookmark]]
name = "X"
url = "https://x.com"
`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Browser != "Google Chrome" {
		t.Errorf("Browser = %q, want %q", cfg.Browser, "Google Chrome")
	}
}

func TestBrowserSetting_ToleratesMissingFile(t *testing.T) {
	if got := BrowserSetting(t.TempDir()); got != "" {
		t.Errorf("missing settings file should yield empty browser, got %q", got)
	}
}

func TestBrowserSetting_ReadsValue(t *testing.T) {
	dir := dirWith(t, "browser = \"Arc\"\n", "")
	if got := BrowserSetting(dir); got != "Arc" {
		t.Errorf("BrowserSetting = %q, want %q", got, "Arc")
	}
}

func TestLoad_DuplicateKeyErrors(t *testing.T) {
	_, err := Load(bookmarksDir(t, `
[[bookmark]]
key = "g"
name = "GitHub"
url = "https://github.com"

[[bookmark]]
key = "g"
name = "GitLab"
url = "https://gitlab.com"
`))
	if err == nil {
		t.Fatal("expected duplicate-key error")
	}
	if !strings.Contains(err.Error(), "duplicate key") {
		t.Errorf("error %q should mention the duplicate key", err)
	}
}

func TestLoad_MissingFieldsError(t *testing.T) {
	if _, err := Load(bookmarksDir(t, "[[bookmark]]\nurl = \"https://x.com\"\n")); err == nil {
		t.Error("expected error for missing name")
	}
	if _, err := Load(bookmarksDir(t, "[[bookmark]]\nname = \"X\"\n")); err == nil {
		t.Error("expected error for missing url")
	}
}

func TestLoad_AcceptsMultiCharKey(t *testing.T) {
	cfg, err := Load(bookmarksDir(t, `
[[bookmark]]
key = "wt"
name = "Work Tasks"
url = "https://tasks.example"
`))
	if err != nil {
		t.Fatalf("multi-char key should be valid: %v", err)
	}
	if url, ok := cfg.URLForKey("wt"); !ok || url != "https://tasks.example" {
		t.Errorf("URLForKey(wt) = %q, %v", url, ok)
	}
}

func TestLoad_KeyTooLongErrors(t *testing.T) {
	_, err := Load(bookmarksDir(t, `
[[bookmark]]
key = "wxyz"
name = "Too Long"
url = "https://x.com"
`))
	if err == nil {
		t.Fatal("expected error for a 4-character key")
	}
}

func TestLoad_PrefixFreeViolationErrors(t *testing.T) {
	_, err := Load(bookmarksDir(t, `
[[bookmark]]
key = "w"
name = "Work"
url = "https://w.example"

[[bookmark]]
key = "wt"
name = "Work Tasks"
url = "https://wt.example"
`))
	if err == nil {
		t.Fatal("expected error: 'w' is a prefix of 'wt'")
	}
	if !strings.Contains(err.Error(), "prefix") {
		t.Errorf("error should explain the prefix conflict, got %q", err)
	}
}

func TestLoad_MissingFileWrapsNotExist(t *testing.T) {
	_, err := Load(t.TempDir()) // empty dir: no bookmarks.toml
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("missing bookmarks file should wrap os.ErrNotExist, got %v", err)
	}
}

func TestLoad_InvalidTOMLErrors(t *testing.T) {
	if _, err := Load(bookmarksDir(t, "this is = not [valid")); err == nil {
		t.Error("expected parse error for invalid bookmarks TOML")
	}
}

func TestLoad_InvalidSettingsTOMLErrors(t *testing.T) {
	if _, err := Load(dirWith(t, "this is = not [valid", "[[bookmark]]\nname=\"X\"\nurl=\"https://x.com\"\n")); err == nil {
		t.Error("expected parse error for invalid settings TOML")
	}
}

func TestAppend_MergesByURLPreservingExisting(t *testing.T) {
	cfg, err := Load(bookmarksDir(t, `
[[bookmark]]
key = "g"
name = "GitHub"
url = "https://github.com"
`))
	if err != nil {
		t.Fatal(err)
	}
	added := cfg.Append([]Bookmark{
		{Name: "GitHub dup", URL: "https://github.com"}, // already present → skipped
		{Name: "Go", URL: "https://go.dev"},             // new
	})
	if added != 1 {
		t.Errorf("added = %d, want 1", added)
	}
	if len(cfg.Bookmarks) != 2 {
		t.Fatalf("total = %d, want 2", len(cfg.Bookmarks))
	}
	if cfg.Bookmarks[0].Key != "g" {
		t.Error("existing keyed bookmark should be preserved")
	}
}

func TestSaveBookmarks_RoundTrip(t *testing.T) {
	cfg := &Config{
		Bookmarks: []Bookmark{
			{Key: "g", Name: "GitHub", URL: "https://github.com", Tags: []string{"dev"}},
			{Name: `Quote " and \ slash`, URL: "https://example.com"},
		},
	}
	dir := t.TempDir()

	if backup, err := SaveBookmarks(dir, cfg); err != nil || backup != "" {
		t.Fatalf("first SaveBookmarks: backup=%q err=%v", backup, err)
	}

	got, err := Load(dir)
	if err != nil {
		t.Fatalf("rendered bookmarks do not load: %v", err)
	}
	if len(got.Bookmarks) != 2 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if got.Bookmarks[1].Name != `Quote " and \ slash` {
		t.Errorf("special characters not preserved: %q", got.Bookmarks[1].Name)
	}
	if url, ok := got.URLForKey("g"); !ok || url != "https://github.com" {
		t.Errorf("key not preserved through round-trip")
	}

	// Second save backs up the existing file.
	backup, err := SaveBookmarks(dir, cfg)
	if err != nil || backup == "" {
		t.Fatalf("second SaveBookmarks should back up: backup=%q err=%v", backup, err)
	}
	if !strings.HasSuffix(backup, ".bak") {
		t.Errorf("backup path = %q", backup)
	}
}

func TestSaveBookmarks_LeavesSettingsUntouched(t *testing.T) {
	// The win of the two-file split: writing bookmarks never rewrites config.toml.
	dir := dirWith(t, "browser = \"Arc\"\n[[group]]\nkey = \"w\"\nname = \"Work\"\n", "")
	before, err := os.ReadFile(SettingsPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Bookmarks: []Bookmark{{Name: "X", URL: "https://x.com"}}}
	if _, err := SaveBookmarks(dir, cfg); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(SettingsPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("SaveBookmarks must not modify config.toml")
	}
}

func TestLoad_GroupsFromSettings(t *testing.T) {
	cfg, err := Load(dirWith(t,
		"[[group]]\nkey = \"w\"\nname = \"Work\"\n",
		"[[bookmark]]\nkey = \"wt\"\nname = \"Work Tasks\"\nurl = \"https://tasks.example\"\n"))
	if err != nil {
		t.Fatalf("does not load: %v", err)
	}
	if name, ok := cfg.GroupName("w"); !ok || name != "Work" {
		t.Errorf("group label not loaded: %q %v", name, ok)
	}
	if _, ok := cfg.URLForKey("wt"); !ok {
		t.Error("grouped bookmark not loaded")
	}
}

func TestLeaderTags_DefaultsTrue(t *testing.T) {
	cfg, err := Load(bookmarksDir(t, "[[bookmark]]\nname=\"X\"\nurl=\"https://x.com\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.LeaderTags {
		t.Error("leader_tags should default to true when unset")
	}
}

func TestLeaderTags_ExplicitFalse(t *testing.T) {
	cfg, err := Load(dirWith(t, "leader_tags = false\n", "[[bookmark]]\nname=\"X\"\nurl=\"https://x.com\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LeaderTags {
		t.Error("leader_tags=false should disable tags")
	}
}

func TestDir_Precedence(t *testing.T) {
	t.Setenv("BML_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	if got, _ := Dir("/explicit/dir"); got != "/explicit/dir" {
		t.Errorf("flag should win, got %q", got)
	}

	t.Setenv("BML_CONFIG", "/env/dir")
	if got, _ := Dir(""); got != "/env/dir" {
		t.Errorf("BML_CONFIG should win over default, got %q", got)
	}

	t.Setenv("BML_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	if got, _ := Dir(""); got != "/xdg/bml" {
		t.Errorf("XDG dir = %q", got)
	}
}

func TestWriteStarter_CreatesBothFilesAndIsIdempotent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub")

	created, err := WriteStarter(dir)
	if err != nil || !created {
		t.Fatalf("first WriteStarter: created=%v err=%v", created, err)
	}
	for _, p := range []string{BookmarksPath(dir), SettingsPath(dir)} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("starter did not create %s: %v", p, err)
		}
	}
	// The starter must itself be valid and loadable.
	if _, err := Load(dir); err != nil {
		t.Fatalf("starter config does not load: %v", err)
	}

	created, err = WriteStarter(dir)
	if err != nil || created {
		t.Fatalf("second WriteStarter should be a no-op: created=%v err=%v", created, err)
	}
}
