package browser

import (
	"strings"
	"testing"
)

func TestBuildScript_FocusBranchIncludesMatchLoop(t *testing.T) {
	script := buildScript("Brave Browser", "https://github.com", false)

	if !strings.Contains(script, `tell application "Brave Browser"`) {
		t.Errorf("script missing app target:\n%s", script)
	}
	if !strings.Contains(script, "repeat with w in windows") {
		t.Errorf("focus script should iterate windows:\n%s", script)
	}
	// Matching uses the schemeless form so http/https both match.
	if !strings.Contains(script, `set frag to "github.com"`) {
		t.Errorf("focus script should match the schemeless fragment:\n%s", script)
	}
	// The new tab still opens the real URL with its scheme.
	if !strings.Contains(script, `open location "https://github.com"`) {
		t.Errorf("script should open the full URL:\n%s", script)
	}
}

func TestBuildScript_ForceNewSkipsMatchLoop(t *testing.T) {
	script := buildScript("Brave Browser", "https://github.com", true)

	if strings.Contains(script, "repeat with w in windows") {
		t.Errorf("forceNew script must not iterate/match tabs:\n%s", script)
	}
	if !strings.Contains(script, `open location "https://github.com"`) {
		t.Errorf("forceNew script should open the URL:\n%s", script)
	}
}

func TestBuildScript_AddsSchemeWhenMissing(t *testing.T) {
	script := buildScript("Brave Browser", "github.com", false)
	if !strings.Contains(script, `open location "https://github.com"`) {
		t.Errorf("schemeless input should open as https://:\n%s", script)
	}
	if !strings.Contains(script, `set frag to "github.com"`) {
		t.Errorf("schemeless input should match on the bare host:\n%s", script)
	}
}

func TestOpenOrFocus_UsesRunSeam(t *testing.T) {
	var got string
	c := &Chromium{app: "Brave Browser", run: func(script string) error {
		got = script
		return nil
	}}
	if err := c.OpenOrFocus("github.com", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "open location") {
		t.Errorf("run did not receive a script:\n%s", got)
	}
}

func TestSchemeless(t *testing.T) {
	cases := map[string]string{
		"https://github.com":      "github.com",
		"http://github.com/notif": "github.com/notif",
		"github.com":              "github.com",
	}
	for in, want := range cases {
		if got := schemeless(in); got != want {
			t.Errorf("schemeless(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWithScheme(t *testing.T) {
	cases := map[string]string{
		"github.com":         "https://github.com",
		"https://github.com": "https://github.com",
		"http://example.org": "http://example.org",
	}
	for in, want := range cases {
		if got := withScheme(in); got != want {
			t.Errorf("withScheme(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEscapeAppleScript(t *testing.T) {
	if got := escapeAppleScript(`a"b\c`); got != `a\"b\\c` {
		t.Errorf("escapeAppleScript = %q", got)
	}
}
