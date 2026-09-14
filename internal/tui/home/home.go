package home

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/wolf29f/hushtongue/internal/services"
	"github.com/wolf29f/hushtongue/internal/tui"
	"github.com/wolf29f/hushtongue/internal/tui/dictionary"
	"github.com/wolf29f/hushtongue/internal/tui/translate"
)

type Model struct {
	services services.Services

	// State
	options []struct {
		Label string
		Cmd   tea.Cmd
	}
	selectedOption int

	// IO
	keys keyMap

	// UI stuff
	width, height int
	help          help.Model
}

var _ tea.Model = Model{}

func NewModel(services *services.Services) Model {
	return Model{
		services: *services,
		options: []struct {
			Label string
			Cmd   tea.Cmd
		}{
			{
				Label: "🌐 Traduire",
				Cmd: func() tea.Msg {
					return tui.PushPageMsg{Page: translate.NewModel(services)}
				},
			},
			{
				Label: "📘 Dictionnaire",
				Cmd: func() tea.Msg {
					return tui.PushPageMsg{Page: dictionary.NewModel(services)}
				},
			},
			{
				Label: "🚪 Quitter",
				Cmd:   tea.Quit,
			},
		},

		keys: keys,
		help: help.New(),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Up):
			if m.selectedOption > 0 {
				m.selectedOption--
			}
		case key.Matches(msg, m.keys.Down):
			if m.selectedOption < len(m.options)-1 {
				m.selectedOption++
			}
		case key.Matches(msg, m.keys.Enter):
			if m.selectedOption >= 0 && m.selectedOption < len(m.options) {
				return m, m.options[m.selectedOption].Cmd
			}
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m Model) View() tea.View {

	rows := make([]string, len(m.options))
	for i, item := range m.options {
		if i == m.selectedOption {
			rows[i] = selectedStyle.Render(item.Label)
		} else {
			rows[i] = itemStyle.Render("  " + item.Label)
		}
	}

	list := lipgloss.JoinVertical(lipgloss.Left, rows...)

	menuBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Render(list)

	helpView := m.help.View(m.keys)

	_, helpHeight := lipgloss.Size(helpView)

	centered := lipgloss.Place(
		m.width, m.height-helpHeight,
		lipgloss.Center, lipgloss.Center,
		menuBox,
	)

	bottom := lipgloss.Place(
		m.width, helpHeight,
		lipgloss.Center, lipgloss.Bottom,
		helpView,
	)

	return tea.NewView(centered + "\n" + bottom)

}
