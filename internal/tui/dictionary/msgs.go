package dictionary

import (
	tea "charm.land/bubbletea/v2"
	"github.com/wolf29f/hushtongue/internal/services/storage"
)

type switchLangMsg struct{}

type newWordMsg struct {
	word string
}

func newWord(word string) tea.Cmd {
	return func() tea.Msg {
		return newWordMsg{word: word}
	}
}

type wordsLoadedMsg struct {
	words []storage.Word
}
