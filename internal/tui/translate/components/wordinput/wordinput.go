package wordinput

import (
	"strings"
	"unicode"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/wolf29f/hushtongue/internal/services/lexicon"
	"github.com/wolf29f/hushtongue/internal/services/storage"
	"github.com/wolf29f/hushtongue/internal/tui/styles"
)

const (
	maxSuggestions = 5
	cursor         = "▏"
)

var (
	faintStyle         = lipgloss.NewStyle().Faint(true)
	suggestionStyle    = lipgloss.NewStyle().PaddingLeft(4)
	selectedSuggestion = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170")).Bold(true)
)

// Model is a text typed word by word, the word being typed completed from a
// lexicon.
type Model struct {
	// Total width, border and padding included
	Width   int
	Focused bool

	lexicon     *lexicon.Lexicon
	words       []string // committed words
	typed       string
	suggestions []storage.Word
	more        bool // whether suggestions were left out
	suggestion  int  // selected suggestion, -1 when the typed word is shown
}

// New creates an empty input over an empty lexicon.
func New() Model {
	return Model{
		lexicon:    &lexicon.Lexicon{},
		suggestion: -1,
	}
}

// SetLexicon colors the words and completes the typed word from l.
func (m Model) SetLexicon(l *lexicon.Lexicon) Model {
	m.lexicon = l
	m.suggestions, m.more = m.lexicon.Complete(m.typed, maxSuggestions)
	m.suggestion = min(m.suggestion, len(m.suggestions)-1)
	return m
}

// Words returns the committed words.
func (m Model) Words() []string {
	return m.words
}

func (m Model) IsEmpty() bool {
	return len(m.words) == 0 && m.current() == ""
}

// Reset erases the whole text.
func (m Model) Reset() Model {
	m.words = nil
	return m.setTyped("")
}

// Commit appends the word being typed to the committed words.
func (m Model) Commit() Model {
	word := m.current()
	if word == "" {
		return m
	}
	m.words = append(m.words, word)
	return m.setTyped("")
}

// Paste appends text, its last word staying in edition unless text ends
// with a space.
func (m Model) Paste(text string) Model {
	text = m.current() + text
	words := strings.Fields(text)
	committed := len(words)
	if r := []rune(text); committed > 0 && !unicode.IsSpace(r[len(r)-1]) {
		committed--
	}

	m.words = append(m.words, words[:committed]...)
	typed := ""
	if committed < len(words) {
		typed = words[committed]
	}
	return m.setTyped(typed)
}

func (m Model) Update(msg tea.Msg) Model {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok || !m.Focused {
		return m
	}
	switch {
	case key.Matches(keyMsg, Keys.Commit):
		return m.Commit()
	case key.Matches(keyMsg, Keys.PrevSuggestion):
		if m.suggestion >= 0 {
			m.suggestion--
		}
	case key.Matches(keyMsg, Keys.NextSuggestion):
		if m.suggestion < len(m.suggestions)-1 {
			m.suggestion++
		}
	case key.Matches(keyMsg, Keys.Erase):
		return m.erase()
	case keyMsg.Text != "" && keyMsg.Mod&(tea.ModCtrl|tea.ModAlt|tea.ModMeta|tea.ModSuper|tea.ModHyper) == 0:
		return m.setTyped(m.current() + keyMsg.Text)
	}
	return m
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
	return m.setTyped(last)
}

func (m Model) View() string {
	words := make([]string, 0, len(m.words)+1)
	for _, word := range m.words {
		words = append(words, styles.WordStatus(m.lexicon.Lookup(word).Status).Render(word))
	}

	current := ""
	if m.current() != "" {
		current = styles.WordStatus(m.lexicon.Lookup(m.current()).Status).Render(m.current())
	}
	if m.Focused {
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
	if m.Focused {
		style = styles.BoxFocused
	}
	return style.Width(m.Width).Render(text)
}

// SuggestionsView renders the completions of the typed word, empty when the
// input isn't focused.
func (m Model) SuggestionsView() string {
	if !m.Focused || len(m.suggestions) == 0 {
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
