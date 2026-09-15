package tui

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type RootModel struct {
	stack         []tea.Model
	width, height int

	// Help related fields
	keyMap KeyMapHelper
	help   help.Model
	footer string
}

func NewRootModel(initialPage tea.Model) RootModel {
	return RootModel{
		stack: []tea.Model{initialPage},
		help:  help.New(),
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
		m.footer = m.helpView()

		// Subtract the footer height from the available window height.
		_, footerHeight := lipgloss.Size(m.footer)
		msg.Height -= footerHeight

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
		if m.keyMap != nil && key.Matches(msg, m.keyMap.Help()) {
			m.help.ShowAll = !m.help.ShowAll
			m.footer = m.helpView()
			return m, func() tea.Msg {
				return tea.WindowSizeMsg{Width: m.width, Height: m.height}
			}
		}

	case PushPageMsg:
		// The pushed page hasn't announced a keymap yet: don't keep showing
		// the previous page's help for something no longer on screen.
		m.keyMap = nil
		m.footer = m.helpView()
		m.stack = append(m.stack, msg.Page)
		return m, tea.Batch(msg.Page.Init(), m.sizeCmd())

	case PopPageMsg:
		if len(m.stack) > 1 {
			m.stack = m.stack[:len(m.stack)-1]
		}
		// The revealed page has no way to re-announce its keymap today, so
		// drop the stale one rather than show help for the popped page.
		m.keyMap = nil
		m.footer = m.helpView()
		return m, nil

	case ReplacePageMsg:
		m.keyMap = nil
		m.footer = m.helpView()
		top := len(m.stack) - 1
		m.stack[top] = msg.Page
		return m, tea.Batch(msg.Page.Init(), m.sizeCmd())

	case PushKeyMapMsg:
		m.keyMap = msg.KeyMap
		m.footer = m.helpView()
		return m, nil
	}

	// route to the top of the stack only
	top := len(m.stack) - 1
	updated, cmd := m.stack[top].Update(msg)
	m.stack[top] = updated
	return m, cmd
}

func (m RootModel) helpView() string {
	if m.keyMap == nil {
		return ""
	}
	helpView := m.help.View(m.keyMap)
	_, helpHeight := lipgloss.Size(helpView)
	footer := lipgloss.Place(
		m.width, helpHeight,
		lipgloss.Center, lipgloss.Bottom,
		helpView,
	)
	return footer
}

func (m RootModel) View() tea.View {
	v := m.stack[len(m.stack)-1].View()
	v.Content = v.Content + "\n" + m.footer
	v.AltScreen = true
	return v
}
