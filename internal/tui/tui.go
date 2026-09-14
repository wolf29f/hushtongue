package tui

import tea "charm.land/bubbletea/v2"

// Message pour naviguer
type PushPageMsg struct {
	Page tea.Model
}
type PopPageMsg struct{}
type ReplacePageMsg struct {
	Page tea.Model
}

type RootModel struct {
	stack         []tea.Model
	width, height int
}

func NewRootModel(initialPage tea.Model) RootModel {
	return RootModel{
		stack: []tea.Model{initialPage},
	}
}

func (m RootModel) Init() tea.Cmd {
	return m.stack[0].Init()
}

// sizeCmd re-emits the current terminal dimensions so a page pushed,
// replaced, or revealed after a pop always knows its size, even though
// the terminal itself hasn't resized.
func (m RootModel) sizeCmd() tea.Cmd {
	width, height := m.width, m.height
	return func() tea.Msg {
		return tea.WindowSizeMsg{Width: width, Height: height}
	}
}

func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Need to be propagated to the entire stack
		m.width, m.height = msg.Width, msg.Height
		var cmds []tea.Cmd
		for i, p := range m.stack {
			updated, cmd := p.Update(msg)
			m.stack[i] = updated
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case PushPageMsg:
		m.stack = append(m.stack, msg.Page)
		return m, tea.Batch(msg.Page.Init(), m.sizeCmd())

	case PopPageMsg:
		if len(m.stack) > 1 {
			m.stack = m.stack[:len(m.stack)-1]
		}
		return m, nil

	case ReplacePageMsg:
		top := len(m.stack) - 1
		m.stack[top] = msg.Page
		return m, tea.Batch(msg.Page.Init(), m.sizeCmd())
	}

	// route to the top of the stack only
	top := len(m.stack) - 1
	updated, cmd := m.stack[top].Update(msg)
	m.stack[top] = updated
	return m, cmd
}

func (m RootModel) View() tea.View {
	v := m.stack[len(m.stack)-1].View()
	v.AltScreen = true
	return v
}
