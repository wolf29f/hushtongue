package wordview

import (
	"log/slog"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/wolf29f/hushtongue/internal/config"
	"github.com/wolf29f/hushtongue/internal/services"
	"github.com/wolf29f/hushtongue/internal/services/storage"
	"github.com/wolf29f/hushtongue/internal/tui"
	"github.com/wolf29f/hushtongue/internal/tui/components/button"
	"github.com/wolf29f/hushtongue/internal/tui/dictionary/components/addmodal"
	"github.com/wolf29f/hushtongue/internal/tui/dictionary/components/wordlist"
)

/*
- sur selection d'un mot:
	- ouverture d'une page dédiée:
	- mot (éditable)
	- type (éditable, si MJ)
	- traductions (list, add/remove/generate if original & MJ)
	- ctrl+s pour sauvegarder les modifications
	- bouton pour supprimer + modale de confirmation (prévoir une modale générique avec configuration du msg de validation/annulation)
*/

// buttonGap is the space between the buttons of the header row.
const buttonGap = 1

const (
	langSource = "source"
	langCon    = "con"
)

type focus int

const (
	focusSwitchLang focus = iota
	focusSwitchKind       // only for MJ
	focusEditWord
	focusTranslations
	focusDeleteWord
	focusOverflowed
)

type Model struct {
	services *services.Services

	// State
	loading  bool
	wordID   int
	word     storage.WordDetails
	language string // langSource or langCon
	focus    focus

	// Components
	translations     wordlist.Model
	editButton       button.Model
	switchKindButton button.Model
	switchLangButton button.Model
	deleteButton     button.Model

	// UI stuff
	width, height int
}

func NewModel(wordID int, services *services.Services) Model {

	m := Model{
		services: services,
		wordID:   wordID,
		loading:  true,

		editButton: button.New("Éditer", nil),
		switchKindButton: button.New(
			switchKindLabel("root"),
			func() tea.Msg { return switchKindMsg{} },
		),
		switchLangButton: button.New(
			switchLangLabel(langSource),
			func() tea.Msg { return switchLangMsg{} },
		),
		deleteButton: button.New("Supprimer", nil),
	}

	wordList, err := services.Storage.ListWords(m.language)
	if err != nil {
		slog.Error("unable to get words", "error", err)
		wordList = []storage.Word{
			{
				ID:   -1,
				Text: "Impossible de charger le dictionnaire, verifiez les logs",
			},
		}
	}
	m.translations = wordlist.NewModel(wordList)
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

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, 0)

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
			m = m.applyFocus()
		case key.Matches(msg, keys.FocusPrev):
			m.focus = (m.focus + focusOverflowed - 1) % focusOverflowed
			m = m.applyFocus()
		case key.Matches(msg, keys.Quit):
			return m, tui.PopPage
		}
	case addmodal.NewWordMsg:
		return m, nil
	case switchLangMsg:
		return m.handleLangSwitch()
	}

	switch m.focus {
	case focusTranslations:
		var cmd tea.Cmd
		m.translations, cmd = m.translations.Update(msg)
		cmds = append(cmds, cmd)
	case focusEditWord:
		var cmd tea.Cmd
		m.editButton, cmd = m.editButton.Update(msg)
		cmds = append(cmds, cmd)
	case focusSwitchLang:
		var cmd tea.Cmd
		m.switchLangButton, cmd = m.switchLangButton.Update(msg)
		cmds = append(cmds, cmd)
	case focusSwitchKind:
		var cmd tea.Cmd
		m.switchKindButton, cmd = m.switchKindButton.Update(msg)
		cmds = append(cmds, cmd)
	case focusDeleteWord:
		var cmd tea.Cmd
		m.deleteButton, cmd = m.deleteButton.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// applyFocus propagates m.focus to the components.
func (m Model) applyFocus() Model {
	m.translations.Focused = m.focus == focusTranslations
	m.editButton.Focused = m.focus == focusEditWord
	m.switchLangButton.Focused = m.focus == focusSwitchLang
	m.switchKindButton.Focused = m.focus == focusSwitchKind
	m.deleteButton.Focused = m.focus == focusDeleteWord
	return m
}

func (m Model) handleLangSwitch() (tea.Model, tea.Cmd) {
	slog.Debug("handling language switch message")

	if m.language == langSource {
		m.language = langCon
	} else {
		m.language = langSource
	}
	m.switchLangButton.Content = switchLangLabel(m.language)

	// TODO: upate word data
	// TODO: clear all translations
	// TODO: modal to warn user about switching language, that it would clear translations (only if translation exists)

	return m, nil
}

// switchLangLabel renders "source/con" with the active language highlighted.
func switchLangLabel(lang string) string {
	if lang == langSource {
		return activeLangStyle.Render(config.SourceLangLabel) + "/" + config.ConLangLabel
	}
	return config.SourceLangLabel + "/" + activeLangStyle.Render(config.ConLangLabel)
}

func (m Model) computeLayout() Model {

	// Buttons are sized to fit their content
	for _, b := range []*button.Model{
		&m.switchLangButton,
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
		m.switchLangButton.View(), gap,
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
