package tui

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// Navigation messages for managing pages in the TUI stack. They are only
// handled by the root Model: pages emit them through the commands below.
type pushPageMsg struct {
	page tea.Model
}
type popPageMsg struct{}
type replacePageMsg struct {
	page tea.Model
}

type KeyMapHelper interface {
	help.KeyMap
	Help() key.Binding
}

type pushKeyMapMsg struct {
	keyMap KeyMapHelper
}

type pushModalMsg struct {
	modal tea.Model
}

type popModalMsg struct{}

// PushPage pushes page on top of the page stack.
func PushPage(page tea.Model) tea.Cmd {
	return func() tea.Msg {
		return pushPageMsg{page: page}
	}
}

// PopPage removes the current page from the stack.
func PopPage() tea.Msg {
	return popPageMsg{}
}

// ReplacePage replaces the current page with page.
func ReplacePage(page tea.Model) tea.Cmd {
	return func() tea.Msg {
		return replacePageMsg{page: page}
	}
}

// SetKeyMap sets the keymap displayed in the help footer.
func SetKeyMap(keyMap KeyMapHelper) tea.Cmd {
	return func() tea.Msg {
		return pushKeyMapMsg{keyMap: keyMap}
	}
}

// PushModal displays modal on top of the current page.
func PushModal(modal tea.Model) tea.Cmd {
	return func() tea.Msg {
		return pushModalMsg{modal: modal}
	}
}

// PopModal closes the current modal.
func PopModal() tea.Msg {
	return popModalMsg{}
}
