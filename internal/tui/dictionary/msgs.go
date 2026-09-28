package dictionary

import "github.com/wolf29f/hushtongue/internal/services/storage"

type switchLangMsg struct{}

type wordsLoadedMsg struct {
	words []storage.Word
}
