package editmodal

import tea "charm.land/bubbletea/v2"

type NewTextMsg struct {
	Text string
}

func NewText(text string) tea.Cmd {
	return func() tea.Msg {
		return NewTextMsg{Text: text}
	}
}
