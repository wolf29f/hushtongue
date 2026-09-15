package home

import (
	"charm.land/bubbles/v2/key"
)

// FIXME: add a HelpMsg for a component who get focus to propagate the help to the tui elements and then let the tui element display the help.
// Implies to handle elements focus for composed views.

// keyMap defines a set of keybindings. To work for help it must satisfy
// key.Map. It could also very easily be a map[string]key.Binding.
type keyMap struct {
	Up      key.Binding
	Down    key.Binding
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
		{k.Up, k.Down},
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

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "choix précédent"),
	),
	Down: key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "choix suivant"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("⏎", "valider"),
	),
	HelpKey: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "aide"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "esc", "ctrl+c"),
		key.WithHelp("q", "quitter"),
	),
}
