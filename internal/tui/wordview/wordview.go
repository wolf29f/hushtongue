package wordview

import (
	"log/slog"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/wolf29f/hushtongue/internal/services"
	"github.com/wolf29f/hushtongue/internal/services/storage"
	"github.com/wolf29f/hushtongue/internal/tui"
	"github.com/wolf29f/hushtongue/internal/tui/components/button"
	"github.com/wolf29f/hushtongue/internal/tui/components/confirmmodal"
	"github.com/wolf29f/hushtongue/internal/tui/components/inputmodal"
	"github.com/wolf29f/hushtongue/internal/tui/components/wordlist"
)

/*
- sur selection d'un mot:
	- ouverture d'une page dédiée:
	- mot (éditable)
	- type (éditable, si MJ)
	- traductions (list, add/remove/generate if original & MJ)
	- ctrl+s pour sauvegarder les modifications
	- bouton pour supprimer + modale de confirmation
*/

// buttonGap is the space between the buttons of the header row.
const buttonGap = 1

type focus int

const (
	focusSwitchKind focus = iota // only for MJ
	focusEditWord
	focusTranslations
	focusDeleteWord
	focusOverflowed
)

type Model struct {
	services *services.Services

	// State
	loading bool
	wordID  int
	word    storage.WordDetails
	focus   focus

	// Components
	translations     wordlist.Model
	editButton       button.Model
	switchKindButton button.Model
	deleteButton     button.Model

	// UI stuff
	width, height int
}

func NewModel(wordID int, services *services.Services) Model {

	m := Model{
		services: services,
		wordID:   wordID,
		loading:  true,
		focus:    focusEditWord,

		editButton: button.New("Éditer",
			func() tea.Msg { return editWordMsg{} },
		),
		switchKindButton: button.New(
			switchKindLabel("root"),
			func() tea.Msg { return switchKindMsg{} },
		),
		deleteButton: button.New("Supprimer", tui.PushModal(confirmmodal.New(confirmmodal.Config{
			Title:        "Supprimer ce mot ?",
			ConfirmLabel: "Supprimer",
			OnConfirm:    func() tea.Msg { return deleteWordMsg{} },
		}))),
	}

	// TODO: load the word's translations once the storage supports them
	m.translations = wordlist.NewModel(nil)
	m = m.applyFocus()
	m = m.computeLayout()

	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadWordDetails,
		tui.SetKeyMap(m.translations.KeyMapHelper()),
	)
}

func (m Model) loadWordDetails() tea.Msg {
	wordDetails, err := m.services.Storage.GetWord(m.wordID)
	if err != nil {
		slog.Error("unable to get word details", "error", err)
		return wordDetailsMsg{Word: storage.WordDetails{}}
	}

	return wordDetailsMsg{Word: wordDetails}
}

func (m Model) saveWordDetails() tea.Msg {
	wordDetails, err := m.services.Storage.SaveWord(m.word)
	if err != nil {
		slog.Error("unable to save word details", "error", err)
		// Reload the stored word to discard the rejected edit
		return m.loadWordDetails()
	}

	return wordDetailsMsg{Word: wordDetails}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case wordDetailsMsg:
		m.word = msg.Word
		m.loading = false
		m = m.computeLayout()
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.computeLayout()
		return m, nil
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, keys.FocusNext):
			m.focus = (m.focus + 1) % focusOverflowed
			return m.applyFocus(), nil
		case key.Matches(msg, keys.FocusPrev):
			m.focus = (m.focus + focusOverflowed - 1) % focusOverflowed
			return m.applyFocus(), nil
		case key.Matches(msg, keys.Quit):
			return m, tui.PopPage
		}
	case editWordMsg:
		return m, tui.PushModal(inputmodal.New(inputmodal.Config{
			Title:       "Veuillez saisir le texte",
			Placeholder: "Saisissez votre mot",
			Value:       m.word.Text,
			SubmitHelp:  "valider le texte",
			OnSubmit:    newText,
		}))
	case newTextMsg:
		return m.handleNewTextMsg(msg)
	case deleteWordMsg:
		// TODO: delete the word once the storage supports it
		slog.Debug("handling delete word message", "wordID", m.wordID)
		return m, nil
	}

	// Components ignore keys when they don't have focus
	var translationsCmd, editCmd, switchKindCmd, deleteCmd tea.Cmd
	m.translations, translationsCmd = m.translations.Update(msg)
	m.editButton, editCmd = m.editButton.Update(msg)
	m.switchKindButton, switchKindCmd = m.switchKindButton.Update(msg)
	m.deleteButton, deleteCmd = m.deleteButton.Update(msg)

	return m, tea.Batch(translationsCmd, editCmd, switchKindCmd, deleteCmd)
}

// applyFocus propagates m.focus to the components.
func (m Model) applyFocus() Model {
	m.translations.Focused = m.focus == focusTranslations
	m.editButton.Focused = m.focus == focusEditWord
	m.switchKindButton.Focused = m.focus == focusSwitchKind
	m.deleteButton.Focused = m.focus == focusDeleteWord
	return m
}

func (m Model) handleNewTextMsg(msg newTextMsg) (Model, tea.Cmd) {
	text := strings.TrimSpace(msg.text)
	if text == "" || text == m.word.Text {
		return m, nil
	}

	m.word.Text = text
	return m, m.saveWordDetails
}

func (m Model) computeLayout() Model {

	// Buttons are sized to fit their content
	for _, b := range []*button.Model{
		&m.switchKindButton,
		&m.editButton,
		&m.deleteButton,
	} {
		b.Width = lipgloss.Width(b.Content) + b.Style.GetHorizontalFrameSize()
		b.Height = b.Style.GetVerticalFrameSize() + 1
	}

	// The header row is as tall as its buttons, the delete row as its button
	headerHeight := m.editButton.Height
	m.translations = m.translations.SetSize(
		m.width,
		m.height-headerHeight-m.deleteButton.Height,
	)

	return m
}

func (m Model) View() tea.View {
	gap := strings.Repeat(" ", buttonGap)
	buttons := lipgloss.JoinHorizontal(
		lipgloss.Center,
		m.switchKindButton.View(), gap,
		m.editButton.View(),
	)

	// The word takes the remaining width, buttons are pushed to the right
	word := lipgloss.Place(
		max(0, m.width-lipgloss.Width(buttons)), lipgloss.Height(buttons),
		lipgloss.Left, lipgloss.Center,
		wordStyle.Render(m.word.Text),
	)

	content := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Center, word, buttons),
		m.translations.View().Content,
		m.deleteButton.View(),
	)

	return tea.NewView(lipgloss.Place(
		m.width, m.height,
		lipgloss.Left, lipgloss.Top,
		content,
	))
}

func switchKindLabel(kind string) string {
	return "<" + kind + ">"
}
