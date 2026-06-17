package config

import (
	"fmt"
	"net/url"
	"strings"
)

// enginePlaceholder is substituted with the URL-escaped query when an engine's
// template is turned into a concrete search URL.
const enginePlaceholder = "{{input}}"

// Engine is a named search-engine URL template containing an {{input}}
// placeholder (e.g. "google" → "https://www.google.com/search?q={{input}}").
// It is the single configurable unit of search mode: every search action is just
// "fill this engine's template with the query and act on the result".
type Engine struct {
	Name     string
	Template string
}

// URL fills the engine's template with the URL-escaped query (spaces become "+",
// special characters are percent-encoded).
func (e Engine) URL(query string) string {
	return strings.ReplaceAll(e.Template, enginePlaceholder, url.QueryEscape(query))
}

// builtinEngines are the search engines shipped with bml. There is deliberately
// no per-engine "lucky" variant — duckduckgo_lucky is simply its own engine,
// because no reliable URL-template lucky exists for Google (see ADR 0003).
func builtinEngines() map[string]string {
	return map[string]string{
		"google":           "https://www.google.com/search?q={{input}}",
		"duckduckgo":       "https://duckduckgo.com/?q={{input}}",
		"duckduckgo_lucky": "https://duckduckgo.com/?q=!ducky+{{input}}",
	}
}

const (
	defaultPrimaryEngine   = "google"
	defaultSecondaryEngine = "duckduckgo_lucky"
)

// Search is the resolved search-mode configuration: the engines bound to the
// primary (Enter) and secondary (Tab) actions.
type Search struct {
	Primary   Engine // Enter
	Secondary Engine // Tab
}

// tomlSearch mirrors the [search] table in config.toml: which engines back the
// primary (Enter) and secondary (Tab) actions, plus any user-defined engines
// keyed by name.
type tomlSearch struct {
	DefaultEngine   string            `toml:"default_engine,omitempty"`
	SecondaryEngine string            `toml:"secondary_engine,omitempty"`
	Engines         map[string]string `toml:"engines,omitempty"`
}

// DefaultSearch returns the built-in search configuration (Google primary,
// duckduckgo_lucky secondary), used when no [search] table is present.
func DefaultSearch() Search {
	s, _ := resolveSearch(nil) // built-in defaults can't fail
	return s
}

// resolveSearch builds the Search configuration from the optional [search] table.
// User-defined engines merge over the built-ins (a same-name entry wins). It
// hard-errors on a custom template missing {{input}} or a primary/secondary slot
// naming an engine that doesn't exist.
func resolveSearch(s *tomlSearch) (Search, error) {
	engines := builtinEngines()
	primaryName, secondaryName := defaultPrimaryEngine, defaultSecondaryEngine

	if s != nil {
		for name, tmpl := range s.Engines {
			if !strings.Contains(tmpl, enginePlaceholder) {
				return Search{}, fmt.Errorf("search engine %q: template %q must contain %s", name, tmpl, enginePlaceholder)
			}
			engines[name] = tmpl // user engine overrides a built-in of the same name
		}
		if s.DefaultEngine != "" {
			primaryName = s.DefaultEngine
		}
		if s.SecondaryEngine != "" {
			secondaryName = s.SecondaryEngine
		}
	}

	primary, ok := engines[primaryName]
	if !ok {
		return Search{}, fmt.Errorf("default_engine %q is not a known search engine", primaryName)
	}
	secondary, ok := engines[secondaryName]
	if !ok {
		return Search{}, fmt.Errorf("secondary_engine %q is not a known search engine", secondaryName)
	}
	return Search{
		Primary:   Engine{Name: primaryName, Template: primary},
		Secondary: Engine{Name: secondaryName, Template: secondary},
	}, nil
}
