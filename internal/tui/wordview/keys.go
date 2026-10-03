package wordview

import (
	"charm.land/bubbles/v2/key"
)

type keyMap struct {
	Enter     key.Binding
	FocusNext key.Binding
	FocusPrev key.Binding
	HelpKey   key.Binding

	AddTranslation    key.Binding
	DeleteTranslation key.Binding
	Quit              key.Binding
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
		{k.FocusNext, k.FocusPrev},
		{k.Enter},
		{k.AddTranslation},
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
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("⏎", "valider"),
	),
	FocusNext: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "focus suivant"),
	),
	FocusPrev: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "focus précédent"),
	),
	HelpKey: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "aide"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "esc"),
		key.WithHelp("q", "quitter"),
	),
	AddTranslation: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "ajouter une traduction"),
	),
	DeleteTranslation: key.NewBinding(
		key.WithKeys("d", "backspace", "delete"),
		key.WithHelp("d/⌫/suppr", "supprimer la traduction"),
	),
}
