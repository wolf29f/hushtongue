package addmodal

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/wolf29f/hushtongue/internal/services"
	"github.com/wolf29f/hushtongue/internal/tui"
)

// --- Model ---

type Model struct {
	services *services.Services

	width, height int
}

func NewModel(services *services.Services) Model {
	return Model{
		services: services,
	}
}

func (m Model) Init() tea.Cmd {
	return tui.SetKeyMap(keys)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tui.PopModal
		}
	}

	return m, nil
}

func (m Model) View() tea.View {
	content := lipgloss.NewStyle().
		Padding(2).
		Border(lipgloss.RoundedBorder()).
		Render("Create Word Placeholder")

	return tea.NewView(content)
}
