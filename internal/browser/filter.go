package browser

import (
	"sort"
	"strings"
)

// Filter ranks case-insensitive substring and fuzzy subsequence matches in an
// already loaded directory. Whitespace-separated terms must all match.
func Filter(entries []Entry, query string) []Entry {
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return append([]Entry(nil), entries...)
	}
	type match struct {
		entry Entry
		score int
	}
	matches := []match{}
	for _, e := range entries {
		name := strings.ToLower(e.Name)
		total := 0
		ok := true
		for _, term := range terms {
			score, found := matchScore(name, term)
			if !found {
				ok = false
				break
			}
			total += score
		}
		if ok {
			matches = append(matches, match{e, total})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].score < matches[j].score })
	out := make([]Entry, len(matches))
	for i, m := range matches {
		out[i] = m.entry
	}
	return out
}
func matchScore(name, term string) (int, bool) {
	if i := strings.Index(name, term); i >= 0 {
		return i, true
	}
	source, want := []rune(name), []rune(term)
	at, gaps, last := 0, 0, -1
	for i, r := range source {
		if at < len(want) && r == want[at] {
			if last >= 0 {
				gaps += i - last - 1
			}
			last = i
			at++
		}
	}
	return 1000 + gaps, at == len(want)
}
