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
	StatusUnknown Status = iota
	StatusUntranslated
	StatusTranslated
)

// Entry is what the lexicon knows about a typed word.
type Entry struct {
	Status       Status
	Word         storage.WordDetails   // zero value when unknown
	Translations []storage.Translation // insertion order
}

type indexed struct {
	word       storage.Word
	normalized string
	trigrams   int
}

// Lexicon is an in-memory snapshot of the roots of one language, their
// translations and a trigram index over them.
type Lexicon struct {
	entries map[string]Entry
	words   []indexed
	index   map[string][]int
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
		entries: make(map[string]Entry),
		index:   make(map[string][]int),
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

		grams := trigrams(word.Normalized)
		for _, gram := range grams {
			l.index[gram] = append(l.index[gram], len(l.words))
		}
		l.words = append(l.words, indexed{
			word:       storage.Word{ID: word.ID, Text: word.Text},
			normalized: word.Normalized,
			trigrams:   len(grams),
		})
	}
	return l, nil
}

// Lookup returns the entry of word, matched on its normalized form, leading
// and trailing punctuation ignored.
func (l *Lexicon) Lookup(word string) Entry {
	return l.entries[key(word)]
}

// Complete returns up to limit words close to prefix, best first, and
// whether more were left out.
func (l *Lexicon) Complete(prefix string, limit int) (words []storage.Word, more bool) {
	grams := trigrams(key(prefix))
	if len(grams) == 0 {
		return nil, false
	}

	shared := make(map[int]int)
	for _, gram := range grams {
		for _, i := range l.index[gram] {
			shared[i]++
		}
	}

	type match struct {
		indexed
		score float64
	}
	matches := make([]match, 0, len(shared))
	for i, count := range shared {
		w := l.words[i]
		matches = append(matches, match{
			indexed: w,
			score:   float64(count) / float64(len(grams)+w.trigrams-count),
		})
	}
	slices.SortFunc(matches, func(a, b match) int {
		return cmp.Or(
			cmp.Compare(b.score, a.score),
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

func key(word string) string {
	return storage.Normalize(strings.TrimFunc(word, unicode.IsPunct))
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
