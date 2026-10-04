package translationgenerator

import (
	"charm.land/lipgloss/v2"
)

var (
	wordStyle      = lipgloss.NewStyle().Bold(true).PaddingLeft(1)
	kindStyle      = lipgloss.NewStyle().Faint(true).PaddingLeft(1)
	selectedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true)
	knownStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	generatedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)
