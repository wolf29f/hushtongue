package startup

import (
	"fmt"
	"log/slog"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/wolf29f/hushtongue/internal/services"
	"github.com/wolf29f/hushtongue/internal/tui"
	"github.com/wolf29f/hushtongue/internal/tui/home"
)

type Model struct {
	services *services.Services

	// State
	failed bool

	// UI stuff
	width, height int
	spinner       spinner.Model
}

func NewModel(services *services.Services) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return Model{
		services: services,
		spinner:  s,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			if err := m.services.Storage.Init(); err != nil {
				slog.Error("unable to initialize storage", "error", err)
				return InitFailedMsg{}
			}
			return InitDoneMsg{}
		},
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case InitDoneMsg:
		return m, tui.ReplacePage(home.NewModel(m.services))
	case InitFailedMsg:
		m.failed = true
		return m, nil
	case tea.KeyPressMsg:
		if m.failed {
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m Model) View() tea.View {
	content := fmt.Sprintf("%s Chargement...", m.spinner.View())
	if m.failed {
		content = "Impossible d'ouvrir le dictionnaire, consultez les logs.\n\nAppuyez sur une touche pour quitter."
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Render(content)

	return tea.NewView(lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		box,
	))
}
