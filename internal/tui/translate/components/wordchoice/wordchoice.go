package wordchoice

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/wolf29f/hushtongue/internal/services/lexicon"
	"github.com/wolf29f/hushtongue/internal/services/storage"
	"github.com/wolf29f/hushtongue/internal/tui/styles"
)

const columnGap = 2

var (
	faintStyle        = lipgloss.NewStyle().Faint(true)
	activeHeaderStyle = lipgloss.NewStyle().Foreground(styles.Accent).Bold(true)
)

// Model is the choice of a translation for each word of a text, shown as
// columns wrapping over rows.
type Model struct {
	// Total width, border and padding included
	Width   int
	Focused bool

	lexicon *lexicon.Lexicon
	words   []string
	choices []int // selected item of each word's column
	slot    int   // selected word
}

// New creates an empty choice over an empty lexicon.
func New() Model {
	return Model{lexicon: &lexicon.Lexicon{}}
}

// SetWords sets the words to translate, keeping the choices of the words
// left in place.
func (m Model) SetWords(words []string) Model {
	choices := make([]int, len(words))
	copy(choices, m.choices)
	m.words = slices.Clone(words)
	m.choices = choices
	m.slot = max(0, min(m.slot, len(words)-1))
	return m
}

// SetLexicon selects the translation added from a "+", if any, and keeps
// the choices within the columns.
func (m Model) SetLexicon(l *lexicon.Lexicon) Model {
	previous := m.lexicon
	m.lexicon = l
	for i, word := range m.words {
		before := previous.Lookup(word)
		after := m.lexicon.Lookup(word)
		if m.choices[i] == addIndex(before) && len(after.Translations) > len(before.Translations) {
			m.choices[i] = len(after.Translations) - 1
		}
		m.choices[i] = min(m.choices[i], addIndex(after))
	}
	return m
}

// Translation returns the translated text: the selected translation of
// each word, keeping its punctuation, or the word itself when untranslated.
func (m Model) Translation() string {
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

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok || !m.Focused || len(m.words) == 0 {
		return m, nil
	}

	word := m.words[m.slot]
	entry := m.lexicon.Lookup(word)
	choice := m.choices[m.slot]
	switch {
	case key.Matches(keyMsg, Keys.PrevWord):
		m.slot = max(0, m.slot-1)
	case key.Matches(keyMsg, Keys.NextWord):
		m.slot = min(len(m.words)-1, m.slot+1)
	case key.Matches(keyMsg, Keys.PrevCandidate):
		m.choices[m.slot] = max(0, choice-1)
	case key.Matches(keyMsg, Keys.NextCandidate):
		m.choices[m.slot] = min(addIndex(entry), choice+1)
	case key.Matches(keyMsg, Keys.Add),
		key.Matches(keyMsg, pressKey) && choice == addIndex(entry):
		return m, func() tea.Msg { return AddPressedMsg{Word: word} }
	case key.Matches(keyMsg, Keys.Delete) && choice < len(entry.Translations):
		translation := entry.Translations[choice]
		return m, func() tea.Msg { return DeletePressedMsg{Translation: translation} }
	}
	return m, nil
}

// addIndex returns the index of the "+" item of a word's column, below its
// translations or its "?".
func addIndex(entry lexicon.Entry) int {
	return max(1, len(entry.Translations))
}

// columnView renders a word above its stacked items: its translations, or
// a "?" when it has none, and the "+" adding a translation.
func (m Model) columnView(i int) string {
	word := m.words[i]
	entry := m.lexicon.Lookup(word)
	active := m.Focused && i == m.slot

	var items []string
	var itemStyles []lipgloss.Style
	for _, translation := range entry.Translations {
		items = append(items, translation.Word.Text)
		itemStyles = append(itemStyles, styles.WordStatus(lexicon.StatusTranslated))
	}
	if len(items) == 0 {
		items = append(items, "?")
		itemStyles = append(itemStyles, styles.WordStatus(entry.Status))
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

func (m Model) View() string {
	style := styles.Box
	if m.Focused {
		style = styles.BoxFocused
	}
	inner := max(1, m.Width-style.GetHorizontalFrameSize())

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
	return style.Width(m.Width).Render(text)
}

// AddPressedMsg asks to add a translation to Word, as typed.
type AddPressedMsg struct {
	Word string
}

// DeletePressedMsg asks to delete Translation.
type DeletePressedMsg struct {
	Translation storage.Translation
}
