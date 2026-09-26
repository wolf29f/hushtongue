package dictionary

import (
	"log/slog"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/wolf29f/hushtongue/internal/services"
	"github.com/wolf29f/hushtongue/internal/services/storage"
	"github.com/wolf29f/hushtongue/internal/tui"
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

type focus int

const (
	focusWordList focus = iota
	focusAddWord
	focusOverflowed
)

type Model struct {
	services *services.Services

	// State
	language string // con or original
	focus    focus

	// Components
	wordList        wordlist.Model
	translationList tea.Model // list translations of the selected word, add/remove/generate(if origin) a translation
	edit            tea.Model // edit a word or translation, type (prefix/root/suffix)

	// UI stuff
	width, height int

	addButtonHeight, addButtonWidth int
}

func NewModel(services *services.Services) Model {

	m := Model{
		services: services,

		language: "source", // "source" or "con"
		focus:    0,

		translationList: nil,
		edit:            nil,
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
		case key.Matches(msg, keys.FocusPrev):
			m.focus = (m.focus + focusOverflowed - 1) % focusOverflowed
		case key.Matches(msg, keys.Quit):
			return m, tui.PopPage
		}
	case addmodal.NewWordMsg:
		// Handle the new word message here, e.g., update the word list
		return m.handleNewWordMsg(msg)
	}
	switch m.focus {
	case focusWordList:
		var cmd tea.Cmd
		m.wordList, cmd = m.wordList.Update(msg)
		cmds = append(cmds, cmd)
	case focusAddWord:
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			switch {
			case key.Matches(msg, keys.Enter):
				cmds = append(cmds,
					tui.PushModal(addmodal.NewModel()),
				)
			}
		}
	}

	return m, tea.Batch(cmds...)
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
	} else {
		m.wordList.SetItems(wordList)
	}

	return m, nil
}

func (m Model) computeLayout() Model {

	m.addButtonHeight = defaultBoxStyle.GetVerticalFrameSize() + 1
	m.addButtonWidth = m.width/3 - defaultBoxStyle.GetHorizontalFrameSize()

	m.wordList.SetWidth(m.width/3 - defaultBoxStyle.GetHorizontalFrameSize())
	m.wordList.SetHeight(m.height - defaultBoxStyle.GetVerticalFrameSize() - m.addButtonHeight)

	return m
}

func (m Model) View() tea.View {

	var addButtonRender string
	var wordListRender string

	switch m.focus {
	case focusWordList:
		addButtonRender = defaultBoxStyle.
			Render(lipgloss.PlaceHorizontal(m.addButtonWidth, lipgloss.Center, "Ajouter"))
		wordListRender = defaultBoxStyleFocus.
			Render(m.wordList.View().Content)
	case focusAddWord:
		addButtonRender = defaultBoxStyleFocus.
			Render(lipgloss.PlaceHorizontal(m.addButtonWidth, lipgloss.Center, "Ajouter"))
		wordListRender = defaultBoxStyle.
			Render(m.wordList.View().Content)
	}

	leftColumn := lipgloss.JoinVertical(lipgloss.Top, addButtonRender, wordListRender)

	horizontalContent := lipgloss.JoinHorizontal(lipgloss.Center, leftColumn, "->")

	return tea.NewView(lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		horizontalContent,
	))
}
