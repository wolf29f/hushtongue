package dictionary

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
L'ui
- a gauche une liste des mots
	- haut/bas pour choisir un mot
	- ctrl+a pour ajouter (modale)
	- ctrl+f pour rechercher/filtrer
	- ctrl+d pour supprimer un mot (modale de confirmation)
- au milieux, une flecher vers la droite
- a droite une list de traductions
- sur selection d'un mot:
	- ouverture d'une page dédiée:
	- mot (éditable)
	- type (éditable, si MJ)
	- traductions (list, add/remove/generate if original & MJ)
	- ctrl+s pour sauvegarder les modifications
	- bouton pour supprimer + modale de confirmation (prévoir une modale générique avec configuration du msg de validation/annulation)
*/

const (
	langSource = "source"
	langCon    = "con"
)

// minButtonGap is the minimum space between the buttons above the word list.
const minButtonGap = 1

type focus int

const (
	focusWordList focus = iota
	focusAddWord
	focusSwitchLang
	focusOverflowed
)

type Model struct {
	services *services.Services

	// State
	language string // langSource or langCon
	focus    focus

	// Components
	wordList         wordlist.Model
	addButton        button.Model
	switchLangButton button.Model

	// UI stuff
	width, height int
}

func NewModel(services *services.Services) Model {

	m := Model{
		services: services,

		language: langSource,
		focus:    focusWordList,

		addButton: button.New("Ajouter", tui.PushModal(addmodal.NewModel())),
		switchLangButton: button.New(
			switchLangLabel(langSource),
			func() tea.Msg { return switchLangMsg{} },
		),
	}

	wordList, err := services.Storage.ListWords(m.language)
	if err != nil {
		slog.Error("unable to get words", "error", err)
		wordList = []storage.Word{
			{
				ID:   "NaN",
				Text: "Impossible de charger le dictionnaire, verifiez les logs",
			},
		}
	}
	m.wordList = wordlist.NewModel(wordList)
	m = m.applyFocus()
	m = m.computeLayout()

	return m
}

func (m Model) Init() tea.Cmd {
	return tui.SetKeyMap(m.wordList.KeyMapHelper())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, 0)

	switch msg := msg.(type) {
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
		return m.handleNewWordMsg(msg)
	case switchLangMsg:
		return m.handleLangSwitch()
	}
	switch m.focus {
	case focusWordList:
		var cmd tea.Cmd
		m.wordList, cmd = m.wordList.Update(msg)
		cmds = append(cmds, cmd)
	case focusAddWord:
		var cmd tea.Cmd
		m.addButton, cmd = m.addButton.Update(msg)
		cmds = append(cmds, cmd)
	case focusSwitchLang:
		var cmd tea.Cmd
		m.switchLangButton, cmd = m.switchLangButton.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// applyFocus propagates m.focus to the components.
func (m Model) applyFocus() Model {
	m.wordList.Focused = m.focus == focusWordList
	m.addButton.Focused = m.focus == focusAddWord
	m.switchLangButton.Focused = m.focus == focusSwitchLang
	return m
}

func (m Model) handleNewWordMsg(msg addmodal.NewWordMsg) (tea.Model, tea.Cmd) {
	slog.Debug("handling new word message", "word", msg.Word)

	if err := m.services.Storage.AddWord(m.language, msg.Word); err != nil {
		slog.Error("unable to add word", "error", err)
	}

	// Refresh the word list after adding a new word
	wordList, err := m.services.Storage.ListWords(m.language)
	if err != nil {
		slog.Error("unable to get words", "error", err)
		return m, nil
	}

	var cmd tea.Cmd
	m.wordList, cmd = m.wordList.SetItems(wordList)
	return m, cmd
}

func (m Model) handleLangSwitch() (tea.Model, tea.Cmd) {
	slog.Debug("handling language switch message")

	if m.language == langSource {
		m.language = langCon
	} else {
		m.language = langSource
	}
	m.switchLangButton.Content = switchLangLabel(m.language)

	// Refresh the word list after switching the language
	wordList, err := m.services.Storage.ListWords(m.language)
	if err != nil {
		slog.Error("unable to get words", "error", err)
		return m, nil
	}

	var cmd tea.Cmd
	m.wordList, cmd = m.wordList.Reset().SetItems(wordList)
	return m, cmd
}

// switchLangLabel renders "source/con" with the active language highlighted.
func switchLangLabel(lang string) string {
	if lang == langSource {
		return activeLangStyle.Render(config.SourceLangLabel) + "/" + config.ConLangLabel
	}
	return config.SourceLangLabel + "/" + activeLangStyle.Render(config.ConLangLabel)
}

func (m Model) computeLayout() Model {

	leftWidth := m.width / 3
	// Buttons share the width equally, the gap absorbs the remainder
	buttonWidth := (leftWidth - minButtonGap) / 2

	m.addButton.Width = buttonWidth
	m.addButton.Height = m.addButton.Style.GetVerticalFrameSize() + 1

	m.switchLangButton.Width = buttonWidth
	m.switchLangButton.Height = m.switchLangButton.Style.GetVerticalFrameSize() + 1

	m.wordList = m.wordList.SetSize(leftWidth, m.height-m.addButton.Height)

	return m
}

func (m Model) View() tea.View {
	gap := m.width/3 - m.addButton.Width - m.switchLangButton.Width

	leftColumn := lipgloss.JoinVertical(lipgloss.Top,
		lipgloss.JoinHorizontal(
			lipgloss.Center,
			m.addButton.View(),
			strings.Repeat(" ", max(0, gap)),
			m.switchLangButton.View(),
		),
		m.wordList.View().Content,
	)

	horizontalContent := lipgloss.JoinHorizontal(lipgloss.Center, leftColumn, "->")

	return tea.NewView(lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		horizontalContent,
	))
}
