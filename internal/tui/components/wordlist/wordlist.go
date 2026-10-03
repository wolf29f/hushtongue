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
	// Unbound rather than disabled: the list re-enables them on resize.
	// Long lists are browsed with the filter instead.
	l.KeyMap.NextPage.Unbind()
	l.KeyMap.PrevPage.Unbind()

	return Model{
		Model:        l,
		Style:        styles.Box,
		FocusedStyle: styles.BoxFocused,
	}
}

// SetSize sets the total size, border and padding included.
func (m Model) SetSize(width, height int) Model {
	m.Model.SetSize(
		width-m.Style.GetHorizontalFrameSize(),
		height-m.Style.GetVerticalFrameSize(),
	)
	return m
}

// Reset clears the filter and moves the cursor back to the first item.
func (m Model) Reset() Model {
	m.ResetFilter()
	m.Select(0)
	return m
}

func (m Model) SetItems(words []storage.Word) (Model, tea.Cmd) {
	items := make([]list.Item, len(words))
	for i, w := range words {
		items[i] = item(w)
	}
	cmd := m.Model.SetItems(items)
	return m, cmd
}

// SelectedWord returns the word under the cursor, if any.
func (m Model) SelectedWord() (storage.Word, bool) {
	i, ok := m.SelectedItem().(item)
	return storage.Word(i), ok
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// Keys only go to the focused component, other messages always do
		if !m.Focused {
			return m, nil
		}
		if m.FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "enter":
			if i, ok := m.SelectedItem().(item); ok {
				m.chosen = &i
				return m, func() tea.Msg {
					return WordSelectedMsg{ID: i.ID}
				}
			}
		}
	}

	var cmd tea.Cmd
	m.Model, cmd = m.Model.Update(msg)
	return m, cmd
}

// CapturesKey reports whether the list needs msg for itself, in which case
// the parent must not handle it.
func (m Model) CapturesKey(msg tea.KeyPressMsg) bool {
	if !m.Focused {
		return false
	}
	switch m.FilterState() {
	case list.Filtering:
		return true
	case list.FilterApplied:
		return key.Matches(msg, m.KeyMap.ClearFilter)
	}
	return false
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

// KeyMapHelper returns the list's keymap, extended with the parent's extra
// bindings.
func (m Model) KeyMapHelper(extra ...key.Binding) tui.KeyMapHelper {
	fullHelp := m.FullHelp()
	if len(extra) > 0 {
		fullHelp = append(fullHelp, extra)
	}
	return keyMapHelper{
		shortHelp: append(m.ShortHelp(), extra...),
		fullHelp:  fullHelp,
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

type WordSelectedMsg struct {
	ID int
}
