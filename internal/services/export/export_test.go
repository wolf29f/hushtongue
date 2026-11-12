package export_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/wolf29f/hushtongue/internal/services/export"
	"github.com/wolf29f/hushtongue/internal/services/storage"
	"github.com/wolf29f/hushtongue/internal/services/storage/sqlite"
)

func TestWrite(t *testing.T) {
	t.Run("writes the words and translation links", func(t *testing.T) {
		dao := newTestDAO(t)
		if err := dao.AddWord(storage.LangSource, "mer"); err != nil {
			t.Fatalf("AddWord() failed: %v", err)
		}
		if err := dao.AddWord(storage.LangSource, "re"); err != nil {
			t.Fatalf("AddWord() failed: %v", err)
		}
		if _, err := dao.ChangeWordKind(2, storage.KindPrefix); err != nil {
			t.Fatalf("ChangeWordKind() failed: %v", err)
		}
		if err := dao.AddTranslationWord(1, "ailo"); err != nil {
			t.Fatalf("AddTranslationWord() failed: %v", err)
		}
		if err := dao.SaveGeneratedTranslation(2, []storage.TranslationPart{
			{Kind: storage.KindPrefix, SourceID: 2, Source: "re", Con: "ki"},
		}); err != nil {
			t.Fatalf("SaveGeneratedTranslation() failed: %v", err)
		}

		var got bytes.Buffer
		if err := export.Write(&got, dao); err != nil {
			t.Fatalf("Write() failed: %v", err)
		}

		want := `{
  "format": "hushtongue-export",
  "version": 1,
  "words": [
    {
      "id": 1,
      "lang": "source",
      "text": "mer",
      "kind": "root"
    },
    {
      "id": 2,
      "lang": "source",
      "text": "re",
      "kind": "prefix"
    },
    {
      "id": 3,
      "lang": "con",
      "text": "ailo",
      "kind": "root"
    },
    {
      "id": 4,
      "lang": "con",
      "text": "ki",
      "kind": "prefix"
    }
  ],
  "translations": [
    {
      "source": 1,
      "con": 3,
      "origin": "manual"
    },
    {
      "source": 2,
      "con": 4,
      "origin": "generated"
    }
  ]
}
`
		if got.String() != want {
			t.Errorf("Write() =\n%s\nwant\n%s", got.String(), want)
		}
	})

	t.Run("writes empty lists for an empty dictionary", func(t *testing.T) {
		var got bytes.Buffer
		if err := export.Write(&got, newTestDAO(t)); err != nil {
			t.Fatalf("Write() failed: %v", err)
		}

		want := `{
  "format": "hushtongue-export",
  "version": 1,
  "words": [],
  "translations": []
}
`
		if got.String() != want {
			t.Errorf("Write() =\n%s\nwant\n%s", got.String(), want)
		}
	})
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
