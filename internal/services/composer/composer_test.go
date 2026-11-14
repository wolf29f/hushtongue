package composer_test

import (
	"context"
	"os"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wolf29f/hushtongue/internal/pgtest"
	"github.com/wolf29f/hushtongue/internal/services/composer"
	"github.com/wolf29f/hushtongue/internal/services/storage"
	"github.com/wolf29f/hushtongue/internal/services/storage/postgres"
)

func TestMain(m *testing.M) {
	os.Exit(pgtest.Run(m))
}

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
		for _, proposal := range proposals {
			if len(proposal.Homonyms) > 0 {
				t.Errorf("homonyms of %+v = %v, want none for the word's own translation", proposal.Parts, proposal.Homonyms)
			}
		}
	})

	t.Run("flags a compound whose concatenation translates another word", func(t *testing.T) {
		dao := newTestDAO(t)
		word := addWord(t, dao, "source", "remer", "root")
		re := addWord(t, dao, "source", "re", "prefix")
		mer := addWord(t, dao, "source", "mer", "root")
		link(t, dao, re, addWord(t, dao, "con", "ki", "prefix"))
		link(t, dao, mer, addWord(t, dao, "con", "ailo", "root"))
		link(t, dao, mer, addWord(t, dao, "con", "vora", "root"))
		link(t, dao, addWord(t, dao, "source", "fleur", "root"), addWord(t, dao, "con", "kiailo", "root"))

		proposals := propose(t, dao, word)

		for _, proposal := range proposals {
			var want []string
			if cons([]composer.Proposal{proposal})[0] == "ki-ailo" {
				want = []string{"fleur"}
			}
			if !slices.Equal(proposal.Homonyms, want) {
				t.Errorf("homonyms of %+v = %v, want %v", proposal.Parts, proposal.Homonyms, want)
			}
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
		service := composer.New(dao.store)

		proposal := propose(t, dao, word)[0]
		if err := service.Apply(context.Background(), dao.user, word, proposal); err != nil {
			t.Fatalf("Apply() failed: %v", err)
		}

		translations, err := dao.store.ListTranslations(context.Background(), dao.user, word)
		if err != nil {
			t.Fatalf("ListTranslations() failed: %v", err)
		}
		if want := "ki" + proposal.Parts[1].Con; len(translations) != 1 || translations[0].Word.Text != want {
			t.Errorf("translations = %v, want %q", translations, want)
		}
	})
}

func propose(t *testing.T, dao *testDB, wordID int) []composer.Proposal {
	t.Helper()
	proposals, err := composer.New(dao.store).Propose(context.Background(), dao.user, wordID)
	if err != nil {
		t.Fatalf("Propose() failed: %v", err)
	}
	return proposals
}

func sources(proposals []composer.Proposal) []string {
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

func cons(proposals []composer.Proposal) []string {
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

type testDB struct {
	store *postgres.Store
	pool  *pgxpool.Pool
	user  int
}

func newTestDAO(t *testing.T) *testDB {
	t.Helper()
	pool := pgtest.New(t)
	return &testDB{store: postgres.New(pool), pool: pool, user: pgtest.NewUser(t, pool)}
}

func addWord(t *testing.T, dao *testDB, lang, text, kind string) int {
	t.Helper()
	word, err := dao.store.AddWord(context.Background(), dao.user, lang, text)
	if err != nil {
		t.Fatalf("adding word %q: %v", text, err)
	}
	if kind != storage.KindRoot {
		if _, err := dao.store.ChangeWordKind(context.Background(), dao.user, word.ID, kind); err != nil {
			t.Fatalf("changing kind of %q: %v", text, err)
		}
	}
	return word.ID
}

func link(t *testing.T, dao *testDB, sourceID, conID int) {
	t.Helper()
	if _, err := dao.pool.Exec(context.Background(),
		`INSERT INTO translations (user_id, source_word_id, con_word_id) VALUES ($1, $2, $3)`,
		dao.user, sourceID, conID,
	); err != nil {
		t.Fatalf("inserting translation: %v", err)
	}
}
