package translate

import (
	"charm.land/lipgloss/v2"
	"github.com/wolf29f/hushtongue/internal/tui/styles"
)

var (
	faintStyle      = lipgloss.NewStyle().Faint(true)
	activeLangStyle = lipgloss.NewStyle().Foreground(styles.Accent)

	disabledButtonStyle = styles.Box.
				Foreground(lipgloss.Color("8")).
				BorderForeground(lipgloss.Color("8"))
)
