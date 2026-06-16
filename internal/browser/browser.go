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
