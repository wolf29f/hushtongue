package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/wolf29f/hushtongue/internal/tui/components/button"
)

type showErrorMsg struct {
	message string
}

// ShowError displays message in the error modal, above the page and any
// modal, until the user dismisses it. Messages shown in a row are queued.
func ShowError(message string) tea.Cmd {
	return func() tea.Msg {
		return showErrorMsg{message: message}
	}
}

var errorModalStyle = lipgloss.NewStyle().
	Padding(1, 2).
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("1"))

func errorModalView(message string) string {
	okButton := button.New("OK", nil)
	okButton.Focused = true
	okButton.Width = lipgloss.Width(okButton.Content) + okButton.FocusedStyle.GetHorizontalFrameSize()
	okButton.Height = okButton.FocusedStyle.GetVerticalFrameSize() + 1

	return errorModalStyle.Render(
		lipgloss.JoinVertical(lipgloss.Center, message+"\n", okButton.View()),
	)
}

type errorKeyMap struct {
	Close   key.Binding
	HelpKey key.Binding
}

// ShortHelp returns keybindings to be shown in the mini help view. It's part
// of the key.Map interface.
func (k errorKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Close, k.HelpKey}
}

// FullHelp returns keybindings for the expanded help view. It's part of the
// key.Map interface.
func (k errorKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Close},
		{k.HelpKey},
	}
}

// Help returns the keybinding that toggles the help view. It's part of the
// KeyMapHelper interface.
func (k errorKeyMap) Help() key.Binding {
	return k.HelpKey
}

var errorKeys = errorKeyMap{
	Close: key.NewBinding(
		key.WithKeys("enter", "esc"),
		key.WithHelp("⏎/esc", "fermer"),
	),
	HelpKey: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "aide"),
	),
}
