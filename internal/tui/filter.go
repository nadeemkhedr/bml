package tui

import (
	"sort"
	"strings"

	"bml/internal/config"
)

// Result is one ranked search hit. NameMatch holds the rune indexes in the
// bookmark's Name that matched the query; TagMatch is aligned to Bookmark.Tags,
// with each entry holding the matched rune indexes in that tag (nil if the tag
// didn't match). Both drive highlighting.
type Result struct {
	Bookmark  config.Bookmark
	NameMatch []int
	TagMatch  [][]int
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

		// Per-tag match positions (and the best tag score for ranking).
		var tagMatch [][]int
		tagScore, tagOK := 0, false
		if len(b.Tags) > 0 {
			tagMatch = make([][]int, len(b.Tags))
			for i, t := range b.Tags {
				if idx, s, ok := subseq(q, strings.ToLower(t)); ok {
					tagMatch[i] = idx
					if !tagOK || s > tagScore {
						tagScore = s
					}
					tagOK = true
				}
			}
		}
		if !nameOK && !urlOK && !tagOK {
			continue
		}

		// Rank by the best-scoring field, but highlight every field that matched.
		best := 0
		if nameOK && bonusName+nameScore > best {
			best = bonusName + nameScore
		}
		if tagOK && bonusTag+tagScore > best {
			best = bonusTag + tagScore
		}
		if urlOK && bonusURL+urlScore > best {
			best = bonusURL + urlScore
		}

		var nm []int
		if nameOK {
			nm = nameIdx
		}
		out = append(out, Result{Bookmark: b, NameMatch: nm, TagMatch: tagMatch, score: best})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return out[i].Bookmark.Name < out[j].Bookmark.Name
	})
	return out
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
