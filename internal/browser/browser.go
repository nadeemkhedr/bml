// Package browser defines the seam between bml and the platform/browser that
// actually focuses or opens tabs. Everything in bml acts on a URL through the
// Browser interface; nothing else knows which backend is behind it.
//
// See docs/adr/0001-reimplement-browser-automation-in-go.md for why the seam is
// a single coarse method.
package browser

// Browser is the one pluggable seam for acting on a URL.
//
// OpenOrFocus focuses an already-open tab whose URL matches url, or opens a new
// tab if none matches. When forceNew is true it skips matching and always opens
// a new tab. How a tab is matched is owned by the backend (v1: a
// scheme-insensitive substring match performed inside AppleScript).
type Browser interface {
	OpenOrFocus(url string, forceNew bool) error
}

// Tab is a single live tab open in the browser, captured as a title/URL pair.
// It is ephemeral browser state enumerated on demand (never persisted), distinct
// from a stored bookmark.
type Tab struct {
	Title string
	URL   string
}

// TabLister is an optional capability a backend may implement on top of Browser:
// enumerating the browser's currently open tabs. It is kept separate from the
// core Browser seam so a backend that cannot (or chooses not to) list tabs simply
// does not implement it, and tab mode is unavailable on it. See
// docs/adr/0005-tab-mode-and-tablister-capability.md.
type TabLister interface {
	// ListTabs returns the open tabs of the browser, flat across all windows.
	// It must not steal focus or launch a browser that is not running.
	ListTabs() ([]Tab, error)
}
