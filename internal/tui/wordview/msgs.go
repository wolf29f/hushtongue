package wordview

import (
	tea "charm.land/bubbletea/v2"
	"github.com/wolf29f/hushtongue/internal/services/storage"
)

type switchKindPressedMsg struct{}

type editPressedMsg struct{}

type addTranslationPressedMsg struct{}

type deleteConfirmedMsg struct{}

type wordDeletedMsg struct{}

type kindChangeConfirmedMsg struct {
	kind string
}

type textSubmittedMsg struct {
	text string
}

func textSubmitted(text string) tea.Cmd {
	return func() tea.Msg {
		return textSubmittedMsg{text: text}
	}
}

type wordLoadedMsg struct {
	Word storage.WordDetails
}

type translationsLoadedMsg struct {
	translations []storage.Translation
}

type translationDeleteConfirmedMsg struct {
	id int
}
