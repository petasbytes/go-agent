package metrics

import (
	"strings"
	"unicode/utf8"
)

// Features holds basic local text features derived from an input string.
type Features struct {
	Bytes int
	Runes int
	Words int
	Lines int
}

// CountFeatures computes and returns byte, rune, word, and line counts for the input string.
func CountFeatures(s string) Features {
	b := len(s)
	r := utf8.RuneCountInString(s)
	w := countWords(s)
	l := countLines(s)
	return Features{Bytes: b, Runes: r, Words: w, Lines: l}
}

// countWords counts words split on Unicode whitespace.
func countWords(s string) int {
	return len(strings.Fields(s))
}

// countLines returns 0 for empty strings; otherwise 1 plus the number of '\n' runes.
func countLines(s string) int {
	if s == "" {
		return 0
	}
	return 1 + strings.Count(s, "\n")
}
