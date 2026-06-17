package tui

import (
	"testing"

	"bml/internal/config"
)

func corpus() []config.Bookmark {
	return []config.Bookmark{
		{Key: "g", Name: "GitHub", URL: "https://github.com", Tags: []string{"dev"}},
		{Key: "n", Name: "Hacker News", URL: "https://news.ycombinator.com", Tags: []string{"news"}},
		{Name: "Go docs", URL: "https://pkg.go.dev", Tags: []string{"dev", "reference"}},
	}
}

func names(rs []Result) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Bookmark.Name
	}
	return out
}

func TestFilter_EmptyQueryReturnsAllInOrder(t *testing.T) {
	got := names(Filter(corpus(), ""))
	want := []string{"GitHub", "Hacker News", "Go docs"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("empty query order = %v, want %v", got, want)
		}
	}
}

func TestFilter_MatchesByName(t *testing.T) {
	got := Filter(corpus(), "hub")
	if len(got) == 0 || got[0].Bookmark.Name != "GitHub" {
		t.Fatalf("expected GitHub first for 'hub', got %v", names(got))
	}
	if len(got[0].NameMatch) == 0 {
		t.Error("a name match should populate NameMatch for highlighting")
	}
}

func TestFilter_MatchesByURL(t *testing.T) {
	got := Filter(corpus(), "ycomb")
	if len(got) != 1 || got[0].Bookmark.Name != "Hacker News" {
		t.Fatalf("expected Hacker News via url, got %v", names(got))
	}
}

func TestFilter_MatchesByTag(t *testing.T) {
	got := names(Filter(corpus(), "reference"))
	if len(got) != 1 || got[0] != "Go docs" {
		t.Fatalf("expected Go docs via tag, got %v", got)
	}
}

func TestFilter_TagMatchPositions(t *testing.T) {
	got := Filter(corpus(), "dev")
	var gh *Result
	for i := range got {
		if got[i].Bookmark.Name == "GitHub" {
			gh = &got[i]
		}
	}
	if gh == nil {
		t.Fatal("GitHub should match the 'dev' tag")
	}
	// GitHub has one tag, "dev", fully matched -> indexes 0,1,2.
	if len(gh.TagMatch) != 1 || len(gh.TagMatch[0]) != 3 {
		t.Errorf("expected dev tag fully matched, got %v", gh.TagMatch)
	}
	// Go docs has tags [dev, reference]; "dev" matches, "reference" does not.
	var godocs *Result
	for i := range got {
		if got[i].Bookmark.Name == "Go docs" {
			godocs = &got[i]
		}
	}
	if godocs == nil || len(godocs.TagMatch) != 2 || godocs.TagMatch[1] != nil {
		t.Errorf("non-matching tag should be nil, got %v", godocs)
	}
}

func TestFilter_URLMatchPositions(t *testing.T) {
	got := Filter(corpus(), "ycomb")
	if len(got) != 1 || got[0].Bookmark.Name != "Hacker News" {
		t.Fatalf("expected Hacker News via url, got %v", names(got))
	}
	if len(got[0].URLMatch) == 0 {
		t.Error("a url match should populate URLMatch for highlighting")
	}
}

func TestFilter_EmptyQueryNoTagMatch(t *testing.T) {
	for _, r := range Filter(corpus(), "") {
		if r.TagMatch != nil {
			t.Errorf("empty query should not produce tag highlights: %v", r.TagMatch)
		}
	}
}

func TestFilter_TierOrdering(t *testing.T) {
	bms := []config.Bookmark{
		{Name: "Animal Theme", URL: "https://a.com"},    // scatter: a-n-i-m..e
		{Name: "Myanimex", URL: "https://b.com"},        // substring, mid-word
		{Name: "Idle Anime List", URL: "https://c.com"}, // begins a word
		{Name: "Anime Hub", URL: "https://d.com"},       // field prefix
	}
	got := names(Filter(bms, "anime"))
	want := []string{"Anime Hub", "Idle Anime List", "Myanimex", "Animal Theme"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tier order = %v, want %v", got, want)
		}
	}
}

func TestFilter_TierBeatsField(t *testing.T) {
	bms := []config.Bookmark{
		{Name: "Animal Theme", URL: "https://x.com"},                   // name scatter
		{Name: "Zebra", URL: "https://y.com", Tags: []string{"anime"}}, // tag prefix
	}
	got := names(Filter(bms, "anime"))
	if got[0] != "Zebra" {
		t.Errorf("a tag prefix should outrank a name scatter, got %v", got)
	}
}

func TestFilter_ExcludesNonMatches(t *testing.T) {
	if got := Filter(corpus(), "zzzz"); len(got) != 0 {
		t.Fatalf("expected no matches, got %v", names(got))
	}
}

func TestFilter_NameOutranksTag(t *testing.T) {
	// "dev" is a tag on GitHub and Go docs; "Go docs" also contains no "dev" in
	// its name, but a name hit should still outrank a tag-only hit. Query "go"
	// hits "Go docs" by name and nothing else by name.
	got := Filter(corpus(), "go")
	if got[0].Bookmark.Name != "Go docs" {
		t.Errorf("name hit should rank first, got %v", names(got))
	}
}
