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

// Chromium drives a Chromium-based browser on macOS via AppleScript (osascript).
type Chromium struct {
	app string
	// run executes an AppleScript program. It is a seam so tests can assert the
	// generated script without invoking osascript.
	run func(script string) error
}

// NewChromium returns a backend that drives the named macOS application
// (e.g. "Brave Browser", "Google Chrome", "Arc", "Microsoft Edge").
func NewChromium(app string) *Chromium {
	if app == "" {
		app = DefaultChromiumApp
	}
	return &Chromium{app: app, run: runOsascript}
}

// OpenOrFocus implements Browser.
func (c *Chromium) OpenOrFocus(url string, forceNew bool) error {
	return c.run(buildScript(c.app, url, forceNew))
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

// runOsascript executes an AppleScript program and maps known macOS automation
// failures to friendly errors.
func runOsascript(script string) error {
	cmd := exec.Command("osascript", "-e", script)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := stderr.String()
		// -1743 / "Not authorized to send Apple events" is the TCC denial.
		if strings.Contains(msg, "-1743") || strings.Contains(msg, "Not authorized") {
			return ErrAutomationDenied
		}
		if msg != "" {
			return fmt.Errorf("osascript: %s", strings.TrimSpace(msg))
		}
		return fmt.Errorf("osascript: %w", err)
	}
	return nil
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
