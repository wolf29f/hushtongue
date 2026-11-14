package postgres_test

import (
	"errors"
	"testing"

	"github.com/wolf29f/hushtongue/internal/services/storage"
)

// TestIsolation acts as another user on the words and links of f.user: each
// call must report them as missing and leave them untouched.
func TestIsolation(t *testing.T) {
	f := newFixture(t)
	mer := f.word(t, "source", "mer", "root")
	ailo := f.word(t, "con", "ailo", "root")
	merAilo := f.translation(t, mer, ailo)

	other := f.otherUser(t)
	ciel := other.word(t, "source", "ciel", "root")
	vora := other.word(t, "con", "vora", "root")

	tests := map[string]func() error{
		"GetWord": func() error {
			_, err := other.store.GetWord(ctx, other.user, mer)
			return err
		},
		"SaveWord": func() error {
			_, err := other.store.SaveWord(ctx, other.user, storage.WordDetails{ID: mer, Language: "source", Text: "terre", Kind: "root"})
			return err
		},
		"MergeWords from": func() error {
			return other.store.MergeWords(ctx, other.user, mer, ciel)
		},
		"MergeWords into": func() error {
			return other.store.MergeWords(ctx, other.user, ciel, mer)
		},
		"DeleteWord": func() error {
			return other.store.DeleteWord(ctx, other.user, mer)
		},
		"ChangeWordKind": func() error {
			_, err := other.store.ChangeWordKind(ctx, other.user, mer, "prefix")
			return err
		},
		"ListTranslations": func() error {
			_, err := other.store.ListTranslations(ctx, other.user, mer)
			return err
		},
		"DeleteTranslation": func() error {
			return other.store.DeleteTranslation(ctx, other.user, merAilo)
		},
		"AddTranslation from": func() error {
			return other.store.AddTranslation(ctx, other.user, mer, vora)
		},
		"AddTranslation to": func() error {
			return other.store.AddTranslation(ctx, other.user, ciel, ailo)
		},
		"AddTranslationWord": func() error {
			return other.store.AddTranslationWord(ctx, other.user, mer, "terra")
		},
		"SaveGeneratedTranslation word": func() error {
			return other.store.SaveGeneratedTranslation(ctx, other.user, mer, []storage.TranslationPart{
				{Kind: "root", Source: "mer", Con: "kalo"},
			})
		},
		"SaveGeneratedTranslation linked parts": func() error {
			return other.store.SaveGeneratedTranslation(ctx, other.user, ciel, []storage.TranslationPart{
				{Kind: "root", SourceID: mer, Source: "mer", ConID: ailo, Con: "ailo"},
			})
		},
		"SaveGeneratedTranslation part": func() error {
			return other.store.SaveGeneratedTranslation(ctx, other.user, ciel, []storage.TranslationPart{
				{Kind: "root", SourceID: ciel, Source: "ciel", ConID: ailo, Con: "ailo"},
			})
		},
	}
	for name, call := range tests {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, storage.ErrNotFound) {
				t.Errorf("error = %v, want ErrNotFound", err)
			}

			word, err := f.store.GetWord(ctx, f.user, mer)
			if err != nil || word.Text != "mer" || word.Kind != "root" {
				t.Errorf("GetWord(mer) = %+v, %v, want it unchanged", word, err)
			}
			f.assertTranslations(t, mer, "ailo")
			other.assertTranslations(t, ciel)
		})
	}

	t.Run("lists only the user's words and links", func(t *testing.T) {
		words, err := other.store.ListWordDetails(ctx, other.user, storage.LangSource)
		if err != nil || len(words) != 1 || words[0].ID != ciel {
			t.Errorf("ListWordDetails() = %v, %v, want ciel only", words, err)
		}
		translations, err := other.store.ListTranslationsByLanguage(ctx, other.user, storage.LangSource)
		if err != nil || len(translations) != 0 {
			t.Errorf("ListTranslationsByLanguage() = %v, %v, want none", translations, err)
		}
	})
}
