package generator_test

import (
	"errors"
	"slices"
	"strconv"
	"testing"

	"github.com/wolf29f/hushtongue/internal/services/generator"
)

func free(string) bool { return false }

// TestGenerateGolden pins the generated words: a change here changes the
// translation generated for every source word not stored yet.
func TestGenerateGolden(t *testing.T) {
	tests := []struct {
		kind  generator.Kind
		words map[string]string
	}{
		{generator.KindRoot, map[string]string{
			"mer": "somu", "fleur": "tuko", "soleil": "nali", "lune": "oti", "maison": "nuto",
			"arbre": "saku", "feu": "etue", "eau": "kave", "chemin": "aru", "etoile": "mame",
		}},
		{generator.KindPrefix, map[string]string{
			"re": "ri", "de": "me", "pre": "re", "anti": "ne", "sur": "sa",
			"sous": "mo", "in": "lo", "co": "vo", "dis": "mote", "mal": "ki",
		}},
		{generator.KindSuffix, map[string]string{
			"ette": "setu", "eur": "nuhe", "ment": "runi", "tion": "ti", "able": "ma",
			"ier": "ha", "ise": "tena", "age": "mula", "ure": "se", "on": "ko",
		}},
	}
	for _, tt := range tests {
		for source, want := range tt.words {
			got, err := generator.Generate(source, tt.kind, free)
			if err != nil || got != want {
				t.Errorf("Generate(%q, %d) = %q, %v, want %q", source, tt.kind, got, err, want)
			}
		}
	}
}

func TestGenerateAvoidsTakenWords(t *testing.T) {
	first, err := generator.Generate("mer", generator.KindRoot, free)
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	taken := func(word string) bool { return word == first }
	got, err := generator.Generate("mer", generator.KindRoot, taken)
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}
	if got == first {
		t.Errorf("Generate() = %q, which is taken", got)
	}
	again, _ := generator.Generate("mer", generator.KindRoot, taken)
	if again != got {
		t.Errorf("Generate() = %q then %q, want the same variant", got, again)
	}
}

func TestGenerateExhausted(t *testing.T) {
	_, err := generator.Generate("mer", generator.KindPrefix, func(string) bool { return true })
	if !errors.Is(err, generator.ErrExhausted) {
		t.Errorf("Generate() error = %v, want ErrExhausted", err)
	}
}

func TestGenerateShapes(t *testing.T) {
	isVowel := func(c byte) bool { return slices.Contains(generator.Vowels, c) }
	for i := range 1000 {
		source := "w" + strconv.Itoa(i)
		prefix, _ := generator.Generate(source, generator.KindPrefix, free)
		if !isVowel(prefix[len(prefix)-1]) {
			t.Errorf("prefix %q doesn't end on a vowel", prefix)
		}
		suffix, _ := generator.Generate(source, generator.KindSuffix, free)
		if isVowel(suffix[0]) {
			t.Errorf("suffix %q doesn't start on a consonant", suffix)
		}
		root, _ := generator.Generate(source, generator.KindRoot, free)
		for _, word := range []string{prefix, suffix, root} {
			for j := 1; j < len(word); j++ {
				if word[j] == word[j-1] || !isVowel(word[j]) && !isVowel(word[j-1]) {
					t.Errorf("%q has a repeated letter or a consonant cluster", word)
					break
				}
			}
		}
	}
}
