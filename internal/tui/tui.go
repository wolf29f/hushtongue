package tui

import (
	"log/slog"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type RootModel struct {
	stack []tea.Model
	modal tea.Model
	// errors queues the messages of the error modal, shown above the page
	// and the modal, front first.
	errors        []string
	width, height int

	// Help related fields
	keyMap KeyMapHelper
	// pageKeyMap holds the page's keymap while a modal is open, so it can
	// be restored when the modal closes.
	pageKeyMap KeyMapHelper
	// hiddenKeyMap holds the page's or modal's keymap while the error modal
	// is open.
	hiddenKeyMap KeyMapHelper
	help         help.Model
	footer       string
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
		if m.modal != nil {
			updated, cmd := m.modal.Update(msg)
			m.modal = updated
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
		// The error modal swallows every key
		if len(m.errors) > 0 {
			if press, ok := msg.(tea.KeyPressMsg); ok && key.Matches(press, errorKeys.Close) {
				return m.dismissError(), m.sizeCmd()
			}
			return m, nil
		}

	case pushPageMsg:
		// The pushed page hasn't announced a keymap yet: don't keep showing
		// the previous page's help for something no longer on screen.
		m = m.setKeyMap(nil)
		m.stack = append(m.stack, msg.page)
		return m, tea.Batch(msg.page.Init(), m.sizeCmd())

	case popPageMsg:
		if len(m.stack) > 1 {
			m.stack = m.stack[:len(m.stack)-1]
		}
		// Re-init the revealed page so it refreshes its data and
		// re-announces its keymap.
		m = m.setKeyMap(nil)
		top := len(m.stack) - 1
		return m, tea.Batch(m.stack[top].Init(), m.sizeCmd())

	case replacePageMsg:
		m = m.setKeyMap(nil)
		top := len(m.stack) - 1
		m.stack[top] = msg.page
		return m, tea.Batch(msg.page.Init(), m.sizeCmd())

	case pushKeyMapMsg:
		slog.Debug("received keymap msg")
		m = m.setKeyMap(msg.keyMap)
		slog.Debug("updated keymap msg", "keyMap", m.keyMap, "footer", m.footer)
		return m, m.sizeCmd()

	case pushModalMsg:
		m.pageKeyMap = m.underlyingKeyMap()
		m.modal = msg.modal
		return m, tea.Batch(msg.modal.Init(), m.sizeCmd())
	case popModalMsg:
		// Only restore the page's keymap: re-initializing the page here
		// would race with the modal's result message.
		m.modal = nil
		m = m.setKeyMap(m.pageKeyMap)
		m.pageKeyMap = nil
		return m, m.sizeCmd()

	case showErrorMsg:
		if len(m.errors) == 0 {
			m.hiddenKeyMap = m.keyMap
			m.keyMap = errorKeys
			m.footer = m.helpView()
		}
		m.errors = append(m.errors, msg.message)
		return m, m.sizeCmd()
	}

	// If modal => route message to the active modal
	if m.modal != nil {
		var cmd tea.Cmd
		m.modal, cmd = m.modal.Update(msg)
		return m, cmd
	}
	// No modal => route message to the top of the stack
	top := len(m.stack) - 1
	updated, cmd := m.stack[top].Update(msg)
	m.stack[top] = updated
	return m, cmd
}

// setKeyMap sets the page's or modal's keymap. While the error modal is open,
// it is kept aside until the last error is dismissed.
func (m RootModel) setKeyMap(keyMap KeyMapHelper) RootModel {
	if len(m.errors) > 0 {
		m.hiddenKeyMap = keyMap
		return m
	}
	m.keyMap = keyMap
	m.footer = m.helpView()
	return m
}

// underlyingKeyMap returns the page's or modal's keymap, even while the error
// modal is open.
func (m RootModel) underlyingKeyMap() KeyMapHelper {
	if len(m.errors) > 0 {
		return m.hiddenKeyMap
	}
	return m.keyMap
}

func (m RootModel) dismissError() RootModel {
	m.errors = m.errors[1:]
	if len(m.errors) == 0 {
		m.keyMap = m.hiddenKeyMap
		m.hiddenKeyMap = nil
		m.footer = m.helpView()
	}
	return m
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
	pageView := m.stack[len(m.stack)-1].View()
	pageView.Content = pageView.Content + "\n" + m.footer
	pageView.AltScreen = true

	if m.modal == nil && len(m.errors) == 0 {
		return pageView
	}

	layers := []*lipgloss.Layer{lipgloss.NewLayer(pageView.Content).Z(0)}
	if m.modal != nil {
		layers = append(layers, m.centeredLayer(m.modal.View().Content).Z(1))
	}
	if len(m.errors) > 0 {
		layers = append(layers, m.centeredLayer(errorModalView(m.errors[0])).Z(2))
	}

	v := tea.NewView(lipgloss.NewCompositor(layers...).Render())
	v.AltScreen = true
	return v
}

func (m RootModel) centeredLayer(content string) *lipgloss.Layer {
	w, h := lipgloss.Size(content)
	return lipgloss.NewLayer(content).X((m.width - w) / 2).Y((m.height - h) / 2)
}
