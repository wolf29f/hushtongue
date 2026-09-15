package tui

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// Navigation messages for managing pages in the TUI stack
type PushPageMsg struct {
	Page tea.Model
}
type PopPageMsg struct{}
type ReplacePageMsg struct {
	Page tea.Model
}

type KeyMapHelper interface {
	help.KeyMap
	Help() key.Binding
}

type PushKeyMapMsg struct {
	KeyMap KeyMapHelper
}

type PushModalMsg struct {
	Modal tea.Model
}
