package config

import (
	"strings"
	"testing"
)

func TestEngineURL_EscapesQuery(t *testing.T) {
	e := Engine{Name: "google", Template: "https://www.google.com/search?q={{input}}"}
	if got := e.URL("foo bar"); got != "https://www.google.com/search?q=foo+bar" {
		t.Errorf("spaces: URL = %q", got)
	}
	if got := e.URL("c++ a & b"); got != "https://www.google.com/search?q=c%2B%2B+a+%26+b" {
		t.Errorf("special chars: URL = %q", got)
	}
}

func TestEngineURL_LuckyBang(t *testing.T) {
	e := Engine{Name: "duckduckgo_lucky", Template: "https://duckduckgo.com/?q=!ducky+{{input}}"}
	if got := e.URL("golang context"); got != "https://duckduckgo.com/?q=!ducky+golang+context" {
		t.Errorf("URL = %q", got)
	}
}

func TestDefaultSearch(t *testing.T) {
	s := DefaultSearch()
	if s.Primary.Name != "google" || s.Secondary.Name != "duckduckgo_lucky" {
		t.Errorf("defaults = %q / %q, want google / duckduckgo_lucky", s.Primary.Name, s.Secondary.Name)
	}
}

const aBookmark = "[[bookmark]]\nname = \"X\"\nurl = \"https://x.com\"\n"

func TestLoad_SearchDefaultsWhenAbsent(t *testing.T) {
	cfg, err := Load(bookmarksDir(t, aBookmark))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Search.Primary.Name != "google" || cfg.Search.Secondary.Name != "duckduckgo_lucky" {
		t.Errorf("got %q / %q, want google / duckduckgo_lucky", cfg.Search.Primary.Name, cfg.Search.Secondary.Name)
	}
}

func TestLoad_SearchSelectsBuiltins(t *testing.T) {
	cfg, err := Load(dirWith(t, `
[search]
default_engine = "duckduckgo"
secondary_engine = "google"
`, aBookmark))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Search.Primary.Name != "duckduckgo" {
		t.Errorf("primary = %q, want duckduckgo", cfg.Search.Primary.Name)
	}
	if cfg.Search.Secondary.Name != "google" {
		t.Errorf("secondary = %q, want google", cfg.Search.Secondary.Name)
	}
}

func TestLoad_CustomEngineUsableByName(t *testing.T) {
	cfg, err := Load(dirWith(t, `
[search]
default_engine = "kagi"

[search.engines]
kagi = "https://kagi.com/search?q={{input}}"
`, aBookmark))
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Search.Primary.URL("a b"); got != "https://kagi.com/search?q=a+b" {
		t.Errorf("custom engine URL = %q", got)
	}
}

func TestLoad_CustomEngineOverridesBuiltin(t *testing.T) {
	cfg, err := Load(dirWith(t, `
[search.engines]
google = "https://example.com/q={{input}}"
`, aBookmark))
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Search.Primary.URL("z"); got != "https://example.com/q=z" {
		t.Errorf("override URL = %q, want the custom template", got)
	}
}

func TestLoad_UnknownEngineErrors(t *testing.T) {
	for _, field := range []string{"default_engine", "secondary_engine"} {
		_, err := Load(dirWith(t, "[search]\n"+field+" = \"nope\"\n", aBookmark))
		if err == nil || !strings.Contains(err.Error(), "nope") {
			t.Errorf("%s naming unknown engine: err = %v, want a message naming \"nope\"", field, err)
		}
	}
}

func TestLoad_CustomEngineMissingPlaceholderErrors(t *testing.T) {
	_, err := Load(dirWith(t, `
[search.engines]
bad = "https://example.com/search"
`, aBookmark))
	if err == nil || !strings.Contains(err.Error(), "{{input}}") {
		t.Errorf("err = %v, want a message about the missing {{input}}", err)
	}
}

func TestLoad_SPrefixedBookmarkKeyReserved(t *testing.T) {
	for _, key := range []string{"s", "sg", "S"} {
		_, err := Load(bookmarksDir(t, "[[bookmark]]\nkey = \""+key+"\"\nname = \"X\"\nurl = \"https://x.com\"\n"))
		if err == nil || !strings.Contains(err.Error(), "reserved") {
			t.Errorf("key %q: err = %v, want a reserved-for-search error", key, err)
		}
	}
}

func TestLoad_SPrefixedGroupKeyReserved(t *testing.T) {
	_, err := Load(dirWith(t, "[[group]]\nkey = \"s\"\nname = \"Stuff\"\n", `
[[bookmark]]
key = "g"
name = "X"
url = "https://x.com"
`))
	if err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Errorf("group key \"s\": err = %v, want a reserved-for-search error", err)
	}
}
