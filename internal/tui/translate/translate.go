package translate

import (
	"fmt"
	"log/slog"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"
	"github.com/wolf29f/hushtongue/internal/config"
	"github.com/wolf29f/hushtongue/internal/services"
	"github.com/wolf29f/hushtongue/internal/services/lexicon"
	"github.com/wolf29f/hushtongue/internal/services/storage"
	"github.com/wolf29f/hushtongue/internal/tui"
	"github.com/wolf29f/hushtongue/internal/tui/components/button"
	"github.com/wolf29f/hushtongue/internal/tui/components/confirmmodal"
	"github.com/wolf29f/hushtongue/internal/tui/translate/components/wordchoice"
	"github.com/wolf29f/hushtongue/internal/tui/translate/components/wordinput"
	"github.com/wolf29f/hushtongue/internal/tui/translationpicker"
)

const buttonGap = 1

type focus int

const (
	focusInput focus = iota
	focusChoice
	focusDirection
	focusMemorize // only when the input has unknown words
	focusOverflowed
)

// Model is the page translating a text, word by word, from one language to
// the other.
type Model struct {
	services *services.Services

	// State
	from    string // language typed in, storage.LangSource or storage.LangCon
	lexicon *lexicon.Lexicon
	focus   focus
	status  string // feedback shown until the next key

	// Components
	input           wordinput.Model
	choice          wordchoice.Model
	directionButton button.Model
	memorizeButton  button.Model

	// UI stuff
	width, height int
}

// NewModel creates the translation page, typing in the language the user
// knows the least.
func NewModel(services *services.Services) Model {
	from := storage.LangCon
	if config.IsForGM {
		from = storage.LangSource
	}

	m := Model{
		services: services,
		from:     from,
		lexicon:  &lexicon.Lexicon{},
		focus:    focusInput,

		input:  wordinput.New(),
		choice: wordchoice.New(),
		directionButton: button.New(directionLabel(from),
			func() tea.Msg { return switchDirectionPressedMsg{} },
		),
		memorizeButton: button.New("Mémoriser les nouveaux mots",
			func() tea.Msg { return memorizePressedMsg{} },
		),
	}
	m = m.applyFocus()
	m = m.computeLayout()

	return m
}

// Init loads the lexicon, so that changes made on other pages show up when
// the page becomes active again.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.load,
		m.setKeyMap(),
	)
}

func (m Model) setKeyMap() tea.Cmd {
	k := keys
	k.focus = m.focus
	return tui.SetKeyMap(k)
}

func (m Model) load() tea.Msg {
	l, err := lexicon.Load(m.services.Storage, m.from)
	if err != nil {
		slog.Error("unable to load lexicon", "error", err)
		return tui.ShowError("Impossible de charger le dictionnaire.")()
	}
	return lexiconLoadedMsg{from: m.from, lexicon: l}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.computeLayout()
		return m, nil
	case lexiconLoadedMsg:
		return m.handleLexiconLoadedMsg(msg)
	case tea.PasteMsg:
		return m.paste(msg.Content), nil
	case clipboardReadMsg:
		return m.paste(msg.text), nil
	case copiedMsg:
		m.status = "Traduction copiée dans le presse-papiers."
		return m, nil
	case switchDirectionPressedMsg:
		return m.handleSwitchDirectionPressedMsg()
	case switchDirectionConfirmedMsg:
		return m.switchDirection()
	case memorizePressedMsg:
		return m, m.memorize()
	case wordchoice.AddPressedMsg:
		return m, m.addTranslation(msg.Word)
	case wordchoice.DeletePressedMsg:
		return m, tui.PushModal(confirmmodal.New(confirmmodal.Config{
			Title:        fmt.Sprintf("Supprimer le lien avec « %s » ?", msg.Translation.Word.Text),
			ConfirmLabel: "Supprimer le lien",
			OnConfirm:    func() tea.Msg { return translationDeleteConfirmedMsg{id: msg.Translation.ID} },
		}))
	case translationDeleteConfirmedMsg:
		return m, m.deleteTranslation(msg.id)
	case tea.KeyPressMsg:
		m.status = ""
		switch {
		case key.Matches(msg, keys.FocusNext):
			return m.moveFocus(1)
		case key.Matches(msg, keys.FocusPrev):
			return m.moveFocus(-1)
		case key.Matches(msg, keys.Copy):
			m = m.commit()
			if len(m.input.Words()) == 0 {
				return m, nil
			}
			return m, m.copy()
		case key.Matches(msg, keys.Paste):
			return m, readClipboard
		case m.focus == focusInput && key.Matches(msg, keys.QuitInput),
			m.focus != focusInput && key.Matches(msg, keys.Quit):
			return m, m.quit()
		}
	}

	// Components ignore keys when they don't have focus
	var choiceCmd, directionCmd, memorizeCmd tea.Cmd
	m.input = m.input.Update(msg)
	m.choice = m.choice.SetWords(m.input.Words())
	m.choice, choiceCmd = m.choice.Update(msg)
	m.directionButton, directionCmd = m.directionButton.Update(msg)
	m.memorizeButton, memorizeCmd = m.memorizeButton.Update(msg)

	return m, tea.Batch(choiceCmd, directionCmd, memorizeCmd)
}

func (m Model) commit() Model {
	m.input = m.input.Commit()
	m.choice = m.choice.SetWords(m.input.Words())
	return m
}

func (m Model) paste(text string) Model {
	m.input = m.input.Paste(text)
	m.choice = m.choice.SetWords(m.input.Words())
	return m
}

func (m Model) handleLexiconLoadedMsg(msg lexiconLoadedMsg) (Model, tea.Cmd) {
	// A load started before a direction switch is stale
	if msg.from != m.from {
		return m, nil
	}

	m.lexicon = msg.lexicon
	m.input = m.input.SetLexicon(m.lexicon)
	m.choice = m.choice.SetLexicon(m.lexicon)

	if m.focus == focusMemorize && len(m.listUnknownWords()) == 0 {
		m.focus = focusDirection
		m = m.applyFocus()
		return m, m.setKeyMap()
	}
	return m, nil
}

func (m Model) quit() tea.Cmd {
	if m.input.IsEmpty() {
		return tui.PopPage
	}
	return tui.PushModal(confirmmodal.New(confirmmodal.Config{
		Title:        "Quitter la traduction ?\nLa saisie sera effacée.",
		ConfirmLabel: "Quitter",
		OnConfirm:    tui.PopPage,
	}))
}

func (m Model) handleSwitchDirectionPressedMsg() (Model, tea.Cmd) {
	if m.input.IsEmpty() {
		return m.switchDirection()
	}
	return m, tui.PushModal(confirmmodal.New(confirmmodal.Config{
		Title:        "Changer de direction ?\nLa saisie sera effacée.",
		ConfirmLabel: "Changer",
		OnConfirm:    func() tea.Msg { return switchDirectionConfirmedMsg{} },
	}))
}

func (m Model) switchDirection() (Model, tea.Cmd) {
	if m.from == storage.LangSource {
		m.from = storage.LangCon
	} else {
		m.from = storage.LangSource
	}
	m.directionButton.Content = directionLabel(m.from)
	m.lexicon = &lexicon.Lexicon{}
	m.input = m.input.SetLexicon(m.lexicon).Reset()
	m.choice = m.choice.SetLexicon(m.lexicon).SetWords(nil)
	m = m.computeLayout()
	return m, m.load
}

func (m Model) listUnknownWords() []string {
	var words []string
	seen := make(map[string]bool)
	for _, word := range m.input.Words() {
		clean := lexicon.Clean(word)
		normalized := storage.Normalize(clean)
		if clean == "" || seen[normalized] || m.lexicon.Lookup(word).Status != lexicon.StatusUnknown {
			continue
		}
		seen[normalized] = true
		words = append(words, clean)
	}
	return words
}

func (m Model) memorize() tea.Cmd {
	words := m.listUnknownWords()
	return func() tea.Msg {
		for _, word := range words {
			if err := m.services.Storage.AddWord(m.from, word); err != nil {
				slog.Error("unable to add word", "word", word, "error", err)
				return tea.Batch(tui.ShowError(fmt.Sprintf("Impossible d'ajouter le mot « %s ».", word)), m.load)()
			}
		}
		return m.load()
	}
}

// addTranslation opens the translation picker for word, storing it first
// when it is unknown.
func (m Model) addTranslation(word string) tea.Cmd {
	word = lexicon.Clean(word)
	if word == "" {
		return nil
	}
	if entry := m.lexicon.Lookup(word); entry.Status != lexicon.StatusUnknown {
		return tui.PushPage(translationpicker.NewModel(entry.Word.ID, m.services))
	}

	return func() tea.Msg {
		if err := m.services.Storage.AddWord(m.from, word); err != nil {
			slog.Error("unable to add word", "word", word, "error", err)
			return tui.ShowError(fmt.Sprintf("Impossible d'ajouter le mot « %s ».", word))()
		}
		l, err := lexicon.Load(m.services.Storage, m.from)
		if err != nil {
			slog.Error("unable to load lexicon", "error", err)
			return tui.ShowError("Impossible de charger le dictionnaire.")()
		}
		return tui.PushPage(translationpicker.NewModel(l.Lookup(word).Word.ID, m.services))()
	}
}

func (m Model) deleteTranslation(id int) tea.Cmd {
	return func() tea.Msg {
		if err := m.services.Storage.DeleteTranslation(id); err != nil {
			slog.Error("unable to delete translation", "error", err)
			return tea.Batch(tui.ShowError("Impossible de supprimer la traduction."), m.load)()
		}
		return m.load()
	}
}

func (m Model) copy() tea.Cmd {
	text := m.choice.Translation()
	return func() tea.Msg {
		if err := clipboard.WriteAll(text); err != nil {
			slog.Error("unable to write clipboard", "error", err)
			return tui.ShowError("Impossible de copier la traduction.")()
		}
		return copiedMsg{}
	}
}

func readClipboard() tea.Msg {
	text, err := clipboard.ReadAll()
	if err != nil {
		slog.Error("unable to read clipboard", "error", err)
		return tui.ShowError("Impossible de lire le presse-papiers.")()
	}
	return clipboardReadMsg{text: text}
}

func (m Model) applyFocus() Model {
	m.input.Focused = m.focus == focusInput
	m.choice.Focused = m.focus == focusChoice
	m.directionButton.Focused = m.focus == focusDirection
	m.memorizeButton.Focused = m.focus == focusMemorize
	return m
}

// moveFocus commits the word being typed when leaving the input.
func (m Model) moveFocus(step focus) (Model, tea.Cmd) {
	if m.focus == focusInput {
		m = m.commit()
	}
	m.focus = (m.focus + focusOverflowed + step) % focusOverflowed
	if m.focus == focusMemorize && len(m.listUnknownWords()) == 0 {
		m.focus = (m.focus + focusOverflowed + step) % focusOverflowed
	}
	m = m.applyFocus()
	return m, m.setKeyMap()
}

func directionLabel(from string) string {
	if from == storage.LangSource {
		return activeLangStyle.Render(config.SourceLangLabel) + " → " + config.ConLangLabel
	}
	return activeLangStyle.Render(config.ConLangLabel) + " → " + config.SourceLangLabel
}

func (m Model) computeLayout() Model {
	m.input.Width = m.width
	m.choice.Width = m.width

	// Buttons are sized to fit their content
	for _, b := range []*button.Model{&m.directionButton, &m.memorizeButton} {
		b.Width = lipgloss.Width(b.Content) + b.Style.GetHorizontalFrameSize()
		b.Height = b.Style.GetVerticalFrameSize() + 1
	}
	return m
}

func (m Model) View() tea.View {
	memorizeButton := m.memorizeButton
	if len(m.listUnknownWords()) == 0 {
		memorizeButton.Style = disabledButtonStyle
	}
	buttons := lipgloss.JoinHorizontal(lipgloss.Center,
		m.directionButton.View(),
		strings.Repeat(" ", buttonGap),
		memorizeButton.View(),
	)

	parts := []string{buttons, m.input.View()}
	if suggestions := m.input.SuggestionsView(); suggestions != "" {
		parts = append(parts, suggestions)
	}
	parts = append(parts, m.choice.View())
	if m.status != "" {
		parts = append(parts, faintStyle.Render(m.status))
	}

	return tea.NewView(lipgloss.Place(
		m.width, m.height,
		lipgloss.Left, lipgloss.Top,
		lipgloss.JoinVertical(lipgloss.Left, parts...),
	))
}
