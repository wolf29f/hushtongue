package addmodal

import tea "charm.land/bubbletea/v2"

type NewWordMsg struct {
	Word string
}

func NewWord(word string) tea.Cmd {
	return func() tea.Msg {
		return NewWordMsg{Word: word}
	}
}
