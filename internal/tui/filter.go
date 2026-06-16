package tui

import (
	"sort"
	"strings"

	"bml/internal/config"
)

// Result is one ranked search hit. NameMatch holds the rune indexes in the
// bookmark's Name that matched the query, for highlighting.
type Result struct {
	Bookmark  config.Bookmark
	NameMatch []int
	score     int
}

// Field weighting — a name hit outranks a tag hit, which outranks a url hit.
const (
	bonusName = 2000
	bonusTag  = 1000
	bonusURL  = 0
)

// Filter ranks bookmarks against a fuzzy query over name, url, and tags. An
// empty query returns every bookmark in original order (no highlight).
func Filter(bms []config.Bookmark, query string) []Result {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		out := make([]Result, len(bms))
		for i, b := range bms {
			out[i] = Result{Bookmark: b}
		}
		return out
	}

	var out []Result
	for _, b := range bms {
		nameIdx, nameScore, nameOK := subseq(q, strings.ToLower(b.Name))
		_, urlScore, urlOK := subseq(q, strings.ToLower(b.URL))
		tagScore, tagOK := bestTag(q, b.Tags)
		if !nameOK && !urlOK && !tagOK {
			continue
		}

		best := 0
		var nm []int
		if nameOK {
			best = bonusName + nameScore
			nm = nameIdx
		}
		if tagOK && bonusTag+tagScore > best {
			best = bonusTag + tagScore
			nm = nil
		}
		if urlOK && bonusURL+urlScore > best {
			best = bonusURL + urlScore
			nm = nil
		}
		out = append(out, Result{Bookmark: b, NameMatch: nm, score: best})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return out[i].Bookmark.Name < out[j].Bookmark.Name
	})
	return out
}

func bestTag(q string, tags []string) (int, bool) {
	best, ok := 0, false
	for _, t := range tags {
		if _, s, matched := subseq(q, strings.ToLower(t)); matched && (!ok || s > best) {
			best, ok = s, true
		}
	}
	return best, ok
}

// subseq greedily matches pattern as a subsequence of text, returning the
// matched rune indexes and a score that rewards consecutive and word-boundary
// hits. pattern is assumed lowercase; text should be too.
func subseq(pattern, text string) ([]int, int, bool) {
	pr := []rune(pattern)
	tr := []rune(text)
	if len(pr) == 0 {
		return nil, 0, true
	}

	idx := make([]int, 0, len(pr))
	score, pi, prev := 0, 0, -2
	for ti := 0; ti < len(tr) && pi < len(pr); ti++ {
		if tr[ti] != pr[pi] {
			continue
		}
		score++
		if ti == prev+1 {
			score += 5 // consecutive
		}
		if ti == 0 || isBoundary(tr[ti-1]) {
			score += 3 // start of a word
		}
		idx = append(idx, ti)
		prev = ti
		pi++
	}
	if pi != len(pr) {
		return nil, 0, false
	}
	return idx, score, true
}

func isBoundary(r rune) bool {
	switch r {
	case ' ', '/', '.', '-', '_', ':':
		return true
	}
	return false
}
