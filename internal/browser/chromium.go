package browser

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// DefaultChromiumApp is the macOS application name driven when none is
// configured. All Chromium browsers on macOS share the same AppleScript
// dialect, so this backend also covers Google Chrome, Arc, and Microsoft Edge
// by changing the app name.
const DefaultChromiumApp = "Brave Browser"

// ErrAutomationDenied is returned when macOS has not granted (or has revoked)
// permission for bml to control the browser via Apple events.
var ErrAutomationDenied = errors.New("browser automation permission denied: grant access under System Settings → Privacy & Security → Automation")

// ErrBrowserNotRunning is returned from ListTabs when the configured browser is
// not running. Listing never launches it (that is the whole point of the guard),
// so there are simply no tabs to enumerate.
var ErrBrowserNotRunning = errors.New("browser isn't running")

// notRunningSentinel is what the list script returns when the browser is not
// running, distinguishing it from a running browser with zero open tabs.
const notRunningSentinel = "@@BML_NOT_RUNNING@@"

// Chromium drives a Chromium-based browser on macOS via AppleScript (osascript).
type Chromium struct {
	app string
	// run executes an AppleScript program. It is a seam so tests can assert the
	// generated script without invoking osascript.
	run func(script string) error
	// runOut executes an AppleScript program and returns its stdout. It is a
	// separate seam so the listing path can be tested without invoking osascript.
	runOut func(script string) (string, error)
}

// NewChromium returns a backend that drives the named macOS application
// (e.g. "Brave Browser", "Google Chrome", "Arc", "Microsoft Edge").
func NewChromium(app string) *Chromium {
	if app == "" {
		app = DefaultChromiumApp
	}
	return &Chromium{app: app, run: runOsascript, runOut: runOutOsascript}
}

// OpenOrFocus implements Browser.
func (c *Chromium) OpenOrFocus(url string, forceNew bool) error {
	return c.run(buildScript(c.app, url, forceNew))
}

// ListTabs implements TabLister. It runs a read-only script (no activate, guarded
// by "is running") and parses the result into tabs.
func (c *Chromium) ListTabs() ([]Tab, error) {
	out, err := c.runOut(buildListScript(c.app))
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == notRunningSentinel {
		return nil, ErrBrowserNotRunning
	}
	return parseTabs(out), nil
}

// buildScript renders the AppleScript that focuses a matching tab or opens a new
// one. Matching is a scheme-insensitive substring test: an open tab matches when
// its URL contains the stored URL's schemeless form, so a bookmark for
// "github.com" focuses a tab regardless of http/https. The new tab is opened
// with the real URL (scheme added if missing).
func buildScript(app, url string, forceNew bool) string {
	frag := escapeAppleScript(schemeless(url))
	full := escapeAppleScript(withScheme(url))
	appLit := escapeAppleScript(app)

	var b strings.Builder
	fmt.Fprintf(&b, "tell application \"%s\"\n", appLit)
	b.WriteString("\tactivate\n")
	b.WriteString("\tif not (exists window 1) then reopen\n")
	if !forceNew {
		fmt.Fprintf(&b, "\tset frag to \"%s\"\n", frag)
		b.WriteString("\trepeat with w in windows\n")
		b.WriteString("\t\tset i to 1\n")
		b.WriteString("\t\trepeat with t in tabs of w\n")
		b.WriteString("\t\t\tif URL of t contains frag then\n")
		b.WriteString("\t\t\t\tset active tab index of w to i\n")
		b.WriteString("\t\t\t\tset index of w to 1\n")
		b.WriteString("\t\t\t\treturn\n")
		b.WriteString("\t\t\tend if\n")
		b.WriteString("\t\t\tset i to i + 1\n")
		b.WriteString("\t\tend repeat\n")
		b.WriteString("\tend repeat\n")
	}
	fmt.Fprintf(&b, "\topen location \"%s\"\n", full)
	b.WriteString("end tell\n")
	return b.String()
}

// buildListScript renders the read-only AppleScript that enumerates open tabs.
// Unlike buildScript it does NOT activate the browser (listing must never steal
// focus) and is guarded by "is running" so it never launches a browser that is
// closed — in that case it returns the not-running sentinel. Each tab is emitted
// as "URL<tab>title" on its own line; the URL is emitted first because URLs never
// contain a tab or newline, so the leading field is always clean to parse.
func buildListScript(app string) string {
	appLit := escapeAppleScript(app)

	var b strings.Builder
	// Bind the separators OUTSIDE the tell block: inside `tell application
	// "Brave Browser"` the token `tab` is shadowed by the browser's `tab` element
	// type and would stringify to the literal "tab" instead of an ASCII tab. At
	// the top level `tab`/`linefeed` are the standard text constants.
	b.WriteString("set _sep to tab\n")
	b.WriteString("set _nl to linefeed\n")
	b.WriteString("set _bml to \"\"\n")
	fmt.Fprintf(&b, "if application \"%s\" is running then\n", appLit)
	fmt.Fprintf(&b, "\ttell application \"%s\"\n", appLit)
	b.WriteString("\t\trepeat with w in windows\n")
	b.WriteString("\t\t\trepeat with t in tabs of w\n")
	b.WriteString("\t\t\t\tset _bml to _bml & (URL of t) & _sep & (title of t) & _nl\n")
	b.WriteString("\t\t\tend repeat\n")
	b.WriteString("\t\tend repeat\n")
	b.WriteString("\tend tell\n")
	b.WriteString("\treturn _bml\n")
	b.WriteString("else\n")
	fmt.Fprintf(&b, "\treturn \"%s\"\n", notRunningSentinel)
	b.WriteString("end if\n")
	return b.String()
}

// parseTabs turns the list script's output into tabs. Each non-empty line is
// "URL<tab>title"; lines without a tab (e.g. a stray newline embedded in a title)
// are skipped. Titles may be empty — the display layer falls back to the URL.
func parseTabs(out string) []Tab {
	var tabs []Tab
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		i := strings.IndexByte(line, '\t')
		if i < 0 {
			continue
		}
		tabs = append(tabs, Tab{URL: line[:i], Title: line[i+1:]})
	}
	return tabs
}

// runOsascript executes an AppleScript program and maps known macOS automation
// failures to friendly errors.
func runOsascript(script string) error {
	cmd := exec.Command("osascript", "-e", script)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return osascriptErr(err, stderr.String())
	}
	return nil
}

// runOutOsascript executes an AppleScript program and returns its stdout, mapping
// known automation failures to friendly errors.
func runOutOsascript(script string) (string, error) {
	cmd := exec.Command("osascript", "-e", script)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", osascriptErr(err, stderr.String())
	}
	return stdout.String(), nil
}

// osascriptErr maps an osascript failure (with its stderr) to a friendly error.
func osascriptErr(err error, msg string) error {
	// -1743 / "Not authorized to send Apple events" is the TCC denial.
	if strings.Contains(msg, "-1743") || strings.Contains(msg, "Not authorized") {
		return ErrAutomationDenied
	}
	if msg != "" {
		return fmt.Errorf("osascript: %s", strings.TrimSpace(msg))
	}
	return fmt.Errorf("osascript: %w", err)
}

// schemeless strips a leading "scheme://" so matching ignores http vs https.
func schemeless(url string) string {
	if i := strings.Index(url, "://"); i >= 0 {
		return url[i+len("://"):]
	}
	return url
}

// withScheme prepends https:// when the URL carries no scheme.
func withScheme(url string) string {
	if strings.Contains(url, "://") {
		return url
	}
	return "https://" + url
}

// escapeAppleScript escapes a value for inclusion in an AppleScript double-quoted
// string literal.
func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
