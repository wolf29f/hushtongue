package translation_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/wolf29f/hushtongue/internal/services/storage"
	"github.com/wolf29f/hushtongue/internal/services/storage/sqlite"
	"github.com/wolf29f/hushtongue/internal/services/translation"
)

func TestPropose(t *testing.T) {
	t.Run("decomposes on known affixes and keeps the user's text", func(t *testing.T) {
		dao := newTestDAO(t)
		word := addWord(t, dao, "source", "Refaîte", "root")
		re := addWord(t, dao, "source", "re", "prefix")
		ki := addWord(t, dao, "con", "ki", "prefix")
		link(t, dao, re, ki)

		proposals := propose(t, dao, word)

		if got, want := sources(proposals), []string{"re-faîte", "Refaîte"}; !slices.Equal(got, want) {
			t.Fatalf("sources = %v, want %v", got, want)
		}
		compound := proposals[0].Parts
		if compound[0].ConID != ki || compound[0].Con != "ki" {
			t.Errorf("prefix part = %+v, want the existing translation ki", compound[0])
		}
		if compound[1].SourceID != 0 || compound[1].ConID != 0 || compound[1].Con == "" {
			t.Errorf("root part = %+v, want an unknown root with a generated translation", compound[1])
		}
		pure := proposals[1].Parts[0]
		if pure.SourceID != word || pure.ConID != 0 || pure.Con == "" {
			t.Errorf("pure generation = %+v, want the word with a generated translation", pure)
		}
	})

	t.Run("lists the complete translations first, one per combination", func(t *testing.T) {
		dao := newTestDAO(t)
		word := addWord(t, dao, "source", "remer", "root")
		re := addWord(t, dao, "source", "re", "prefix")
		mer := addWord(t, dao, "source", "mer", "root")
		link(t, dao, re, addWord(t, dao, "con", "ki", "prefix"))
		link(t, dao, mer, addWord(t, dao, "con", "ailo", "root"))
		link(t, dao, mer, addWord(t, dao, "con", "vora", "root"))
		link(t, dao, word, addWord(t, dao, "con", "kiailo", "root"))

		proposals := propose(t, dao, word)

		want := []string{"kiailo", "ki-ailo", "ki-vora"}
		if got := cons(proposals); !slices.Equal(got, want) {
			t.Errorf("cons = %v, want %v", got, want)
		}
	})

	t.Run("stacks affixes and keeps roots long enough", func(t *testing.T) {
		dao := newTestDAO(t)
		word := addWord(t, dao, "source", "redéfaire", "root")
		addWord(t, dao, "source", "re", "prefix")
		addWord(t, dao, "source", "dé", "prefix")
		addWord(t, dao, "source", "faire", "prefix")
		addWord(t, dao, "source", "ire", "suffix")

		proposals := propose(t, dao, word)

		want := []string{"re-dé-fa-ire", "re-dé-faire", "re-défa-ire", "re-défaire", "redéfa-ire", "redéfaire"}
		got := sources(proposals)
		slices.Sort(got[:len(got)-1])
		if !slices.Equal(got, want) {
			t.Errorf("sources = %v, want %v", got, want)
		}
	})

	t.Run("generates the same word for a section, distinct words otherwise", func(t *testing.T) {
		dao := newTestDAO(t)
		word := addWord(t, dao, "source", "remer", "root")
		addWord(t, dao, "source", "re", "prefix")
		addWord(t, dao, "source", "er", "suffix")

		proposals := propose(t, dao, word)

		generated := make(map[string]string)
		for _, proposal := range proposals {
			for _, part := range proposal.Parts {
				key := part.Kind + "/" + part.Source
				if con, ok := generated[key]; ok && con != part.Con {
					t.Errorf("%s generated as %q and %q", key, con, part.Con)
				}
				generated[key] = part.Con
			}
		}
		seen := make(map[string]string)
		for key, con := range generated {
			if other, ok := seen[con]; ok {
				t.Errorf("%s and %s both generated as %q", key, other, con)
			}
			seen[con] = key
		}
	})

	t.Run("doesn't decompose an affix", func(t *testing.T) {
		dao := newTestDAO(t)
		word := addWord(t, dao, "source", "remer", "prefix")
		addWord(t, dao, "source", "re", "prefix")

		proposals := propose(t, dao, word)

		if len(proposals) != 1 || len(proposals[0].Parts) != 1 {
			t.Fatalf("proposals = %+v, want a single part", proposals)
		}
		part := proposals[0].Parts[0]
		if part.SourceID != word || part.Kind != "prefix" || part.ConID != 0 {
			t.Errorf("part = %+v, want the generated prefix", part)
		}
	})
}

func TestApply(t *testing.T) {
	t.Run("stores the proposal", func(t *testing.T) {
		dao := newTestDAO(t)
		word := addWord(t, dao, "source", "Refaîte", "root")
		re := addWord(t, dao, "source", "re", "prefix")
		link(t, dao, re, addWord(t, dao, "con", "ki", "prefix"))
		service := translation.New(dao)

		proposal := propose(t, dao, word)[0]
		if err := service.Apply(word, proposal); err != nil {
			t.Fatalf("Apply() failed: %v", err)
		}

		translations, err := dao.ListTranslations(word)
		if err != nil {
			t.Fatalf("ListTranslations() failed: %v", err)
		}
		if want := "ki" + proposal.Parts[1].Con; len(translations) != 1 || translations[0].Word.Text != want {
			t.Errorf("translations = %v, want %q", translations, want)
		}
	})
}

func propose(t *testing.T, dao *sqlite.DAO, wordID int) []translation.Proposal {
	t.Helper()
	proposals, err := translation.New(dao).Propose(wordID)
	if err != nil {
		t.Fatalf("Propose() failed: %v", err)
	}
	return proposals
}

func sources(proposals []translation.Proposal) []string {
	got := make([]string, len(proposals))
	for i, proposal := range proposals {
		for j, part := range proposal.Parts {
			if j > 0 {
				got[i] += "-"
			}
			got[i] += part.Source
		}
	}
	return got
}

func cons(proposals []translation.Proposal) []string {
	got := make([]string, len(proposals))
	for i, proposal := range proposals {
		for j, part := range proposal.Parts {
			if j > 0 {
				got[i] += "-"
			}
			got[i] += part.Con
		}
	}
	return got
}

func newTestDAO(t *testing.T) *sqlite.DAO {
	t.Helper()
	dao, err := sqlite.Load(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	t.Cleanup(func() {
		if err := dao.Close(); err != nil {
			t.Errorf("Close() failed: %v", err)
		}
	})
	if err := dao.Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	return dao
}

func addWord(t *testing.T, dao *sqlite.DAO, lang, text, kind string) int {
	t.Helper()
	if err := dao.AddWord(lang, text); err != nil {
		t.Fatalf("adding word %q: %v", text, err)
	}
	var id int
	if err := dao.DB.QueryRow(`SELECT MAX(id) FROM words`).Scan(&id); err != nil {
		t.Fatalf("reading word id: %v", err)
	}
	if kind != storage.KindRoot {
		if _, err := dao.ChangeWordKind(id, kind); err != nil {
			t.Fatalf("changing kind of %q: %v", text, err)
		}
	}
	return id
}

func link(t *testing.T, dao *sqlite.DAO, sourceID, conID int) {
	t.Helper()
	if _, err := dao.DB.Exec(
		`INSERT INTO translations (source_word_id, con_word_id) VALUES (?, ?)`,
		sourceID, conID,
	); err != nil {
		t.Fatalf("inserting translation: %v", err)
	}
}
