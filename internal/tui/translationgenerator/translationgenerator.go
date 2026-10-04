package translationgenerator

import (
	"fmt"
	"io"
	"log/slog"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/wolf29f/hushtongue/internal/services"
	"github.com/wolf29f/hushtongue/internal/services/storage"
	"github.com/wolf29f/hushtongue/internal/services/translation"
	"github.com/wolf29f/hushtongue/internal/tui"
	"github.com/wolf29f/hushtongue/internal/tui/styles"
)

// partSeparator separates the parts of a decomposed word.
const partSeparator = "-"

// columnGap is the space between the source and con columns.
const columnGap = 3

// cursorWidth is the width of the cursor in front of each proposal.
const cursorWidth = 2

type item translation.Proposal

func (i item) FilterValue() string { return "" }

func (i item) source() string {
	parts := make([]string, len(i.Parts))
	for j, part := range i.Parts {
		parts[j] = part.Source
	}
	return strings.Join(parts, partSeparator)
}

// con renders the con parts, green when known, red when generated.
func (i item) con() string {
	parts := make([]string, len(i.Parts))
	for j, part := range i.Parts {
		style := generatedStyle
		if part.ConID != 0 {
			style = knownStyle
		}
		parts[j] = style.Render(part.Con)
	}
	return strings.Join(parts, partSeparator)
}

type proposalDelegate struct {
	sourceWidth int
}

func (d proposalDelegate) Height() int                               { return 1 }
func (d proposalDelegate) Spacing() int                              { return 0 }
func (d proposalDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }

func (d proposalDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	cursor, sourceStyle := "  ", lipgloss.NewStyle()
	if index == m.Index() {
		cursor, sourceStyle = "▸ ", selectedStyle
	}
	source := sourceStyle.Width(d.sourceWidth + columnGap).Render(i.source())

	if _, err := fmt.Fprint(w, sourceStyle.Render(cursor)+source+i.con()); err != nil {
		slog.Error("failed to render proposal", "error", err)
	}
}

// Model is the page generating a translation for a source word, from its
// decompositions into affixes and root.
type Model struct {
	services *services.Services

	// State
	wordID int
	word   storage.WordDetails

	// Components
	proposals list.Model

	// UI stuff
	width, height int
	// Widths of the source and con columns, the list is sized to fit them
	sourceWidth, conWidth int
}

// NewModel creates the page generating a translation for the word wordID.
func NewModel(wordID int, services *services.Services) Model {
	l := list.New(nil, proposalDelegate{}, 0, 0)
	l.SetShowStatusBar(false)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)

	m := Model{
		services:  services,
		wordID:    wordID,
		proposals: l,
	}
	m = m.computeLayout()

	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.load,
		tui.SetKeyMap(keys),
	)
}

func (m Model) load() tea.Msg {
	word, err := m.services.Storage.GetWord(m.wordID)
	if err != nil {
		slog.Error("unable to get word details", "error", err)
		return nil
	}

	proposals, err := m.services.Translation.Propose(m.wordID)
	if err != nil {
		slog.Error("unable to propose translations", "error", err)
		return nil
	}

	return loadedMsg{word: word, proposals: proposals}
}

func (m Model) apply(proposal translation.Proposal) tea.Cmd {
	return func() tea.Msg {
		if err := m.services.Translation.Apply(m.wordID, proposal); err != nil {
			slog.Error("unable to save translation", "error", err)
			return nil
		}
		return tui.PopPage()
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loadedMsg:
		return m.handleLoadedMsg(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.computeLayout()
		return m, nil
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tui.PopPage
		case key.Matches(msg, keys.Enter):
			if i, ok := m.proposals.SelectedItem().(item); ok {
				return m, m.apply(translation.Proposal(i))
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.proposals, cmd = m.proposals.Update(msg)
	return m, cmd
}

func (m Model) handleLoadedMsg(msg loadedMsg) (Model, tea.Cmd) {
	m.word = msg.word

	items := make([]list.Item, len(msg.proposals))
	m.sourceWidth, m.conWidth = 0, 0
	for i, proposal := range msg.proposals {
		items[i] = item(proposal)
		m.sourceWidth = max(m.sourceWidth, lipgloss.Width(item(proposal).source()))
		m.conWidth = max(m.conWidth, lipgloss.Width(item(proposal).con()))
	}
	m.proposals.SetDelegate(proposalDelegate{sourceWidth: m.sourceWidth})

	cmd := m.proposals.SetItems(items)
	m = m.computeLayout()
	return m, cmd
}

// computeLayout sizes the list to fit its proposals, within the page.
func (m Model) computeLayout() Model {
	maxWidth := m.width - styles.BoxFocused.GetHorizontalFrameSize()
	maxHeight := m.height - lipgloss.Height(m.header()) - styles.BoxFocused.GetVerticalFrameSize()

	m.proposals.SetSize(
		max(0, min(cursorWidth+m.sourceWidth+columnGap+m.conWidth, maxWidth)),
		max(0, min(len(m.proposals.Items()), maxHeight)),
	)
	return m
}

func (m Model) header() string {
	return wordStyle.Render(m.word.Text) + kindStyle.Render("<"+m.word.Kind+">")
}

func (m Model) View() tea.View {
	box := styles.BoxFocused.Render(
		lipgloss.PlaceHorizontal(m.proposals.Width(), lipgloss.Left, m.proposals.View()),
	)

	return tea.NewView(lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Left, m.header(), box),
	))
}
