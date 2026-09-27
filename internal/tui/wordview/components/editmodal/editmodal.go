package editmodal

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/wolf29f/hushtongue/internal/tui"
)

// --- Model ---

type Model struct {
	textInput textinput.Model

	width, height int
}

func NewModel(defaultText string) Model {
	ti := textinput.New()
	ti.Placeholder = "Saisissez votre mot"
	ti.Focus()
	ti.CharLimit = 156
	ti.SetWidth(20)
	ti.SetValue(defaultText)

	return Model{
		textInput: ti,
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
		case key.Matches(msg, keys.Enter):
			return m, tea.Sequence(tui.PopModal, NewText(m.textInput.Value()))
		}
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m Model) View() tea.View {
	content := lipgloss.NewStyle().
		Padding(2).
		Border(lipgloss.RoundedBorder()).
		Render(
			lipgloss.JoinVertical(lipgloss.Top, m.headerView(), m.textInput.View()),
		)

	return tea.NewView(content)
}

func (m Model) headerView() string { return "Veuillez saisir le texte\n" }
