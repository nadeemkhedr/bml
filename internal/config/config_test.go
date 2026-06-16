package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bookmarks.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad_Valid(t *testing.T) {
	cfg, err := Load(writeTemp(t, `
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
	cfg, err := Load(writeTemp(t, `
browser = "Google Chrome"

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
	if got := BrowserSetting(filepath.Join(t.TempDir(), "nope.toml")); got != "" {
		t.Errorf("missing file should yield empty browser, got %q", got)
	}
}

func TestBrowserSetting_ReadsValue(t *testing.T) {
	path := writeTemp(t, "browser = \"Arc\"\n")
	if got := BrowserSetting(path); got != "Arc" {
		t.Errorf("BrowserSetting = %q, want %q", got, "Arc")
	}
}

func TestLoad_DuplicateKeyErrors(t *testing.T) {
	_, err := Load(writeTemp(t, `
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
	if _, err := Load(writeTemp(t, "[[bookmark]]\nurl = \"https://x.com\"\n")); err == nil {
		t.Error("expected error for missing name")
	}
	if _, err := Load(writeTemp(t, "[[bookmark]]\nname = \"X\"\n")); err == nil {
		t.Error("expected error for missing url")
	}
}

func TestLoad_AcceptsMultiCharKey(t *testing.T) {
	cfg, err := Load(writeTemp(t, `
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
	_, err := Load(writeTemp(t, `
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
	_, err := Load(writeTemp(t, `
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
	_, err := Load(filepath.Join(t.TempDir(), "nope.toml"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("missing file should wrap os.ErrNotExist, got %v", err)
	}
}

func TestLoad_InvalidTOMLErrors(t *testing.T) {
	if _, err := Load(writeTemp(t, "this is = not [valid")); err == nil {
		t.Error("expected parse error for invalid TOML")
	}
}

func TestAppend_MergesByURLPreservingExisting(t *testing.T) {
	cfg, err := Load(writeTemp(t, `
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

func TestRenderSave_RoundTrip(t *testing.T) {
	cfg := &Config{
		Browser: "Arc",
		Bookmarks: []Bookmark{
			{Key: "g", Name: "GitHub", URL: "https://github.com", Tags: []string{"dev"}},
			{Name: `Quote " and \ slash`, URL: "https://example.com"},
		},
	}
	path := filepath.Join(t.TempDir(), "out.toml")

	if backup, err := Save(path, cfg); err != nil || backup != "" {
		t.Fatalf("first Save: backup=%q err=%v", backup, err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("rendered config does not load: %v", err)
	}
	if got.Browser != "Arc" || len(got.Bookmarks) != 2 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if got.Bookmarks[1].Name != `Quote " and \ slash` {
		t.Errorf("special characters not preserved: %q", got.Bookmarks[1].Name)
	}
	if url, ok := got.URLForKey("g"); !ok || url != "https://github.com" {
		t.Errorf("key not preserved through round-trip")
	}

	// Second save backs up the existing file.
	backup, err := Save(path, cfg)
	if err != nil || backup == "" {
		t.Fatalf("second Save should back up: backup=%q err=%v", backup, err)
	}
	if !strings.HasSuffix(backup, ".bak") {
		t.Errorf("backup path = %q", backup)
	}
}

func TestRenderSave_PreservesGroups(t *testing.T) {
	cfg := &Config{
		Groups:    []Group{{Key: "w", Name: "Work"}},
		Bookmarks: []Bookmark{{Key: "wt", Name: "Work Tasks", URL: "https://tasks.example"}},
	}
	path := filepath.Join(t.TempDir(), "out.toml")
	if _, err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("does not load: %v", err)
	}
	if name, ok := got.GroupName("w"); !ok || name != "Work" {
		t.Errorf("group label not preserved: %q %v", name, ok)
	}
	if _, ok := got.URLForKey("wt"); !ok {
		t.Error("grouped bookmark not preserved")
	}
}

func TestPath_Precedence(t *testing.T) {
	t.Setenv("BML_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	if got, _ := Path("/explicit/flag.toml"); got != "/explicit/flag.toml" {
		t.Errorf("flag should win, got %q", got)
	}

	t.Setenv("BML_CONFIG", "/env/path.toml")
	if got, _ := Path(""); got != "/env/path.toml" {
		t.Errorf("BML_CONFIG should win over default, got %q", got)
	}

	t.Setenv("BML_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	if got, _ := Path(""); got != "/xdg/bml/bookmarks.toml" {
		t.Errorf("XDG path = %q", got)
	}
}

func TestWriteStarter_CreatesAndIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "bookmarks.toml")

	created, err := WriteStarter(path)
	if err != nil || !created {
		t.Fatalf("first WriteStarter: created=%v err=%v", created, err)
	}
	// The starter must itself be valid and loadable.
	if _, err := Load(path); err != nil {
		t.Fatalf("starter config does not load: %v", err)
	}

	created, err = WriteStarter(path)
	if err != nil || created {
		t.Fatalf("second WriteStarter should be a no-op: created=%v err=%v", created, err)
	}
}
