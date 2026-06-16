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

// Match tiers, best first. A query that begins a field beats one that begins a
// word inside it, which beats a mid-word substring, which beats a scattered
// (subsequence) match.
const (
	tierScatter = 1 // chars present but not contiguous
	tierSubstr  = 2 // contiguous substring, mid-word
	tierWord    = 3 // begins a word (word-boundary), not the field start
	tierPrefix  = 4 // field starts with the query
)

// Field weighting within a tier — a name hit outranks a tag hit, which outranks
// a url hit.
const (
	bonusName = 3000
	bonusTag  = 2000
	bonusURL  = 1000
)

// Filter ranks bookmarks against a query over name, url, and tags. An empty
// query returns every bookmark in original order (no highlight). Results are
// ordered by match tier first (prefix > word > substring > scatter), then by
// field (name > tag > url), then by an earlier/shorter-match bonus, then name.
func Filter(bms []config.Bookmark, query string) []Result {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		out := make([]Result, len(bms))
		for i, b := range bms {
			out[i] = Result{Bookmark: b}
		}
		return out
	}
	qr := []rune(q)

	var out []Result
	for _, b := range bms {
		nameR := []rune(strings.ToLower(b.Name))
		nameTier, nameIdx, nameOK := fieldMatch(qr, nameR)

		urlR := []rune(strings.ToLower(b.URL))
		urlTier, _, urlOK := fieldMatch(qr, urlR)

		// Per-tag match positions, plus the best tag tier for ranking.
		var tagMatch [][]int
		tagTier, tagPos, tagLen, tagOK := 0, 0, 0, false
		if len(b.Tags) > 0 {
			tagMatch = make([][]int, len(b.Tags))
			for i, t := range b.Tags {
				tr := []rune(strings.ToLower(t))
				tier, idx, ok := fieldMatch(qr, tr)
				if !ok {
					continue
				}
				tagMatch[i] = idx
				tagOK = true
				if tier > tagTier {
					tagTier, tagPos, tagLen = tier, idx[0], len(tr)
				}
			}
		}
		if !nameOK && !urlOK && !tagOK {
			continue
		}

		// Pick the winning field by tier; on ties, name beats tag beats url.
		bestTier, fieldBonus, winPos, winLen := 0, 0, 0, 0
		if nameOK && nameTier > bestTier {
			bestTier, fieldBonus, winPos, winLen = nameTier, bonusName, nameIdx[0], len(nameR)
		}
		if tagOK && tagTier > bestTier {
			bestTier, fieldBonus, winPos, winLen = tagTier, bonusTag, tagPos, tagLen
		}
		if urlOK && urlTier > bestTier {
			bestTier, fieldBonus, winPos, winLen = urlTier, bonusURL, 0, len(urlR)
		}

		// Within tier+field: earlier and shorter matches rank a little higher.
		fine := 900 - winPos*10 - winLen
		if fine < 0 {
			fine = 0
		}
		score := bestTier*10000 + fieldBonus + fine

		var nm []int
		if nameOK {
			nm = nameIdx
		}
		out = append(out, Result{Bookmark: b, NameMatch: nm, TagMatch: tagMatch, score: score})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return out[i].Bookmark.Name < out[j].Bookmark.Name
	})
	return out
}

// fieldMatch classifies how query qr matches text tr and returns the match tier,
// the matched rune indexes (for highlighting), and whether it matched at all.
func fieldMatch(qr, tr []rune) (int, []int, bool) {
	if len(qr) == 0 {
		return 0, nil, false
	}

	// Find the first contiguous occurrence and the first word-boundary one.
	firstPos, wordPos := -1, -1
	for i := 0; i+len(qr) <= len(tr); i++ {
		if !runesEqual(tr[i:i+len(qr)], qr) {
			continue
		}
		if firstPos < 0 {
			firstPos = i
		}
		if (i == 0 || isBoundary(tr[i-1])) && wordPos < 0 {
			wordPos = i
		}
		if firstPos >= 0 && wordPos >= 0 {
			break
		}
	}

	switch {
	case firstPos == 0:
		return tierPrefix, seqIdx(0, len(qr)), true
	case wordPos > 0:
		return tierWord, seqIdx(wordPos, len(qr)), true
	case firstPos > 0:
		return tierSubstr, seqIdx(firstPos, len(qr)), true
	}

	if idx, ok := subseqRunes(qr, tr); ok {
		return tierScatter, idx, true
	}
	return 0, nil, false
}

// subseqRunes reports whether qr is a subsequence of tr, returning the matched
// rune indexes.
func subseqRunes(qr, tr []rune) ([]int, bool) {
	idx := make([]int, 0, len(qr))
	pi := 0
	for ti := 0; ti < len(tr) && pi < len(qr); ti++ {
		if tr[ti] == qr[pi] {
			idx = append(idx, ti)
			pi++
		}
	}
	if pi != len(qr) {
		return nil, false
	}
	return idx, true
}

func runesEqual(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// seqIdx returns the run of n indexes starting at start.
func seqIdx(start, n int) []int {
	idx := make([]int, n)
	for i := range idx {
		idx[i] = start + i
	}
	return idx
}

func isBoundary(r rune) bool {
	switch r {
	case ' ', '/', '.', '-', '_', ':':
		return true
	}
	return false
}
