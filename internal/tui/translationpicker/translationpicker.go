package translationpicker

import (
	"fmt"
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
	"github.com/wolf29f/hushtongue/internal/tui/components/wordlist"
	"github.com/wolf29f/hushtongue/internal/tui/translationgenerator"
)

// buttonGap is the space between the buttons below the word list.
const buttonGap = 1

type focus int

const (
	focusWordList focus = iota
	focusNewWord
	focusGenerate // only for MJ, on a source word
	focusOverflowed
)

// Model is the page choosing a translation for a word, among the words of
// the other language.
type Model struct {
	services *services.Services

	// State
	wordID int
	word   storage.WordDetails
	focus  focus

	// Components
	wordList       wordlist.Model
	newWordButton  button.Model
	generateButton button.Model

	// UI stuff
	width, height int
}

// NewModel creates the page choosing a translation for the word wordID.
func NewModel(wordID int, services *services.Services) Model {
	m := Model{
		services: services,
		wordID:   wordID,
		focus:    focusWordList,

		wordList: wordlist.NewModel(nil),
		newWordButton: button.New("Nouveau mot",
			func() tea.Msg { return newWordPressedMsg{} },
		),
		generateButton: button.New("Générer",
			func() tea.Msg { return generatePressedMsg{} },
		),
	}
	m = m.applyFocus()
	m = m.computeLayout()

	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadWord,
		m.setKeyMap(),
	)
}

// setKeyMap announces the help keymap, which depends on the word.
func (m Model) setKeyMap() tea.Cmd {
	generate := keys.Generate
	generate.SetEnabled(m.canGenerate())
	return tui.SetKeyMap(m.wordList.KeyMapHelper(keys.Pick, keys.NewWord, generate))
}

func (m Model) loadWord() tea.Msg {
	word, err := m.services.Storage.GetWord(m.wordID)
	if err != nil {
		slog.Error("unable to get word details", "error", err)
		return tui.ShowError("Impossible de charger le mot.")()
	}
	return wordLoadedMsg{word: word}
}

// loadWords loads the words of the language opposite to the word's.
func (m Model) loadWords() tea.Msg {
	words, err := m.services.Storage.ListWords(m.targetLanguage())
	if err != nil {
		slog.Error("unable to get words", "error", err)
		return tui.ShowError("Impossible de charger les mots.")()
	}
	return wordsLoadedMsg{words: words}
}

func (m Model) addTranslation(targetID int) tea.Cmd {
	return func() tea.Msg {
		if err := m.services.Storage.AddTranslation(m.wordID, targetID); err != nil {
			slog.Error("unable to add translation", "error", err)
			return tui.ShowError("Impossible d'ajouter la traduction.")()
		}
		return tui.PopPage()
	}
}

func (m Model) addTranslationWord(text string) tea.Cmd {
	return func() tea.Msg {
		if err := m.services.Storage.AddTranslationWord(m.wordID, text); err != nil {
			slog.Error("unable to add translation word", "error", err)
			return tui.ShowError(fmt.Sprintf("Impossible d'ajouter la traduction « %s ».", text))()
		}
		return tui.PopPage()
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case wordLoadedMsg:
		m.word = msg.word
		m = m.computeLayout()
		return m, tea.Batch(m.loadWords, m.setKeyMap())
	case wordsLoadedMsg:
		var cmd tea.Cmd
		m.wordList, cmd = m.wordList.SetItems(msg.words)
		return m, cmd
	case wordlist.WordSelectedMsg:
		return m, m.addTranslation(msg.ID)
	case newWordPressedMsg:
		return m, tui.PushModal(inputmodal.New(inputmodal.Config{
			Title:       fmt.Sprintf("Nouveau mot en %s", targetLangLabel(m.targetLanguage())),
			Placeholder: "Saisissez un mot",
			SubmitHelp:  "ajouter la traduction",
			OnSubmit:    newWordSubmitted,
		}))
	case newWordSubmittedMsg:
		text := strings.TrimSpace(msg.text)
		if text == "" {
			return m, nil
		}
		return m, m.addTranslationWord(text)
	case generatePressedMsg:
		// Replaced rather than pushed: leaving the generator goes back to
		// the word, not to this picker
		return m, tui.ReplacePage(translationgenerator.NewModel(m.wordID, m.services))
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.computeLayout()
		return m, nil
	case tea.KeyPressMsg:
		if m.wordList.CapturesKey(msg) {
			break
		}
		switch {
		case key.Matches(msg, keys.FocusNext):
			return m.moveFocus(1), nil
		case key.Matches(msg, keys.FocusPrev):
			return m.moveFocus(-1), nil
		case key.Matches(msg, keys.Quit):
			return m, tui.PopPage
		case key.Matches(msg, keys.NewWord):
			return m, m.newWordButton.OnPress
		case m.canGenerate() && key.Matches(msg, keys.Generate):
			return m, m.generateButton.OnPress
		}
	}

	// Components ignore keys when they don't have focus
	var wordListCmd, newWordCmd, generateCmd tea.Cmd
	m.wordList, wordListCmd = m.wordList.Update(msg)
	m.newWordButton, newWordCmd = m.newWordButton.Update(msg)
	m.generateButton, generateCmd = m.generateButton.Update(msg)

	return m, tea.Batch(wordListCmd, newWordCmd, generateCmd)
}

// canGenerate reports whether a translation can be generated for the word.
func (m Model) canGenerate() bool {
	return config.IsForGM && m.word.Language == storage.LangSource
}

// targetLanguage returns the language opposite to the word's.
func (m Model) targetLanguage() string {
	if m.word.Language == storage.LangCon {
		return storage.LangSource
	}
	return storage.LangCon
}

func targetLangLabel(language string) string {
	if language == storage.LangSource {
		return config.SourceLangLabel
	}
	return config.ConLangLabel
}

// applyFocus propagates m.focus to the components.
func (m Model) applyFocus() Model {
	m.wordList.Focused = m.focus == focusWordList
	m.newWordButton.Focused = m.focus == focusNewWord
	m.generateButton.Focused = m.focus == focusGenerate
	return m
}

func (m Model) moveFocus(step focus) Model {
	m.focus = (m.focus + focusOverflowed + step) % focusOverflowed
	if m.focus == focusGenerate && !m.canGenerate() {
		m.focus = (m.focus + focusOverflowed + step) % focusOverflowed
	}
	return m.applyFocus()
}

func (m Model) computeLayout() Model {
	// Buttons are sized to fit their content
	for _, b := range []*button.Model{&m.newWordButton, &m.generateButton} {
		b.Width = lipgloss.Width(b.Content) + b.Style.GetHorizontalFrameSize()
		b.Height = b.Style.GetVerticalFrameSize() + 1
	}

	listHeight := m.height - lipgloss.Height(m.header()) - m.newWordButton.Height
	m.wordList = m.wordList.SetSize(m.width, listHeight)

	return m
}

func (m Model) header() string {
	return wordStyle.Render(m.word.Text) + kindStyle.Render("<"+m.word.Kind+">")
}

func (m Model) View() tea.View {
	buttons := []string{m.newWordButton.View()}
	if m.canGenerate() {
		buttons = append(buttons, strings.Repeat(" ", buttonGap), m.generateButton.View())
	}

	return tea.NewView(lipgloss.Place(
		m.width, m.height,
		lipgloss.Left, lipgloss.Top,
		lipgloss.JoinVertical(lipgloss.Left,
			m.header(),
			m.wordList.View().Content,
			lipgloss.JoinHorizontal(lipgloss.Center, buttons...),
		),
	))
}
