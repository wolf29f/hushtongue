package confirmmodal

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/wolf29f/hushtongue/internal/tui"
	"github.com/wolf29f/hushtongue/internal/tui/components/button"
)

// buttonGap is the space between the confirm and cancel buttons.
const buttonGap = 2

type Config struct {
	Title string
	// Button labels, default to "Valider" and "Annuler"
	ConfirmLabel string
	CancelLabel  string

	// Emitted on confirm, after the modal is closed. May be nil.
	OnConfirm tea.Cmd
	// Emitted on cancel, after the modal is closed. May be nil.
	OnCancel tea.Cmd
}

// --- Model ---

type Model struct {
	title string

	confirmButton button.Model
	cancelButton  button.Model
	// Whether the confirm button has focus, the cancel button otherwise
	confirmFocused bool

	onCancel tea.Cmd

	width, height int
}

func New(cfg Config) Model {
	confirmLabel := cfg.ConfirmLabel
	if confirmLabel == "" {
		confirmLabel = "Valider"
	}
	cancelLabel := cfg.CancelLabel
	if cancelLabel == "" {
		cancelLabel = "Annuler"
	}

	m := Model{
		title:         cfg.Title,
		confirmButton: button.New(confirmLabel, tea.Sequence(tui.PopModal, cfg.OnConfirm)),
		cancelButton:  button.New(cancelLabel, tea.Sequence(tui.PopModal, cfg.OnCancel)),
		onCancel:      cfg.OnCancel,
	}

	// Cancel has focus by default, so that a stray enter doesn't confirm
	return m.applyFocus()
}

func (m Model) Init() tea.Cmd {
	return tui.SetKeyMap(keys)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Sequence(tui.PopModal, m.onCancel)
		case key.Matches(msg, keys.Switch):
			m.confirmFocused = !m.confirmFocused
			return m.applyFocus(), nil
		}
	}

	// Only the focused button reacts
	var confirmCmd, cancelCmd tea.Cmd
	m.confirmButton, confirmCmd = m.confirmButton.Update(msg)
	m.cancelButton, cancelCmd = m.cancelButton.Update(msg)
	return m, tea.Batch(confirmCmd, cancelCmd)
}

// applyFocus propagates m.confirmFocused to the buttons.
func (m Model) applyFocus() Model {
	m.confirmButton.Focused = m.confirmFocused
	m.cancelButton.Focused = !m.confirmFocused
	return m
}

func (m Model) View() tea.View {
	buttons := lipgloss.JoinHorizontal(
		lipgloss.Center,
		m.confirmButton.View(),
		strings.Repeat(" ", buttonGap),
		m.cancelButton.View(),
	)

	content := lipgloss.NewStyle().
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		Render(
			lipgloss.JoinVertical(lipgloss.Center, m.title+"\n", buttons),
		)

	return tea.NewView(content)
}
