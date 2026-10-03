package translationpicker

import (
	"github.com/wolf29f/hushtongue/internal/services/storage"
)

type generatePressedMsg struct{}

type wordLoadedMsg struct {
	word storage.WordDetails
}

type wordsLoadedMsg struct {
	words []storage.Word
}
