package inputmodal

import (
	"charm.land/bubbles/v2/key"
)

type keyMap struct {
	Enter   key.Binding
	HelpKey key.Binding
	Quit    key.Binding
}

// ShortHelp returns keybindings to be shown in the mini help view. It's part
// of the key.Map interface.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.HelpKey, k.Quit}
}

// FullHelp returns keybindings for the expanded help view. It's part of the
// key.Map interface.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Enter},
		{k.HelpKey},
		{k.Quit},
	}
}

// Help returns the keybinding that toggles the help view. It's part of the
// tui.KeyMapHelper interface.
func (k keyMap) Help() key.Binding {
	return k.HelpKey
}

func newKeyMap(submitHelp string) keyMap {
	if submitHelp == "" {
		submitHelp = "valider"
	}

	return keyMap{
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("⏎", submitHelp),
		),
		HelpKey: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "aide"),
		),
		Quit: key.NewBinding(
			key.WithKeys("esc", "ctrl+c"),
			key.WithHelp("esc", "quitter"),
		),
	}
}
