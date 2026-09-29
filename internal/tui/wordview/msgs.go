package wordview

import (
	tea "charm.land/bubbletea/v2"
	"github.com/wolf29f/hushtongue/internal/services/storage"
)

type switchKindMsg struct{}

type editWordMsg struct{}

type deleteWordMsg struct{}

type newTextMsg struct {
	text string
}

func newText(text string) tea.Cmd {
	return func() tea.Msg {
		return newTextMsg{text: text}
	}
}

type wordDetailsMsg struct {
	Word storage.WordDetails
}
