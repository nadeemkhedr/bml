package importer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLookup_NameAndAliasCaseInsensitive(t *testing.T) {
	if s, ok := Lookup("Brave"); !ok || s.Name != "brave" {
		t.Errorf("Lookup(Brave) = %+v, %v", s, ok)
	}
	if s, ok := Lookup("chromium"); !ok || s.Name != "chrome" {
		t.Errorf("alias chromium should resolve to chrome, got %+v %v", s, ok)
	}
	if _, ok := Lookup("safari"); ok {
		t.Error("safari is not a Chromium source and should not resolve")
	}
}

func TestBookmarksPath(t *testing.T) {
	s, _ := Lookup("brave")
	got := s.BookmarksPath("/home/me", "Default")
	want := "/home/me/Library/Application Support/BraveSoftware/Brave-Browser/Default/Bookmarks"
	if got != want {
		t.Errorf("BookmarksPath = %q, want %q", got, want)
	}
}

func TestRead_ParsesFromProfile(t *testing.T) {
	home := t.TempDir()
	s, _ := Lookup("brave")
	path := s.BookmarksPath(home, "Default")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(sampleJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	bms, err := Read(home, "Default", s)
	if err != nil {
		t.Fatal(err)
	}
	if len(bms) != 5 {
		t.Fatalf("expected 5 bookmarks, got %d", len(bms))
	}
}

func TestRead_MissingFileGivesHelpfulError(t *testing.T) {
	s, _ := Lookup("brave")
	_, err := Read(t.TempDir(), "Default", s)
	if err == nil || !strings.Contains(err.Error(), "Brave Browser") {
		t.Errorf("missing file should explain which browser/profile, got %v", err)
	}
}
