package browser

import (
	"errors"
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

func TestBuildListScript_ReadOnlyAndGuarded(t *testing.T) {
	script := buildListScript("Brave Browser")

	if strings.Contains(script, "activate") {
		t.Errorf("list script must not activate the browser:\n%s", script)
	}
	if !strings.Contains(script, `application "Brave Browser" is running`) {
		t.Errorf("list script should guard on is running:\n%s", script)
	}
	if !strings.Contains(script, "repeat with w in windows") || !strings.Contains(script, "repeat with t in tabs of w") {
		t.Errorf("list script should iterate windows and tabs:\n%s", script)
	}
	if !strings.Contains(script, "URL of t") || !strings.Contains(script, "title of t") {
		t.Errorf("list script should emit URL and title:\n%s", script)
	}
	if !strings.Contains(script, notRunningSentinel) {
		t.Errorf("list script should return the not-running sentinel:\n%s", script)
	}
	// The tab separator must be bound OUTSIDE the tell block — inside it, the app's
	// dictionary shadows the `tab` constant and it stringifies to literal "tab".
	sepDecl := strings.Index(script, "set _sep to tab")
	tellAt := strings.Index(script, "tell application")
	if sepDecl < 0 || tellAt < 0 || sepDecl > tellAt {
		t.Errorf("separator must be bound before the tell block to avoid the app's tab shadowing:\n%s", script)
	}
}

func TestParseTabs(t *testing.T) {
	out := "https://github.com\tGitHub\nhttps://news.ycombinator.com\tHacker News\n"
	got := parseTabs(out)
	if len(got) != 2 {
		t.Fatalf("got %d tabs, want 2: %+v", len(got), got)
	}
	if got[0] != (Tab{URL: "https://github.com", Title: "GitHub"}) {
		t.Errorf("tab 0 = %+v", got[0])
	}
	if got[1] != (Tab{URL: "https://news.ycombinator.com", Title: "Hacker News"}) {
		t.Errorf("tab 1 = %+v", got[1])
	}
}

func TestParseTabs_EmptyOutput(t *testing.T) {
	if got := parseTabs(""); len(got) != 0 {
		t.Errorf("empty output should yield no tabs, got %+v", got)
	}
}

func TestParseTabs_BlankTitleAndStrayLines(t *testing.T) {
	// A blank title is preserved (display falls back to the URL); a line with no
	// tab (e.g. a newline embedded in a title) is skipped.
	out := "https://example.com\t\nstray line without a tab\nhttps://pkg.go.dev\tGo docs\n"
	got := parseTabs(out)
	if len(got) != 2 {
		t.Fatalf("got %d tabs, want 2: %+v", len(got), got)
	}
	if got[0] != (Tab{URL: "https://example.com", Title: ""}) {
		t.Errorf("blank-title tab = %+v", got[0])
	}
	if got[1].Title != "Go docs" {
		t.Errorf("tab after stray line = %+v", got[1])
	}
}

func TestListTabs_UsesRunOutSeam(t *testing.T) {
	var gotScript string
	c := &Chromium{app: "Brave Browser", runOut: func(script string) (string, error) {
		gotScript = script
		return "https://github.com\tGitHub\n", nil
	}}
	tabs, err := c.ListTabs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotScript, "is running") {
		t.Errorf("ListTabs did not run the list script:\n%s", gotScript)
	}
	if len(tabs) != 1 || tabs[0].Title != "GitHub" {
		t.Errorf("got %+v", tabs)
	}
}

func TestListTabs_NotRunningSentinel(t *testing.T) {
	c := &Chromium{app: "Brave Browser", runOut: func(string) (string, error) {
		return notRunningSentinel + "\n", nil
	}}
	_, err := c.ListTabs()
	if !errors.Is(err, ErrBrowserNotRunning) {
		t.Errorf("got %v, want ErrBrowserNotRunning", err)
	}
}

func TestListTabs_PropagatesError(t *testing.T) {
	c := &Chromium{app: "Brave Browser", runOut: func(string) (string, error) {
		return "", ErrAutomationDenied
	}}
	if _, err := c.ListTabs(); !errors.Is(err, ErrAutomationDenied) {
		t.Errorf("got %v, want ErrAutomationDenied", err)
	}
}
