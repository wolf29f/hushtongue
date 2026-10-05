package translationpicker

import (
	"charm.land/bubbles/v2/key"
)

type keyMap struct {
	FocusNext key.Binding
	FocusPrev key.Binding
	Pick      key.Binding
	NewWord   key.Binding
	Generate  key.Binding
	Quit      key.Binding
}

var keys = keyMap{
	FocusNext: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "focus suivant"),
	),
	FocusPrev: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "focus précédent"),
	),
	Pick: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("⏎", "choisir la traduction"),
	),
	NewWord: key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "nouveau mot"),
	),
	Generate: key.NewBinding(
		key.WithKeys("g"),
		key.WithHelp("g", "générer"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "esc"),
		key.WithHelp("q", "quitter"),
	),
}
