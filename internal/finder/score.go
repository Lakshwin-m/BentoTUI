package finder

import (
	"path"
	"strings"
	"unicode"
)

func isBoundary(r rune) bool {
	switch r {
	case '/', '\\', '_', '-', '.', ' ':
		return true
	}
	return false
}

// scorePath scores a subsequence match of query against candidate (a relative
// slash-separated path). It returns the score and matched rune indices.
func scorePath(candidate, query string) (int, []int, bool) {
	if query == "" {
		return 0, nil, true
	}
	cr := []rune(candidate)
	qr := []rune(query)
	lowerC := lowerRunes(cr)
	lowerQ := lowerRunes(qr)
	if len(lowerQ) > len(lowerC) {
		return 0, nil, false
	}

	indices := make([]int, 0, len(qr))
	score := 0
	qi := 0
	prev := -2
	for i := 0; i < len(lowerC) && qi < len(lowerQ); i++ {
		if lowerC[i] != lowerQ[qi] {
			continue
		}
		indices = append(indices, i)
		score++ // base per matched character
		if i == 0 {
			score += 12
		} else if isBoundary(cr[i-1]) {
			score += 10
		}
		if i == prev+1 {
			score += 6
		}
		if qi == 0 && i > 0 && cr[i-1] == '/' {
			score += 8
		}
		prev = i
		qi++
	}
	if qi < len(lowerQ) {
		return 0, nil, false
	}

	base := path.Base(candidate)
	if len(qr) <= len([]rune(base)) && strings.HasPrefix(strings.ToLower(base), strings.ToLower(query)) {
		score += 20
	}
	// Mild preference for shorter paths.
	score += max(0, 30-len(cr)/4)
	return score, indices, true
}

func lowerRunes(r []rune) []rune {
	out := make([]rune, len(r))
	for i, c := range r {
		out[i] = unicode.ToLower(c)
	}
	return out
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
