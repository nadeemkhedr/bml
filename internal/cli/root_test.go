package cli

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"bml/internal/browser"
)

// run executes the root command with args against a fresh fake backend.
func run(t *testing.T, args ...string) (*browser.Fake, error) {
	t.Helper()
	fake := &browser.Fake{}
	cmd := NewRootCmd(func(string) browser.Browser { return fake })
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	return fake, cmd.Execute()
}

// tempConfig writes a bookmark file and returns its path.
func tempConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bookmarks.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const sampleConfig = `
[[bookmark]]
key = "g"
name = "GitHub"
url = "https://github.com"
`

const groupedConfig = `
[[group]]
key = "w"
name = "Work"

[[bookmark]]
key = "wt"
name = "Work Tasks"
url = "https://tasks.example"
`

func TestRoot_URLArgFocuses(t *testing.T) {
	fake, err := run(t, "github.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	last, ok := fake.Last()
	if !ok {
		t.Fatal("expected a browser call")
	}
	if last.URL != "github.com" || last.ForceNew {
		t.Errorf("got %+v, want {github.com false}", last)
	}
}

func TestRoot_NewTabFlagForcesNew(t *testing.T) {
	fake, err := run(t, "-n", "github.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	last, _ := fake.Last()
	if !last.ForceNew {
		t.Errorf("expected ForceNew=true, got %+v", last)
	}
}

func TestRoot_KeyArgResolvesToBookmarkURL(t *testing.T) {
	cfg := tempConfig(t, sampleConfig)
	fake, err := run(t, "--config", cfg, "g")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	last, ok := fake.Last()
	if !ok || last.URL != "https://github.com" || last.ForceNew {
		t.Errorf("got %+v, want {https://github.com false}", last)
	}
}

func TestRoot_KeyArgWithNewTabForcesNew(t *testing.T) {
	cfg := tempConfig(t, sampleConfig)
	fake, err := run(t, "--config", cfg, "-n", "g")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	last, _ := fake.Last()
	if last.URL != "https://github.com" || !last.ForceNew {
		t.Errorf("got %+v, want {https://github.com true}", last)
	}
}

func TestRoot_UnboundKeyErrors(t *testing.T) {
	cfg := tempConfig(t, sampleConfig)
	fake, err := run(t, "--config", cfg, "z")
	if err == nil {
		t.Fatal("expected an error for an unbound key")
	}
	if len(fake.Calls) != 0 {
		t.Errorf("browser should not be called, got %+v", fake.Calls)
	}
}

func TestRoot_UsesConfiguredBrowser(t *testing.T) {
	cfg := tempConfig(t, "browser = \"Arc\"\n"+sampleConfig)
	var gotApp string
	cmd := NewRootCmd(func(app string) browser.Browser {
		gotApp = app
		return &browser.Fake{}
	})
	cmd.SetArgs([]string{"--config", cfg, "g"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotApp != "Arc" {
		t.Errorf("browser app = %q, want %q", gotApp, "Arc")
	}
}

func TestRoot_KeySequenceResolves(t *testing.T) {
	cfg := tempConfig(t, groupedConfig)
	fake, err := run(t, "--config", cfg, "wt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if last, _ := fake.Last(); last.URL != "https://tasks.example" || last.ForceNew {
		t.Errorf("got %+v, want {https://tasks.example false}", last)
	}
}

func TestRoot_KeySequenceWithNewTab(t *testing.T) {
	cfg := tempConfig(t, groupedConfig)
	fake, err := run(t, "--config", cfg, "-n", "wt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if last, _ := fake.Last(); !last.ForceNew {
		t.Errorf("expected ForceNew, got %+v", last)
	}
}

func TestRoot_GroupPrefixAloneErrors(t *testing.T) {
	cfg := tempConfig(t, groupedConfig)
	fake, err := run(t, "--config", cfg, "w") // a group, not a bookmark
	if err == nil {
		t.Fatal("expected an error: 'w' is a group prefix, not a bookmark")
	}
	if len(fake.Calls) != 0 {
		t.Errorf("browser should not be called, got %+v", fake.Calls)
	}
}

func TestRoot_NonURLArgErrors(t *testing.T) {
	fake, err := run(t, "github") // 6 chars, no dot
	if err == nil {
		t.Fatal("expected an error for a long non-URL argument")
	}
	if len(fake.Calls) != 0 {
		t.Errorf("browser should not be called on a resolution error, got %+v", fake.Calls)
	}
}

func TestRoot_NoArgsNotYetInteractive(t *testing.T) {
	fake, err := run(t)
	if err == nil {
		t.Fatal("expected a not-implemented error for interactive mode")
	}
	if len(fake.Calls) != 0 {
		t.Errorf("browser should not be called, got %+v", fake.Calls)
	}
}
