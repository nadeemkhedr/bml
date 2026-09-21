package browser

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseVisits(t *testing.T) {
	// typed_count, visit_count, url, title — tab-separated. Includes a title with
	// an embedded tab (must not corrupt the URL) and a malformed row (skipped).
	tsv := "236\t1342\thttps://youtube.com\tYouTube\n" +
		"5\t9\thttps://github.com/foo\tfoo\tbar\n" + // title contains a tab
		"oops\t1\thttps://bad.example\tBad\n" + // non-numeric typed_count -> skipped
		"0\t3\thttps://news.example\n" + // no title field -> allowed
		"\n" // blank line -> skipped

	got := parseVisits(tsv)
	if len(got) != 3 {
		t.Fatalf("got %d visits, want 3: %+v", len(got), got)
	}
	if got[0].URL != "https://youtube.com" || got[0].TypedCount != 236 || got[0].VisitCount != 1342 {
		t.Errorf("row 0 = %+v", got[0])
	}
	if got[1].URL != "https://github.com/foo" || got[1].Title != "foo\tbar" {
		t.Errorf("row 1 should keep the URL intact and fold the tab into the title, got %+v", got[1])
	}
	if got[2].URL != "https://news.example" || got[2].Title != "" {
		t.Errorf("row 2 (no title) = %+v", got[2])
	}
}

// TestReadVisits exercises the real sqlite3 immutable read against a temp DB
// shaped like Chromium's urls table. Skipped where sqlite3 isn't installed.
func TestReadVisits(t *testing.T) {
	sqlite3, err := exec.LookPath("sqlite3")
	if err != nil {
		t.Skip("sqlite3 not installed")
	}
	dir := t.TempDir()
	db := filepath.Join(dir, "History")
	seed := `CREATE TABLE urls(id INTEGER PRIMARY KEY, url TEXT, title TEXT, visit_count INT, typed_count INT, last_visit_time INT, hidden INT DEFAULT 0);
INSERT INTO urls(url,title,visit_count,typed_count,last_visit_time,hidden) VALUES
 ('https://youtube.com','YouTube',1342,236,100,0),
 ('https://amazon.com','Amazon',83,46,90,0),
 ('https://hidden.example','Hidden',999,999,80,1),
 ('https://never.example','Never',0,0,70,0);`
	cmd := exec.Command(sqlite3, db)
	cmd.Stdin = strings.NewReader(seed)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("seeding temp DB failed: %v: %s", err, out)
	}

	got, err := readVisits(db)
	if err != nil {
		t.Fatalf("readVisits: %v", err)
	}
	// hidden and zero-count rows are excluded; ordered by typed_count desc.
	if len(got) != 2 {
		t.Fatalf("got %d visits, want 2 (hidden + zero-count excluded): %+v", len(got), got)
	}
	if got[0].URL != "https://youtube.com" || got[1].URL != "https://amazon.com" {
		t.Errorf("ranking by typed_count wrong: %+v", got)
	}
}

func TestChromiumSupportDir(t *testing.T) {
	if _, ok := chromiumSupportDir("Brave Browser"); !ok {
		t.Error("Brave Browser should resolve to a support dir")
	}
	if _, ok := chromiumSupportDir("Some Unknown Browser"); ok {
		t.Error("an unknown app must not resolve (browse mode then falls back)")
	}
}
