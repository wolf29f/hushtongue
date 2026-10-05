package generator

import (
	"hash/fnv"
	"strconv"
)

// Phonological rules: ancient, peaceful people on an isolated island.
// Open syllables (CV or V), soft sounds, no hard stop consonants
// (b/d/g/p deliberately excluded).
var (
	Consonants = []byte{'m', 'n', 'l', 'r', 's', 'h', 'v', 'k', 't'}
	Vowels     = []byte{'a', 'e', 'i', 'o', 'u'}
)

// Kind distinguishes the kind of word to generate, which constrains its
// shape: roots are full words (2-3 syllables), affixes are short (1
// syllable) so they stay light once attached to a root.
type Kind int

const (
	KindRoot Kind = iota
	KindPrefix
	KindSuffix
)

// hashSeed computes a stable 32-bit hash for a given string (FNV-1a,
// stdlib, no homemade implementation to maintain).
func hashSeed(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// splitmix32 is a minimal, deterministic PRNG: same seed → same sequence
// of values, on any machine and any Go version (unlike math/rand, whose
// cross-version reproducibility guarantees are weaker). Good enough here,
// this isn't crypto.
type splitmix32 struct{ state uint32 }

func newSplitmix32(seed uint32) *splitmix32 { return &splitmix32{state: seed} }

func (s *splitmix32) next() uint32 {
	s.state += 0x9e3779b9
	z := s.state
	z = (z ^ (z >> 16)) * 0x85ebca6b
	z = (z ^ (z >> 13)) * 0xc2b2ae35
	z = z ^ (z >> 16)
	return z
}

// intn returns a pseudo-random integer in [0, n) using the splitmix32 PRNG.
func (s *splitmix32) intn(n int) int {
	return int(s.next() % uint32(n))
}

// buildFromSeed builds a word from a seed, in a shape that depends on kind:
//   - root:   2-3 syllables, CV/V, same behavior as before
//   - prefix: 1 syllable, always ends on a vowel (attaches cleanly in
//     front of a root that starts with a consonant)
//   - suffix: 1 syllable, always starts on a consonant (attaches cleanly
//     behind a root that ends on a vowel — generated roots always end on
//     a vowel under the current rules)
//
// In every case: ~85% of syllables start with a consonant (forced for a
// suffix's first syllable), never two identical vowels in a row, never a
// consonant cluster.
func buildFromSeed(seed uint32, kind Kind) string {
	rng := newSplitmix32(seed)

	syllableCount := 2 + rng.intn(2) // 2 or 3, root default
	if kind == KindPrefix || kind == KindSuffix {
		syllableCount = 1
	}

	var word []byte
	var lastVowel byte
	for i := 0; i < syllableCount; i++ {
		startsWithConsonant := rng.intn(100) < 85
		if kind == KindSuffix && i == 0 {
			// first syllable of a suffix: always a consonant, this is
			// the attachment constraint, not a dice roll
			startsWithConsonant = true
		}
		if startsWithConsonant {
			word = append(word, Consonants[rng.intn(len(Consonants))])
		}
		var v byte
		for {
			v = Vowels[rng.intn(len(Vowels))]
			if v != lastVowel {
				break
			}
		}
		lastVowel = v
		word = append(word, v)
	}
	return string(word)
}

// Generate produces a word for the given source key and kind, deterministic
// and unique with respect to exists. On collision (exists returns true),
// the seed is re-derived from "key_1", "key_2", etc. — always
// deterministic, never a true random draw.
//
// exists should be scoped by kind by the caller (e.g. check only against
// other prefixes when generating a prefix), so a root can't accidentally
// collide with an unrelated affix.
func Generate(sourceKey string, kind Kind, exists func(word string) bool) string {
	var word string
	for attempt := range 50 {
		seedInput := sourceKey
		if attempt > 0 {
			seedInput = sourceKey + "_" + strconv.Itoa(attempt)
		}
		word = buildFromSeed(hashSeed(seedInput), kind)
		if !exists(word) {
			return word
		}
	}
	// Word space exhausted (shouldn't happen before several thousand
	// entries): return the last attempt rather than panicking.
	return word
}
