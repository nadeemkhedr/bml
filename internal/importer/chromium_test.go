package importer

import (
	"strings"
	"testing"
)

const sampleJSON = `{
  "roots": {
    "bookmark_bar": {
      "type": "folder", "name": "Bookmarks bar",
      "children": [
        {"type": "url", "name": "GitHub", "url": "https://github.com"},
        {"type": "folder", "name": "Dev", "children": [
          {"type": "url", "name": "Go", "url": "https://go.dev"},
          {"type": "folder", "name": "Refs", "children": [
            {"type": "url", "name": "Pkg", "url": "https://pkg.go.dev"}
          ]}
        ]},
        {"type": "url", "name": "Dup", "url": "https://github.com"},
        {"type": "url", "name": "", "url": "https://example.com/path"},
        {"type": "url", "name": "Ext", "url": "javascript:void(0)"}
      ]
    },
    "other": {
      "type": "folder", "name": "Other",
      "children": [
        {"type": "url", "name": "HN", "url": "https://news.ycombinator.com"}
      ]
    },
    "synced": {"type": "folder", "name": "Mobile", "children": []}
  }
}`

func TestParse_ConvertsTreeWithFolderTags(t *testing.T) {
	bms, err := Parse([]byte(sampleJSON))
	if err != nil {
		t.Fatal(err)
	}

	// GitHub, Go, Pkg, example.com (host fallback), HN — dup + javascript skipped.
	if len(bms) != 5 {
		t.Fatalf("got %d bookmarks, want 5: %+v", len(bms), bms)
	}

	if bms[0].Name != "GitHub" || len(bms[0].Tags) != 0 {
		t.Errorf("top-level bookmark should have no tags: %+v", bms[0])
	}
	if bms[1].Name != "Go" || strings.Join(bms[1].Tags, ",") != "Dev" {
		t.Errorf("nested bookmark tags wrong: %+v", bms[1])
	}
	if bms[2].Name != "Pkg" || strings.Join(bms[2].Tags, ",") != "Dev,Refs" {
		t.Errorf("deeply nested tags should be the folder chain: %+v", bms[2])
	}
	if bms[3].Name != "example.com" {
		t.Errorf("empty name should fall back to host, got %q", bms[3].Name)
	}
	if bms[4].Name != "HN" {
		t.Errorf("bookmark from 'other' root expected, got %q", bms[4].Name)
	}
}

func TestParse_DedupesByURL(t *testing.T) {
	bms, _ := Parse([]byte(sampleJSON))
	seen := map[string]int{}
	for _, b := range bms {
		seen[b.URL]++
	}
	if seen["https://github.com"] != 1 {
		t.Errorf("duplicate URL should appear once, got %d", seen["https://github.com"])
	}
}

func TestParse_SkipsNonWebSchemes(t *testing.T) {
	for _, u := range []string{"javascript:x", "chrome://flags", "brave://settings", "about:blank", ""} {
		if !skipURL(u) {
			t.Errorf("skipURL(%q) should be true", u)
		}
	}
	if skipURL("https://ok.com") {
		t.Error("https should not be skipped")
	}
}

func TestParse_InvalidJSONErrors(t *testing.T) {
	if _, err := Parse([]byte("{not json")); err == nil {
		t.Error("expected error on invalid JSON")
	}
}
