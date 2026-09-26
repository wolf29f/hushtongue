package dictionary

import (
	"charm.land/lipgloss/v2"
)

var defaultBoxStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	Padding(1, 2)

var defaultBoxStyleFocus = defaultBoxStyle.BorderForeground(lipgloss.Color("5"))
