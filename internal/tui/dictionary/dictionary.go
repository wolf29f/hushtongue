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
	"github.com/wolf29f/hushtongue/internal/tui/components/inputmodal"
	"github.com/wolf29f/hushtongue/internal/tui/dictionary/components/wordlist"
	"github.com/wolf29f/hushtongue/internal/tui/wordview"
)

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

		addButton: button.New("Ajouter", tui.PushModal(inputmodal.New(inputmodal.Config{
			Title:       "Veuillez saisir votre mot",
			Placeholder: "Saisissez un mot",
			SubmitHelp:  "valider le mot",
			OnSubmit:    newWord,
		}))),
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
				ID:   -1,
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
	return tea.Batch(
		m.loadWords,
		tui.SetKeyMap(m.wordList.KeyMapHelper()),
	)
}

// loadWords reloads the word list, so that changes made on other pages
// show up when the dictionary becomes active again.
func (m Model) loadWords() tea.Msg {
	words, err := m.services.Storage.ListWords(m.language)
	if err != nil {
		slog.Error("unable to get words", "error", err)
		return nil
	}
	return wordsLoadedMsg{words: words}
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
	case wordsLoadedMsg:
		var cmd tea.Cmd
		m.wordList, cmd = m.wordList.SetItems(msg.words)
		return m, cmd
	case newWordMsg:
		return m.handleNewWordMsg(msg)
	case switchLangMsg:
		return m.handleLangSwitch()
	case wordlist.WordSelectedMsg:
		return m.handleWordSelectedMsg(msg)
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

func (m Model) handleNewWordMsg(msg newWordMsg) (tea.Model, tea.Cmd) {
	slog.Debug("handling new word message", "word", msg.word)

	if err := m.services.Storage.AddWord(m.language, msg.word); err != nil {
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

func (m Model) handleWordSelectedMsg(msg wordlist.WordSelectedMsg) (tea.Model, tea.Cmd) {
	slog.Debug("handling word selected message", "wordID", msg.ID)

	return m, tui.PushPage(wordview.NewModel(msg.ID, m.services))
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
