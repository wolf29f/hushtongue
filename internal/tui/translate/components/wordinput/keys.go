package wordinput

import (
	"charm.land/bubbles/v2/key"
)

// KeyMap holds the bindings of the input, for the page's help.
type KeyMap struct {
	Commit         key.Binding
	PrevSuggestion key.Binding
	NextSuggestion key.Binding
	Erase          key.Binding
}

var Keys = KeyMap{
	Commit: key.NewBinding(
		key.WithKeys("space", "enter"),
		key.WithHelp("espace/⏎", "valider le mot"),
	),
	PrevSuggestion: key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "suggestion précédente"),
	),
	NextSuggestion: key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "suggestion suivante"),
	),
	Erase: key.NewBinding(
		key.WithKeys("backspace"),
		key.WithHelp("⌫", "effacer"),
	),
}
