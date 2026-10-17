package translate

import (
	"charm.land/bubbles/v2/key"
)

// keyMap shows the bindings of the focused element.
type keyMap struct {
	focus focus

	FocusNext key.Binding
	FocusPrev key.Binding

	Commit         key.Binding
	PrevSuggestion key.Binding
	NextSuggestion key.Binding
	Erase          key.Binding

	PrevWord      key.Binding
	NextWord      key.Binding
	PrevCandidate key.Binding
	NextCandidate key.Binding
	Add           key.Binding
	Delete        key.Binding

	Press key.Binding
	Copy  key.Binding
	Paste key.Binding

	HelpKey   key.Binding
	Quit      key.Binding
	QuitInput key.Binding
}

// ShortHelp returns keybindings to be shown in the mini help view. It's part
// of the key.Map interface.
func (k keyMap) ShortHelp() []key.Binding {
	if k.focus == focusInput {
		// The help key is typed in the input: show the essentials instead
		return []key.Binding{k.Commit, k.FocusNext, k.Copy, k.QuitInput}
	}
	return []key.Binding{k.HelpKey, k.Quit}
}

// FullHelp returns keybindings for the expanded help view. It's part of the
// key.Map interface.
func (k keyMap) FullHelp() [][]key.Binding {
	switch k.focus {
	case focusInput:
		return [][]key.Binding{
			{k.Commit, k.Erase},
			{k.PrevSuggestion, k.NextSuggestion},
			{k.FocusNext, k.FocusPrev},
			{k.Copy, k.Paste},
			{k.QuitInput},
		}
	case focusChoice:
		return [][]key.Binding{
			{k.PrevWord, k.NextWord},
			{k.PrevCandidate, k.NextCandidate},
			{k.Add, k.Delete},
			{k.FocusNext, k.FocusPrev},
			{k.Copy, k.Paste},
			{k.HelpKey},
			{k.Quit},
		}
	}
	return [][]key.Binding{
		{k.Press},
		{k.FocusNext, k.FocusPrev},
		{k.Copy, k.Paste},
		{k.HelpKey},
		{k.Quit},
	}
}

// Help returns the keybinding that toggles the help view. It's part of the
// tui.KeyMapHelper interface.
func (k keyMap) Help() key.Binding {
	if k.focus == focusInput {
		return key.NewBinding(key.WithDisabled())
	}
	return k.HelpKey
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
	Press: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("⏎", "valider"),
	),
	Copy: key.NewBinding(
		key.WithKeys("ctrl+y"),
		key.WithHelp("ctrl+y", "copier la traduction"),
	),
	Paste: key.NewBinding(
		key.WithKeys("ctrl+v"),
		key.WithHelp("ctrl+v", "coller un texte"),
	),
	HelpKey: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "aide"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "esc"),
		key.WithHelp("q", "quitter"),
	),
	QuitInput: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "quitter"),
	),
}
