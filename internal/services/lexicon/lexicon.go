package lexicon

import (
	"cmp"
	"slices"
	"strings"
	"unicode"

	"github.com/wolf29f/hushtongue/internal/services/storage"
)

// Status is how well a typed word is known.
type Status int

const (
	// StatusUnknown: no root of the language has this normalized form.
	StatusUnknown Status = iota
	// StatusUntranslated: the root exists but has no translation yet.
	StatusUntranslated
	// StatusTranslated: the root has at least one translation.
	StatusTranslated
)

// Entry is what the lexicon knows about a typed word.
type Entry struct {
	Status       Status
	Word         storage.WordDetails   // zero value when unknown
	Translations []storage.Translation // insertion order
}

// Lexicon holds the root words of one language and their translations, read
// from storage once by Load: later changes to storage are not seen until the
// next Load. Lookup tells what is known of a typed word, Complete suggests
// roots for a prefix. The zero value is an empty lexicon.
type Lexicon struct {
	entries   map[string]Entry // by normalized form, for Lookup
	completer trigramIndex     // for Complete
}

// trigramIndex finds the roots that look like a prefix being typed. Every
// root is cut into trigrams (sequences of 3 runes, see trigrams); byTrigram
// lists, for each trigram, the roots containing it. To complete a prefix, its
// own trigrams are looked up in byTrigram, which gives the roots sharing at
// least one of them, and these are ranked by similarity with the prefix.
type trigramIndex struct {
	candidates []candidate
	byTrigram  map[string][]int // positions in candidates
}

type candidate struct {
	word         storage.Word
	normalized   string
	trigramCount int // number of distinct trigrams of normalized
}

// Reader is the part of storage a lexicon is loaded from.
type Reader interface {
	ListWordDetails(language string) ([]storage.WordDetails, error)
	ListTranslationsByLanguage(language string) (map[int][]storage.Translation, error)
}

// Load builds the snapshot of language from r.
func Load(r Reader, language string) (*Lexicon, error) {
	words, err := r.ListWordDetails(language)
	if err != nil {
		return nil, err
	}
	translations, err := r.ListTranslationsByLanguage(language)
	if err != nil {
		return nil, err
	}

	l := &Lexicon{
		entries:   make(map[string]Entry),
		completer: trigramIndex{byTrigram: make(map[string][]int)},
	}
	for _, word := range words {
		if word.Kind != storage.KindRoot {
			continue
		}
		entry := Entry{
			Status:       StatusUntranslated,
			Word:         word,
			Translations: translations[word.ID],
		}
		if len(entry.Translations) > 0 {
			entry.Status = StatusTranslated
		}
		l.entries[word.Normalized] = entry
		l.completer.add(storage.Word{ID: word.ID, Text: word.Text}, word.Normalized)
	}
	return l, nil
}

// Lookup returns the entry of word, matched on its normalized form, leading
// and trailing punctuation ignored.
func (l *Lexicon) Lookup(word string) Entry {
	return l.entries[normalizeTyped(word)]
}

// Complete returns up to limit words close to prefix, best first, and
// whether more were left out.
func (l *Lexicon) Complete(prefix string, limit int) (words []storage.Word, more bool) {
	return l.completer.complete(normalizeTyped(prefix), limit)
}

func (t *trigramIndex) add(word storage.Word, normalized string) {
	grams := trigrams(normalized)
	for _, gram := range grams {
		t.byTrigram[gram] = append(t.byTrigram[gram], len(t.candidates))
	}
	t.candidates = append(t.candidates, candidate{
		word:         word,
		normalized:   normalized,
		trigramCount: len(grams),
	})
}

func (t *trigramIndex) complete(prefix string, limit int) (words []storage.Word, more bool) {
	grams := trigrams(prefix)
	if len(grams) == 0 {
		return nil, false
	}

	sharedGrams := make(map[int]int) // by position in candidates
	for _, gram := range grams {
		for _, i := range t.byTrigram[gram] {
			sharedGrams[i]++
		}
	}

	// similarity is the Jaccard index of the two sets of trigrams: shared
	// trigrams over distinct trigrams of both, 1 when the sets are equal. On
	// a tie, the shorter root comes first, then the alphabetical order.
	type match struct {
		candidate
		similarity float64
	}
	matches := make([]match, 0, len(sharedGrams))
	for i, shared := range sharedGrams {
		c := t.candidates[i]
		union := len(grams) + c.trigramCount - shared
		matches = append(matches, match{
			candidate:  c,
			similarity: float64(shared) / float64(union),
		})
	}
	slices.SortFunc(matches, func(a, b match) int {
		return cmp.Or(
			cmp.Compare(b.similarity, a.similarity),
			cmp.Compare(len(a.normalized), len(b.normalized)),
			cmp.Compare(a.normalized, b.normalized),
			cmp.Compare(a.word.ID, b.word.ID),
		)
	})

	if len(matches) > limit {
		matches, more = matches[:limit], true
	}
	words = make([]storage.Word, len(matches))
	for i, m := range matches {
		words[i] = m.word
	}
	return words, more
}

// Clean strips the leading and trailing punctuation of word ("maison," ->
// "maison").
func Clean(word string) string {
	return strings.TrimFunc(word, unicode.IsPunct)
}

func normalizeTyped(word string) string {
	return storage.Normalize(Clean(word))
}

// trigrams lists the distinct trigrams of word, padded at the start only so
// that a word being typed scores well against the words it begins.
func trigrams(word string) []string {
	if word == "" {
		return nil
	}
	runes := []rune("  " + word)
	grams := make([]string, 0, len(runes)-2)
	for i := 0; i+3 <= len(runes); i++ {
		gram := string(runes[i : i+3])
		if !slices.Contains(grams, gram) {
			grams = append(grams, gram)
		}
	}
	return grams
}
