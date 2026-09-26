package button

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/wolf29f/hushtongue/internal/tui/styles"
)

var pressKey = key.NewBinding(key.WithKeys("enter"))

type Model struct {
	Content      string
	Style        lipgloss.Style
	FocusedStyle lipgloss.Style

	// Total size, border and padding included
	Width, Height int

	Focused bool

	// Emitted when enter is pressed while focused
	OnPress tea.Cmd
}

// New creates a button with the default box styles.
func New(content string, onPress tea.Cmd) Model {
	return Model{
		Content:      content,
		Style:        styles.Box,
		FocusedStyle: styles.BoxFocused,
		OnPress:      onPress,
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.Focused {
		return m, nil
	}
	if msg, ok := msg.(tea.KeyPressMsg); ok && key.Matches(msg, pressKey) {
		return m, m.OnPress
	}
	return m, nil
}

func (m Model) View() string {
	style := m.Style
	if m.Focused {
		style = m.FocusedStyle
	}

	innerWidth := max(0, m.Width-style.GetHorizontalFrameSize())
	innerHeight := max(0, m.Height-style.GetVerticalFrameSize())

	return style.Render(lipgloss.Place(
		innerWidth, innerHeight,
		lipgloss.Center, lipgloss.Center,
		m.Content,
	))
}
