package translationpicker

import (
	"charm.land/bubbles/v2/key"
)

type keyMap struct {
	FocusNext key.Binding
	FocusPrev key.Binding
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
	Generate: key.NewBinding(
		key.WithKeys("g"),
		key.WithHelp("g", "générer"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "esc"),
		key.WithHelp("q", "quitter"),
	),
}
