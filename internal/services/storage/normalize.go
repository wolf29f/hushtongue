package storage

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Normalize lowercases word and strips its diacritics ("Éléphant" -> "elephant").
func Normalize(word string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	stripped, _, err := transform.String(t, word)
	if err != nil {
		stripped = word
	}
	return strings.ToLower(stripped)
}
