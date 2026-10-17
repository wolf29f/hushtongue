package translate

import (
	"charm.land/lipgloss/v2"
	"github.com/wolf29f/hushtongue/internal/tui/styles"
)

const cursor = "▏"

var (
	translatedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	untranslatedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	unknownStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))

	faintStyle         = lipgloss.NewStyle().Faint(true)
	activeLangStyle    = lipgloss.NewStyle().Foreground(styles.Accent)
	activeHeaderStyle  = lipgloss.NewStyle().Foreground(styles.Accent).Bold(true)
	suggestionStyle    = lipgloss.NewStyle().PaddingLeft(4)
	selectedSuggestion = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170")).Bold(true)

	disabledButtonStyle = styles.Box.
				Foreground(lipgloss.Color("8")).
				BorderForeground(lipgloss.Color("8"))
)
