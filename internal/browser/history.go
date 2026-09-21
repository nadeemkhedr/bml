package browser

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Visit is one entry from the browser's history: a URL the user has navigated
// to, with the counts that rank address-bar autocomplete. TypedCount (how often
// the user typed the address) is the strongest "I go here" signal — exactly what
// a browser's omnibox weights.
type Visit struct {
	URL        string
	Title      string
	TypedCount int
	VisitCount int
}

// HistoryLister is the optional capability of reading the browser's own visit
// history, used to personalize browse-mode autocomplete with the sites the user
// actually visits. Like TabLister it is kept separate from the core Browser seam
// (see ADR 0005) so a backend that can't read history simply doesn't implement
// it, and callers type-assert for it.
type HistoryLister interface {
	ListHistory() ([]Visit, error)
}

// historyLimit caps how many history rows are read (most-typed/visited first).
// Browse-mode autocomplete only ever shows a handful; this bounds parsing.
const historyLimit = 3000

// historyProfile is the Chromium profile whose history is read. Interactive bml
// uses the browser's Default profile, matching `bml import`'s default.
const historyProfile = "Default"

// chromiumSupportDir maps a Chromium app's AppleScript name to its macOS
// Application Support folder segments, for locating the on-disk profile. Kept
// here (rather than imported from internal/importer, which has the same table
// for bookmark files) so the browser backend stays free of that dependency; both
// lists are tiny and change together rarely.
func chromiumSupportDir(app string) ([]string, bool) {
	switch app {
	case "Brave Browser", "Brave Browser Beta", "Brave Browser Nightly":
		return []string{"BraveSoftware", "Brave-Browser"}, true
	case "Google Chrome", "Google Chrome Canary":
		return []string{"Google", "Chrome"}, true
	case "Microsoft Edge":
		return []string{"Microsoft Edge"}, true
	case "Arc":
		return []string{"Arc", "User Data"}, true
	}
	return nil, false
}

// ListHistory implements HistoryLister for the macOS Chromium backend, reading
// the configured browser's Default-profile history.
func (c *Chromium) ListHistory() ([]Visit, error) {
	dir, ok := chromiumSupportDir(c.app)
	if !ok {
		return nil, fmt.Errorf("don't know where %q stores its history", c.app)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	parts := append([]string{home, "Library", "Application Support"}, dir...)
	parts = append(parts, historyProfile, "History")
	dbPath := filepath.Join(parts...)
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("no %s history at %s", c.app, dbPath)
	}
	return readVisits(dbPath)
}

// historyQuery selects the most address-bar-relevant rows. Numeric columns come
// first so a tab inside a page title (rare, but possible) can't shift the column
// the URL is parsed from — see parseVisits.
var historyQuery = "SELECT typed_count, visit_count, url, title FROM urls " +
	"WHERE hidden = 0 AND (typed_count > 0 OR visit_count > 0) " +
	"ORDER BY typed_count DESC, visit_count DESC, last_visit_time DESC " +
	"LIMIT " + strconv.Itoa(historyLimit) + ";"

// readVisits runs the history query through the system sqlite3 and parses the
// result. It opens the live database with a file:…?immutable=1 URI: SQLite then
// assumes the file never changes, so it takes no lock (Chromium holds one while
// running) and makes no copy, reading in a few milliseconds. The cost is that
// immutable mode ignores the write-ahead log, so the current session's very
// newest visits may be absent — irrelevant when ranking by accumulated counts.
//
// Shelling out to sqlite3 (present on macOS) keeps bml a single pure-Go, zero-CGO
// binary — the same "delegate to an OS tool" choice as the AppleScript backend.
func readVisits(dbPath string) ([]Visit, error) {
	bin, err := exec.LookPath("sqlite3")
	if err != nil {
		return nil, fmt.Errorf("sqlite3 not found: %w", err)
	}
	// Percent-encode the path into the URI (the profile path contains a space);
	// the read is exec'd without a shell, so no shell quoting applies.
	uri := (&url.URL{Scheme: "file", Path: dbPath}).String() + "?immutable=1"

	var stdout, stderr strings.Builder
	cmd := exec.Command(bin, "-separator", "\t", uri, historyQuery)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("reading history: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return parseVisits(stdout.String()), nil
}

// parseVisits parses the tab-separated "typed_count, visit_count, url, title"
// rows from sqlite3. A malformed row is skipped rather than failing the whole
// read. Title is taken last (via SplitN) so a tab within it can't corrupt the
// URL field; the numeric and URL fields never contain tabs.
func parseVisits(tsv string) []Visit {
	var visits []Visit
	for _, line := range strings.Split(tsv, "\n") {
		if line == "" {
			continue
		}
		f := strings.SplitN(line, "\t", 4)
		if len(f) < 3 {
			continue
		}
		typed, err1 := strconv.Atoi(strings.TrimSpace(f[0]))
		visited, err2 := strconv.Atoi(strings.TrimSpace(f[1]))
		if err1 != nil || err2 != nil || f[2] == "" {
			continue
		}
		v := Visit{TypedCount: typed, VisitCount: visited, URL: f[2]}
		if len(f) == 4 {
			v.Title = f[3]
		}
		visits = append(visits, v)
	}
	return visits
}
