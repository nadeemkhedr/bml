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
	cmd := NewRootCmd(fake)
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

func TestRoot_NonURLArgErrors(t *testing.T) {
	fake, err := run(t, "github")
	if err == nil {
		t.Fatal("expected an error for a non-URL, non-dotted argument")
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
