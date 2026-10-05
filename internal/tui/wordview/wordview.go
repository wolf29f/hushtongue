package wordview

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
	"github.com/wolf29f/hushtongue/internal/tui/components/confirmmodal"
	"github.com/wolf29f/hushtongue/internal/tui/components/inputmodal"
	"github.com/wolf29f/hushtongue/internal/tui/components/wordlist"
	"github.com/wolf29f/hushtongue/internal/tui/translationgenerator"
	"github.com/wolf29f/hushtongue/internal/tui/translationpicker"
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
	focusAddTranslation
	focusGenerateTranslation // only for MJ, on a source word
	focusDeleteWord
	focusOverflowed
)

type Model struct {
	services *services.Services

	// State
	loading bool
	wordID  int
	word    storage.WordDetails
	links   []storage.Translation
	focus   focus

	// Components
	translations              wordlist.Model
	editButton                button.Model
	switchKindButton          button.Model
	addTranslationButton      button.Model
	generateTranslationButton button.Model
	deleteButton              button.Model

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
			func() tea.Msg { return editPressedMsg{} },
		),
		switchKindButton: button.New(
			switchKindLabel("root"),
			func() tea.Msg { return switchKindPressedMsg{} },
		),
		addTranslationButton: button.New("Ajouter une traduction",
			func() tea.Msg { return addTranslationPressedMsg{} },
		),
		generateTranslationButton: button.New("Générer une traduction",
			func() tea.Msg { return generateTranslationPressedMsg{} },
		),
		deleteButton: button.New("Supprimer le mot", tui.PushModal(confirmmodal.New(confirmmodal.Config{
			Title:        "Supprimer ce mot du dictionnaire ?\nSes liens de traduction seront aussi supprimés.",
			ConfirmLabel: "Supprimer le mot",
			OnConfirm:    func() tea.Msg { return deleteConfirmedMsg{} },
		}))),
	}

	m.translations = wordlist.NewModel(nil)
	m = m.applyFocus()
	m = m.computeLayout()

	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadWordDetails,
		m.loadTranslations,
		m.setKeyMap(),
	)
}

// setKeyMap announces the help keymap, which depends on the focus and on
// the word.
func (m Model) setKeyMap() tea.Cmd {
	k := keys
	k.GenerateTranslation.SetEnabled(m.canGenerate())
	if m.focus == focusTranslations {
		return tui.SetKeyMap(m.translations.KeyMapHelper(
			k.FocusNext, k.OpenTranslation, k.AddTranslation, k.GenerateTranslation, k.DeleteTranslation,
		))
	}
	return tui.SetKeyMap(k)
}

func (m Model) loadWordDetails() tea.Msg {
	wordDetails, err := m.services.Storage.GetWord(m.wordID)
	if err != nil {
		slog.Error("unable to get word details", "error", err)
		return tea.Batch(
			tui.ShowError("Impossible de charger le mot."),
			func() tea.Msg { return wordLoadedMsg{Word: storage.WordDetails{}} },
		)()
	}

	return wordLoadedMsg{Word: wordDetails}
}

func (m Model) loadTranslations() tea.Msg {
	translations, err := m.services.Storage.ListTranslations(m.wordID)
	if err != nil {
		slog.Error("unable to get translations", "error", err)
		return tui.ShowError("Impossible de charger les traductions.")()
	}

	return translationsLoadedMsg{translations: translations}
}

func (m Model) deleteTranslation(id int) tea.Cmd {
	return func() tea.Msg {
		if err := m.services.Storage.DeleteTranslation(id); err != nil {
			slog.Error("unable to delete translation", "error", err)
			return tea.Batch(tui.ShowError("Impossible de supprimer la traduction."), m.loadTranslations)()
		}

		return m.loadTranslations()
	}
}

func (m Model) saveWordDetails() tea.Msg {
	wordDetails, err := m.services.Storage.SaveWord(m.word)
	if err != nil {
		slog.Error("unable to save word details", "error", err)
		// Reload the stored word to discard the rejected edit
		return tea.Batch(tui.ShowError("Impossible d'enregistrer le mot."), m.loadWordDetails)()
	}

	return wordLoadedMsg{Word: wordDetails}
}

func (m Model) changeKind(kind string) tea.Cmd {
	return func() tea.Msg {
		wordDetails, err := m.services.Storage.ChangeWordKind(m.wordID, kind)
		if err != nil {
			slog.Error("unable to change word kind", "error", err)
			return tea.Batch(tui.ShowError("Impossible de changer le type du mot."), m.loadWordDetails)()
		}

		return wordLoadedMsg{Word: wordDetails}
	}
}

func (m Model) deleteWord() tea.Msg {
	if err := m.services.Storage.DeleteWord(m.wordID); err != nil {
		slog.Error("unable to delete word", "error", err)
		return tui.ShowError("Impossible de supprimer le mot.")()
	}

	return wordDeletedMsg{}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case wordLoadedMsg:
		m.word = msg.Word
		m.loading = false
		m.switchKindButton.Content = switchKindLabel(m.word.Kind)
		m = m.computeLayout()
		return m, m.setKeyMap()
	case translationsLoadedMsg:
		return m.handleTranslationsLoadedMsg(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.computeLayout()
		return m, nil
	case tea.KeyPressMsg:
		if m.translations.CapturesKey(msg) {
			break
		}
		switch {
		case key.Matches(msg, keys.FocusNext):
			return m.moveFocus(1)
		case key.Matches(msg, keys.FocusPrev):
			return m.moveFocus(-1)
		case key.Matches(msg, keys.Quit):
			return m, tui.PopPage
		case key.Matches(msg, keys.AddTranslation):
			return m, m.pushTranslationPicker()
		case m.canGenerate() && key.Matches(msg, keys.GenerateTranslation):
			return m, m.generateTranslationButton.OnPress
		case m.focus == focusTranslations && key.Matches(msg, keys.DeleteTranslation):
			return m.handleDeleteTranslationPressed()
		}
	case editPressedMsg:
		return m, tui.PushModal(inputmodal.New(inputmodal.Config{
			Title:       "Veuillez saisir le texte",
			Placeholder: "Saisissez votre mot",
			Value:       m.word.Text,
			SubmitHelp:  "valider le texte",
			OnSubmit:    textSubmitted,
		}))
	case addTranslationPressedMsg:
		return m, m.pushTranslationPicker()
	case generateTranslationPressedMsg:
		return m, tui.PushPage(translationgenerator.NewModel(m.wordID, m.services))
	case textSubmittedMsg:
		return m.handleTextSubmittedMsg(msg)
	case switchKindPressedMsg:
		return m.handleSwitchKindPressedMsg()
	case kindChangeConfirmedMsg:
		return m, m.changeKind(msg.kind)
	case translationDeleteConfirmedMsg:
		return m, m.deleteTranslation(msg.id)
	case wordlist.WordSelectedMsg:
		return m, tui.PushPage(NewModel(msg.ID, m.services))
	case deleteConfirmedMsg:
		return m, m.deleteWord
	case wordDeletedMsg:
		return m, tui.PopPage
	}

	// Components ignore keys when they don't have focus
	var translationsCmd, editCmd, switchKindCmd, addTranslationCmd, generateTranslationCmd, deleteCmd tea.Cmd
	m.translations, translationsCmd = m.translations.Update(msg)
	m.editButton, editCmd = m.editButton.Update(msg)
	m.switchKindButton, switchKindCmd = m.switchKindButton.Update(msg)
	m.addTranslationButton, addTranslationCmd = m.addTranslationButton.Update(msg)
	m.generateTranslationButton, generateTranslationCmd = m.generateTranslationButton.Update(msg)
	m.deleteButton, deleteCmd = m.deleteButton.Update(msg)

	return m, tea.Batch(translationsCmd, editCmd, switchKindCmd, addTranslationCmd, generateTranslationCmd, deleteCmd)
}

// applyFocus propagates m.focus to the components.
func (m Model) applyFocus() Model {
	m.translations.Focused = m.focus == focusTranslations
	m.editButton.Focused = m.focus == focusEditWord
	m.switchKindButton.Focused = m.focus == focusSwitchKind
	m.addTranslationButton.Focused = m.focus == focusAddTranslation
	m.generateTranslationButton.Focused = m.focus == focusGenerateTranslation
	m.deleteButton.Focused = m.focus == focusDeleteWord
	return m
}

func (m Model) moveFocus(step focus) (Model, tea.Cmd) {
	m.focus = (m.focus + focusOverflowed + step) % focusOverflowed
	if m.focus == focusSwitchKind && !config.IsForGM ||
		m.focus == focusGenerateTranslation && !m.canGenerate() {
		m.focus = (m.focus + focusOverflowed + step) % focusOverflowed
	}
	m = m.applyFocus()
	return m, m.setKeyMap()
}

// canGenerate reports whether a translation can be generated for the word.
func (m Model) canGenerate() bool {
	return config.IsForGM && m.word.Language == storage.LangSource
}

func (m Model) pushTranslationPicker() tea.Cmd {
	return tui.PushPage(translationpicker.NewModel(m.wordID, m.services))
}

func (m Model) handleSwitchKindPressedMsg() (Model, tea.Cmd) {
	kind := nextKind(m.word.Kind)

	count := len(m.translations.Items())
	if count == 0 {
		return m, m.changeKind(kind)
	}

	return m, tui.PushModal(confirmmodal.New(confirmmodal.Config{
		Title: fmt.Sprintf(
			"Passer en %s ?\nLes %d liens de traduction seront supprimés.",
			switchKindLabel(kind), count,
		),
		ConfirmLabel: "Changer",
		OnConfirm:    func() tea.Msg { return kindChangeConfirmedMsg{kind: kind} },
	}))
}

func (m Model) handleTranslationsLoadedMsg(msg translationsLoadedMsg) (Model, tea.Cmd) {
	m.links = msg.translations

	words := make([]storage.Word, len(m.links))
	for i, link := range m.links {
		words[i] = link.Word
	}

	var cmd tea.Cmd
	m.translations, cmd = m.translations.SetItems(words)
	return m, cmd
}

func (m Model) handleDeleteTranslationPressed() (Model, tea.Cmd) {
	if _, ok := m.translations.SelectedWord(); !ok {
		return m, nil
	}

	// The list items are built from m.links, in the same order
	link := m.links[m.translations.GlobalIndex()]
	return m, tui.PushModal(confirmmodal.New(confirmmodal.Config{
		Title:        fmt.Sprintf("Supprimer le lien avec « %s » ?", link.Word.Text),
		ConfirmLabel: "Supprimer le lien",
		OnConfirm:    func() tea.Msg { return translationDeleteConfirmedMsg{id: link.ID} },
	}))
}

func (m Model) handleTextSubmittedMsg(msg textSubmittedMsg) (Model, tea.Cmd) {
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
		&m.addTranslationButton,
		&m.generateTranslationButton,
		&m.deleteButton,
	} {
		b.Width = lipgloss.Width(b.Content) + b.Style.GetHorizontalFrameSize()
		b.Height = b.Style.GetVerticalFrameSize() + 1
	}

	// The header and footer rows are as tall as their buttons
	headerHeight := m.editButton.Height
	footerHeight := m.deleteButton.Height
	m.translations = m.translations.SetSize(
		m.width,
		m.height-headerHeight-footerHeight,
	)

	return m
}

func (m Model) View() tea.View {
	buttons := m.editButton.View()
	if config.IsForGM {
		buttons = lipgloss.JoinHorizontal(
			lipgloss.Center,
			m.switchKindButton.View(), strings.Repeat(" ", buttonGap),
			buttons,
		)
	}

	// The word takes the remaining width, buttons are pushed to the right
	word := lipgloss.Place(
		max(0, m.width-lipgloss.Width(buttons)), lipgloss.Height(buttons),
		lipgloss.Left, lipgloss.Center,
		wordStyle.Render(m.word.Text),
	)

	footerButtons := []string{m.addTranslationButton.View(), strings.Repeat(" ", buttonGap)}
	if m.canGenerate() {
		footerButtons = append(footerButtons, m.generateTranslationButton.View(), strings.Repeat(" ", buttonGap))
	}
	footerButtons = append(footerButtons, m.deleteButton.View())

	content := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Center, word, buttons),
		m.translations.View().Content,
		lipgloss.JoinHorizontal(lipgloss.Center, footerButtons...),
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

func nextKind(kind string) string {
	switch kind {
	case "root":
		return "prefix"
	case "prefix":
		return "suffix"
	default:
		return "root"
	}
}
