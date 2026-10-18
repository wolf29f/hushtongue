package styles

import (
	"charm.land/lipgloss/v2"
	"github.com/wolf29f/hushtongue/internal/services/lexicon"
)

var (
	translatedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	untranslatedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	unknownStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

// WordStatus returns the style of a word showing how well it is known.
func WordStatus(status lexicon.Status) lipgloss.Style {
	switch status {
	case lexicon.StatusTranslated:
		return translatedStyle
	case lexicon.StatusUntranslated:
		return untranslatedStyle
	}
	return unknownStyle
}
