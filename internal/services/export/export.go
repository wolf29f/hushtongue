// Package export writes a dictionary to the JSON file imported by
// hushtongue v2.
package export

import (
	"encoding/json"
	"io"

	"github.com/wolf29f/hushtongue/internal/services/storage"
)

// Format identifies an export file and Version its layout, which v2 relies
// on to import it.
const (
	Format  = "hushtongue-export"
	Version = 1
)

// Dictionary is the part of storage.Storage the export lists the words and
// links from.
type Dictionary interface {
	ListWordDetails(language string) ([]storage.WordDetails, error)
	ListTranslationLinks() ([]storage.TranslationLink, error)
}

type file struct {
	Format       string        `json:"format"`
	Version      int           `json:"version"`
	Words        []word        `json:"words"`
	Translations []translation `json:"translations"`
}

// word ids only link the translations to their words within the file.
type word struct {
	ID   int    `json:"id"`
	Lang string `json:"lang"`
	Text string `json:"text"`
	Kind string `json:"kind"`
}

type translation struct {
	Source int    `json:"source"`
	Con    int    `json:"con"`
	Origin string `json:"origin"`
}

// Write writes every word and translation link of d to w.
func Write(w io.Writer, d Dictionary) error {
	f := file{
		Format:       Format,
		Version:      Version,
		Words:        make([]word, 0),
		Translations: make([]translation, 0),
	}

	for _, language := range []string{storage.LangSource, storage.LangCon} {
		words, err := d.ListWordDetails(language)
		if err != nil {
			return err
		}
		for _, details := range words {
			f.Words = append(f.Words, word{ID: details.ID, Lang: details.Language, Text: details.Text, Kind: details.Kind})
		}
	}

	links, err := d.ListTranslationLinks()
	if err != nil {
		return err
	}
	for _, link := range links {
		f.Translations = append(f.Translations, translation{Source: link.SourceID, Con: link.ConID, Origin: link.Origin})
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(f)
}
