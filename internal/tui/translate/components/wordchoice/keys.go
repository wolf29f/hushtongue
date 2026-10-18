package wordchoice

import (
	"charm.land/bubbles/v2/key"
)

// KeyMap holds the bindings of the choice, for the page's help.
type KeyMap struct {
	PrevWord      key.Binding
	NextWord      key.Binding
	PrevCandidate key.Binding
	NextCandidate key.Binding
	Add           key.Binding
	Delete        key.Binding
}

var Keys = KeyMap{
	PrevWord: key.NewBinding(
		key.WithKeys("left"),
		key.WithHelp("←", "mot précédent"),
	),
	NextWord: key.NewBinding(
		key.WithKeys("right"),
		key.WithHelp("→", "mot suivant"),
	),
	PrevCandidate: key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "traduction précédente"),
	),
	NextCandidate: key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "traduction suivante"),
	),
	Add: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a/⏎ sur +", "ajouter une traduction"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d", "backspace", "delete"),
		key.WithHelp("d/⌫/suppr", "supprimer la traduction"),
	),
}

// pressKey adds a translation when the "+" is selected.
var pressKey = key.NewBinding(key.WithKeys("enter"))
