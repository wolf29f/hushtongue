package translate

import (
	"fmt"
	"log/slog"
	"strings"
	"unicode"

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
	"github.com/wolf29f/hushtongue/internal/tui/styles"
	"github.com/wolf29f/hushtongue/internal/tui/translationpicker"
)

const (
	maxSuggestions = 5
	buttonGap      = 1
	columnGap      = 2
)

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

	// Input: the committed words, and the word being typed
	words       []string
	typed       string
	suggestions []storage.Word
	more        bool // whether suggestions were left out
	suggestion  int  // selected suggestion, -1 when the typed word is shown

	// Translation choice: the selected item of each word's column
	choices []int
	slot    int

	// Components
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
		services:   services,
		from:       from,
		lexicon:    &lexicon.Lexicon{},
		focus:      focusInput,
		suggestion: -1,

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
			if len(m.words) == 0 {
				return m, nil
			}
			return m, m.copy()
		case key.Matches(msg, keys.Paste):
			return m, readClipboard
		}
		switch m.focus {
		case focusInput:
			return m.updateInput(msg)
		case focusChoice:
			return m.updateChoice(msg)
		}
		if key.Matches(msg, keys.Quit) {
			return m, m.quit()
		}
	}

	// Components ignore keys when they don't have focus
	var directionCmd, memorizeCmd tea.Cmd
	m.directionButton, directionCmd = m.directionButton.Update(msg)
	m.memorizeButton, memorizeCmd = m.memorizeButton.Update(msg)

	return m, tea.Batch(directionCmd, memorizeCmd)
}

func (m Model) updateInput(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.QuitInput):
		return m, m.quit()
	case key.Matches(msg, keys.Commit):
		return m.commit(), nil
	case key.Matches(msg, keys.PrevSuggestion):
		if m.suggestion >= 0 {
			m.suggestion--
		}
		return m, nil
	case key.Matches(msg, keys.NextSuggestion):
		if m.suggestion < len(m.suggestions)-1 {
			m.suggestion++
		}
		return m, nil
	case key.Matches(msg, keys.Erase):
		return m.erase(), nil
	case msg.Text != "" && msg.Mod&(tea.ModCtrl|tea.ModAlt|tea.ModMeta|tea.ModSuper|tea.ModHyper) == 0:
		return m.setTyped(m.current() + msg.Text), nil
	}
	return m, nil
}

func (m Model) updateChoice(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if len(m.words) == 0 {
		if key.Matches(msg, keys.Quit) {
			return m, m.quit()
		}
		return m, nil
	}

	entry := m.lexicon.Lookup(m.words[m.slot])
	choice := m.choices[m.slot]
	switch {
	case key.Matches(msg, keys.Quit):
		return m, m.quit()
	case key.Matches(msg, keys.PrevWord):
		m.slot = max(0, m.slot-1)
	case key.Matches(msg, keys.NextWord):
		m.slot = min(len(m.words)-1, m.slot+1)
	case key.Matches(msg, keys.PrevCandidate):
		m.choices[m.slot] = max(0, choice-1)
	case key.Matches(msg, keys.NextCandidate):
		m.choices[m.slot] = min(addIndex(entry), choice+1)
	case key.Matches(msg, keys.Add),
		key.Matches(msg, keys.Press) && choice == addIndex(entry):
		return m, m.addTranslation()
	case key.Matches(msg, keys.Delete) && choice < len(entry.Translations):
		link := entry.Translations[choice]
		return m, tui.PushModal(confirmmodal.New(confirmmodal.Config{
			Title:        fmt.Sprintf("Supprimer le lien avec « %s » ?", link.Word.Text),
			ConfirmLabel: "Supprimer le lien",
			OnConfirm:    func() tea.Msg { return translationDeleteConfirmedMsg{id: link.ID} },
		}))
	}
	return m, nil
}

// addIndex returns the index of the "+" item of a word's column, below its
// translations or its "?".
func addIndex(entry lexicon.Entry) int {
	return max(1, len(entry.Translations))
}

// current returns the selected suggestion, or the typed word.
func (m Model) current() string {
	if m.suggestion >= 0 {
		return m.suggestions[m.suggestion].Text
	}
	return m.typed
}

func (m Model) setTyped(typed string) Model {
	m.typed = typed
	m.suggestion = -1
	m.suggestions, m.more = m.lexicon.Complete(typed, maxSuggestions)
	return m
}

func (m Model) appendWord(word string) Model {
	m.words = append(m.words, word)
	m.choices = append(m.choices, 0)
	return m
}

func (m Model) commit() Model {
	word := m.current()
	if word == "" {
		return m
	}
	return m.appendWord(word).setTyped("")
}

// erase reopens the previous word when the input is empty.
func (m Model) erase() Model {
	if word := []rune(m.current()); len(word) > 0 {
		return m.setTyped(string(word[:len(word)-1]))
	}
	if len(m.words) == 0 {
		return m
	}
	last := m.words[len(m.words)-1]
	m.words = m.words[:len(m.words)-1]
	m.choices = m.choices[:len(m.choices)-1]
	m.slot = max(0, min(m.slot, len(m.words)-1))
	return m.setTyped(last)
}

// paste appends text to the input, its last word staying in edition unless
// text ends with a space.
func (m Model) paste(text string) Model {
	text = m.current() + text
	words := strings.Fields(text)
	committed := len(words)
	if r := []rune(text); committed > 0 && !unicode.IsSpace(r[len(r)-1]) {
		committed--
	}

	m = m.setTyped("")
	for _, word := range words[:committed] {
		m = m.appendWord(word)
	}
	if committed < len(words) {
		m = m.setTyped(words[committed])
	}
	return m
}

func (m Model) handleLexiconLoadedMsg(msg lexiconLoadedMsg) (Model, tea.Cmd) {
	// A load started before a direction switch is stale
	if msg.from != m.from {
		return m, nil
	}

	previous := m.lexicon
	m.lexicon = msg.lexicon
	for i, word := range m.words {
		before := previous.Lookup(word)
		after := m.lexicon.Lookup(word)
		// A translation was added from the "+": select it
		if m.choices[i] == addIndex(before) && len(after.Translations) > len(before.Translations) {
			m.choices[i] = len(after.Translations) - 1
		}
		m.choices[i] = min(m.choices[i], addIndex(after))
	}

	m.suggestions, m.more = m.lexicon.Complete(m.typed, maxSuggestions)
	m.suggestion = min(m.suggestion, len(m.suggestions)-1)

	if m.focus == focusMemorize && len(m.newWords()) == 0 {
		m.focus = focusDirection
		m = m.applyFocus()
		return m, m.setKeyMap()
	}
	return m, nil
}

func (m Model) isEmpty() bool {
	return len(m.words) == 0 && m.current() == ""
}

func (m Model) quit() tea.Cmd {
	if m.isEmpty() {
		return tui.PopPage
	}
	return tui.PushModal(confirmmodal.New(confirmmodal.Config{
		Title:        "Quitter la traduction ?\nLa saisie sera effacée.",
		ConfirmLabel: "Quitter",
		OnConfirm:    tui.PopPage,
	}))
}

func (m Model) handleSwitchDirectionPressedMsg() (Model, tea.Cmd) {
	if m.isEmpty() {
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
	m.words, m.choices, m.slot = nil, nil, 0
	m = m.setTyped("")
	m = m.computeLayout()
	return m, m.load
}

func (m Model) newWords() []string {
	var words []string
	seen := make(map[string]bool)
	for _, word := range m.words {
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
	words := m.newWords()
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

// addTranslation opens the translation picker for the selected word,
// storing it first when it is unknown.
func (m Model) addTranslation() tea.Cmd {
	word := lexicon.Clean(m.words[m.slot])
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

// translation returns the translated text: the selected translation of each
// word, keeping its punctuation, or the word itself when untranslated.
func (m Model) translation() string {
	words := make([]string, len(m.words))
	for i, word := range m.words {
		words[i] = word
		translations := m.lexicon.Lookup(word).Translations
		if len(translations) == 0 {
			continue
		}
		clean := lexicon.Clean(word)
		start := strings.Index(word, clean)
		chosen := translations[min(m.choices[i], len(translations)-1)].Word.Text
		words[i] = word[:start] + chosen + word[start+len(clean):]
	}
	return strings.Join(words, " ")
}

func (m Model) copy() tea.Cmd {
	text := m.translation()
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
	if m.focus == focusMemorize && len(m.newWords()) == 0 {
		m.focus = (m.focus + focusOverflowed + step) % focusOverflowed
	}
	if m.focus == focusChoice {
		m.slot = max(0, min(m.slot, len(m.words)-1))
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
	// Buttons are sized to fit their content
	for _, b := range []*button.Model{&m.directionButton, &m.memorizeButton} {
		b.Width = lipgloss.Width(b.Content) + b.Style.GetHorizontalFrameSize()
		b.Height = b.Style.GetVerticalFrameSize() + 1
	}
	return m
}

func statusStyle(status lexicon.Status) lipgloss.Style {
	switch status {
	case lexicon.StatusTranslated:
		return translatedStyle
	case lexicon.StatusUntranslated:
		return untranslatedStyle
	}
	return unknownStyle
}

func (m Model) inputView(width int) string {
	words := make([]string, 0, len(m.words)+1)
	for _, word := range m.words {
		words = append(words, statusStyle(m.lexicon.Lookup(word).Status).Render(word))
	}

	current := ""
	if m.current() != "" {
		current = statusStyle(m.lexicon.Lookup(m.current()).Status).Render(m.current())
	}
	if m.focus == focusInput {
		current += cursor
	}
	if current != "" {
		words = append(words, current)
	}
	text := strings.Join(words, " ")
	if text == "" {
		text = faintStyle.Render("Saisissez un texte à traduire")
	}

	style := styles.Box
	if m.focus == focusInput {
		style = styles.BoxFocused
	}
	return style.Width(width).Render(text)
}

func (m Model) suggestionsView() string {
	if m.focus != focusInput || len(m.suggestions) == 0 {
		return ""
	}
	rows := make([]string, 0, len(m.suggestions)+1)
	for i, suggestion := range m.suggestions {
		if i == m.suggestion {
			rows = append(rows, selectedSuggestion.Render("▸ "+suggestion.Text))
		} else {
			rows = append(rows, suggestionStyle.Render(suggestion.Text))
		}
	}
	if m.more {
		rows = append(rows, suggestionStyle.Render("…"))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// columnView renders a word above its stacked items: its translations, or
// a "?" when it has none, and the "+" adding a translation.
func (m Model) columnView(i int) string {
	word := m.words[i]
	entry := m.lexicon.Lookup(word)
	active := m.focus == focusChoice && i == m.slot

	var items []string
	var itemStyles []lipgloss.Style
	for _, translation := range entry.Translations {
		items = append(items, translation.Word.Text)
		itemStyles = append(itemStyles, translatedStyle)
	}
	if len(items) == 0 {
		items = append(items, "?")
		itemStyles = append(itemStyles, statusStyle(entry.Status))
	}
	items = append(items, "+")
	itemStyles = append(itemStyles, lipgloss.NewStyle())

	header := faintStyle.Render(word)
	if active {
		header = activeHeaderStyle.Render(word)
	}
	rows := []string{header}
	for j, item := range items {
		style := itemStyles[j]
		switch {
		case j == m.choices[i] && active:
			style = style.Reverse(true)
		case j == m.choices[i]:
			style = style.Bold(true)
		default:
			style = style.Faint(true)
		}
		rows = append(rows, style.Render(item))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m Model) translationView(width int) string {
	style := styles.Box
	if m.focus == focusChoice {
		style = styles.BoxFocused
	}
	inner := max(1, width-style.GetHorizontalFrameSize())

	var rows []string
	var row []string
	rowWidth := 0
	for i := range m.words {
		column := m.columnView(i)
		w := lipgloss.Width(column)
		if len(row) > 0 && rowWidth+columnGap+w > inner {
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, row...))
			row, rowWidth = nil, 0
		}
		if len(row) > 0 {
			row = append(row, strings.Repeat(" ", columnGap))
			rowWidth += columnGap
		}
		row = append(row, column)
		rowWidth += w
	}
	if len(row) > 0 {
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, row...))
	}

	text := strings.Join(rows, "\n\n")
	if text == "" {
		text = faintStyle.Render("La traduction apparaîtra ici")
	}
	return style.Width(width).Render(text)
}

func (m Model) View() tea.View {
	memorizeButton := m.memorizeButton
	if len(m.newWords()) == 0 {
		memorizeButton.Style = disabledButtonStyle
	}
	buttons := lipgloss.JoinHorizontal(lipgloss.Center,
		m.directionButton.View(),
		strings.Repeat(" ", buttonGap),
		memorizeButton.View(),
	)

	parts := []string{buttons, m.inputView(m.width)}
	if suggestions := m.suggestionsView(); suggestions != "" {
		parts = append(parts, suggestions)
	}
	parts = append(parts, m.translationView(m.width))
	if m.status != "" {
		parts = append(parts, faintStyle.Render(m.status))
	}

	return tea.NewView(lipgloss.Place(
		m.width, m.height,
		lipgloss.Left, lipgloss.Top,
		lipgloss.JoinVertical(lipgloss.Left, parts...),
	))
}
