package translationpicker

import (
	tea "charm.land/bubbletea/v2"
	"github.com/wolf29f/hushtongue/internal/services/storage"
)

type generatePressedMsg struct{}

type newWordPressedMsg struct{}

type newWordSubmittedMsg struct {
	text string
}

func newWordSubmitted(text string) tea.Cmd {
	return func() tea.Msg {
		return newWordSubmittedMsg{text: text}
	}
}

type wordLoadedMsg struct {
	word storage.WordDetails
}

type wordsLoadedMsg struct {
	words []storage.Word
}
