package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bml/internal/browser"
	"bml/internal/config"
)

const braveJSON = `{
  "roots": {
    "bookmark_bar": {"type": "folder", "name": "bar", "children": [
      {"type": "url", "name": "GitHub", "url": "https://github.com"},
      {"type": "folder", "name": "Dev", "children": [
        {"type": "url", "name": "Go", "url": "https://go.dev"}
      ]}
    ]},
    "other": {"type": "folder", "name": "other", "children": []},
    "synced": {"type": "folder", "name": "synced", "children": []}
  }
}`

// fakeBraveHome creates a temp HOME containing a Brave Bookmarks file and points
// $HOME at it for the duration of the test.
func fakeBraveHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	path := filepath.Join(home, "Library", "Application Support", "BraveSoftware", "Brave-Browser", "Default", "Bookmarks")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(braveJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
}

func runImport(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var out, errb bytes.Buffer
	cmd := NewRootCmd(func(string) browser.Browser { return &browser.Fake{} })
	cmd.SetArgs(args)
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	err := cmd.Execute() // must run before reading the buffers
	return out.String(), errb.String(), err
}

func TestImport_WritesConfigFromBrave(t *testing.T) {
	fakeBraveHome(t)
	dir := t.TempDir()

	if _, _, err := runImport(t, "import", "brave", "--config", dir); err != nil {
		t.Fatalf("import failed: %v", err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("written config does not load: %v", err)
	}
	if len(cfg.Bookmarks) != 2 {
		t.Fatalf("got %d bookmarks, want 2", len(cfg.Bookmarks))
	}
	// The nested bookmark should carry its folder as a tag.
	var go_ config.Bookmark
	for _, b := range cfg.Bookmarks {
		if b.URL == "https://go.dev" {
			go_ = b
		}
	}
	if strings.Join(go_.Tags, ",") != "Dev" {
		t.Errorf("folder tag missing: %+v", go_)
	}
}

func TestImport_DryRunDoesNotWrite(t *testing.T) {
	fakeBraveHome(t)
	dir := t.TempDir()

	out, _, err := runImport(t, "import", "brave", "--config", dir, "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "[[bookmark]]") {
		t.Error("dry run should print the rendered bookmarks to stdout")
	}
	if _, err := os.Stat(config.BookmarksPath(dir)); !os.IsNotExist(err) {
		t.Error("dry run must not write the bookmarks file")
	}
}

func TestImport_MergeIsIdempotentAndBacksUp(t *testing.T) {
	fakeBraveHome(t)
	dir := t.TempDir()

	if _, _, err := runImport(t, "import", "brave", "--config", dir); err != nil {
		t.Fatal(err)
	}
	// Second import: nothing new, and the prior file is backed up.
	if _, errOut, err := runImport(t, "import", "brave", "--config", dir); err != nil {
		t.Fatal(err)
	} else if !strings.Contains(errOut, "imported 0 new") {
		t.Errorf("re-import should add nothing new, got: %q", errOut)
	}
	if _, err := os.Stat(config.BookmarksPath(dir) + ".bak"); err != nil {
		t.Errorf("expected a .bak backup after re-import: %v", err)
	}
}

func TestImport_LeavesSettingsUntouched(t *testing.T) {
	fakeBraveHome(t)
	dir := t.TempDir()
	settings := "browser = \"Arc\"\n[search]\ndefault_engine = \"duckduckgo\"\n"
	if err := os.WriteFile(config.SettingsPath(dir), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	// Even --replace must not touch config.toml.
	if _, _, err := runImport(t, "import", "brave", "--config", dir, "--replace"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(config.SettingsPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != settings {
		t.Errorf("import --replace modified config.toml:\n%s", got)
	}
}

func TestImport_UnknownBrowserErrors(t *testing.T) {
	if _, _, err := runImport(t, "import", "safari"); err == nil {
		t.Error("expected error for an unsupported browser")
	}
}
