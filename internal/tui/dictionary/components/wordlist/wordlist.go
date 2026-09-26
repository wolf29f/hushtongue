package wordlist

import (
	"fmt"
	"io"
	"log/slog"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/wolf29f/hushtongue/internal/services/storage"
	"github.com/wolf29f/hushtongue/internal/tui"
	"github.com/wolf29f/hushtongue/internal/tui/styles"
)

type item storage.Word

func (i item) FilterValue() string { return i.Text }

type wordDelegate struct{}

func (d wordDelegate) Height() int                               { return 1 }
func (d wordDelegate) Spacing() int                              { return 0 }
func (d wordDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }

func (d wordDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	str := i.Text
	style := lipgloss.NewStyle().PaddingLeft(2)
	if index == m.Index() {
		style = lipgloss.NewStyle().
			PaddingLeft(0).
			Foreground(lipgloss.Color("170")).
			Bold(true)
		str = "▸ " + str
	}
	if _, err := fmt.Fprint(w, style.Render(str)); err != nil {
		slog.Error("failed to render word list item", "error", err)
	}
}

// --- Model ---

type Model struct {
	list.Model
	chosen *item

	Style        lipgloss.Style
	FocusedStyle lipgloss.Style
	Focused      bool
}

func NewModel(words []storage.Word) Model {
	items := make([]list.Item, len(words))
	for i, w := range words {
		items[i] = item(w)
	}

	l := list.New(items, wordDelegate{}, 0, 0)
	l.SetShowStatusBar(false)
	l.SetShowTitle(false)
	l.SetShowHelp(false)

	l.KeyMap.CursorUp.SetHelp("↑/k", "monter")
	l.KeyMap.CursorDown.SetHelp("↓/j", "descendre")
	l.KeyMap.Filter.SetHelp("/", "rechercher")
	l.KeyMap.ClearFilter.SetHelp("esc", "effacer le filtre")
	l.KeyMap.Quit.SetHelp("q", "quitter")
	l.KeyMap.CloseFullHelp.SetHelp("?", "fermer l'aide")

	return Model{
		Model:        l,
		Style:        styles.Box,
		FocusedStyle: styles.BoxFocused,
	}
}

// SetSize sets the total size, border and padding included.
func (m *Model) SetSize(width, height int) {
	m.Model.SetSize(
		width-m.Style.GetHorizontalFrameSize(),
		height-m.Style.GetVerticalFrameSize(),
	)
}

func (m *Model) SetItems(words []storage.Word) {
	items := make([]list.Item, len(words))
	for i, w := range words {
		items[i] = item(w)
	}
	m.Model.SetItems(items)
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)

	case tea.KeyPressMsg:
		if m.FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "enter":
			if i, ok := m.SelectedItem().(item); ok {
				m.chosen = &i
				return m, tea.Quit
			}
		}
	}

	var cmd tea.Cmd
	m.Model, cmd = m.Model.Update(msg)
	return m, cmd
}

func (m Model) View() tea.View {
	style := m.Style
	if m.Focused {
		style = m.FocusedStyle
	}

	return tea.NewView(style.Render(
		lipgloss.PlaceHorizontal(m.Width(), lipgloss.Left, m.Model.View()),
	))
}

func (m Model) KeyMapHelper() tui.KeyMapHelper {
	return keyMapHelper{
		shortHelp: m.ShortHelp(),
		fullHelp:  m.FullHelp(),
		helpKey:   m.KeyMap.ShowFullHelp,
	}
}

type keyMapHelper struct {
	shortHelp []key.Binding
	fullHelp  [][]key.Binding
	helpKey   key.Binding
}

func (kmh keyMapHelper) ShortHelp() []key.Binding {
	return kmh.shortHelp
}

func (kmh keyMapHelper) FullHelp() [][]key.Binding {
	return kmh.fullHelp
}

func (kmh keyMapHelper) Help() key.Binding {
	return kmh.helpKey
}

var _ tui.KeyMapHelper = (*keyMapHelper)(nil)
