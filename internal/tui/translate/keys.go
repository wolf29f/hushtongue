package translate

import (
	"charm.land/bubbles/v2/key"
	"github.com/wolf29f/hushtongue/internal/tui/translate/components/wordchoice"
	"github.com/wolf29f/hushtongue/internal/tui/translate/components/wordinput"
)

// keyMap shows the bindings of the focused element.
type keyMap struct {
	focus focus

	FocusNext key.Binding
	FocusPrev key.Binding

	input  wordinput.KeyMap
	choice wordchoice.KeyMap

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
		return []key.Binding{k.input.Commit, k.FocusNext, k.Copy, k.QuitInput}
	}
	return []key.Binding{k.HelpKey, k.Quit}
}

// FullHelp returns keybindings for the expanded help view. It's part of the
// key.Map interface.
func (k keyMap) FullHelp() [][]key.Binding {
	switch k.focus {
	case focusInput:
		return [][]key.Binding{
			{k.input.Commit, k.input.Erase},
			{k.input.PrevSuggestion, k.input.NextSuggestion},
			{k.FocusNext, k.FocusPrev},
			{k.Copy, k.Paste},
			{k.QuitInput},
		}
	case focusChoice:
		return [][]key.Binding{
			{k.choice.PrevWord, k.choice.NextWord},
			{k.choice.PrevCandidate, k.choice.NextCandidate},
			{k.choice.Add, k.choice.Delete},
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
	input:  wordinput.Keys,
	choice: wordchoice.Keys,

	FocusNext: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "focus suivant"),
	),
	FocusPrev: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "focus précédent"),
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
