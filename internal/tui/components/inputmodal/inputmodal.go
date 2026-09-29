package inputmodal

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/wolf29f/hushtongue/internal/tui"
)

type Config struct {
	Title       string
	Placeholder string
	// Initial value of the input
	Value string
	// Help text of the submit key, defaults to "valider"
	SubmitHelp string

	// Called with the input value on submit, after the modal is closed
	OnSubmit func(value string) tea.Cmd
	// Emitted on cancel, after the modal is closed. May be nil.
	OnCancel tea.Cmd
}

// --- Model ---

type Model struct {
	textInput textinput.Model
	title     string
	keys      keyMap

	onSubmit func(string) tea.Cmd
	onCancel tea.Cmd

	width, height int
}

// New creates a text input modal.
func New(cfg Config) Model {
	ti := textinput.New()
	ti.Placeholder = cfg.Placeholder
	ti.Focus()
	ti.CharLimit = 156
	ti.SetWidth(20)
	ti.SetValue(cfg.Value)

	return Model{
		textInput: ti,
		title:     cfg.Title,
		keys:      newKeyMap(cfg.SubmitHelp),
		onSubmit:  cfg.OnSubmit,
		onCancel:  cfg.OnCancel,
	}
}

func (m Model) Init() tea.Cmd {
	return tui.SetKeyMap(m.keys)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Sequence(tui.PopModal, m.onCancel)
		case key.Matches(msg, m.keys.Enter):
			var cmd tea.Cmd
			if m.onSubmit != nil {
				cmd = m.onSubmit(m.textInput.Value())
			}
			return m, tea.Sequence(tui.PopModal, cmd)
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
			lipgloss.JoinVertical(lipgloss.Top, m.title+"\n", m.textInput.View()),
		)

	return tea.NewView(content)
}
