package dictionary

import (
	tea "charm.land/bubbletea/v2"
	"github.com/wolf29f/hushtongue/internal/services/storage"
)

type switchLangPressedMsg struct{}

type wordSubmittedMsg struct {
	word string
}

func wordSubmitted(word string) tea.Cmd {
	return func() tea.Msg {
		return wordSubmittedMsg{word: word}
	}
}

type wordsLoadedMsg struct {
	words []storage.Word
}
