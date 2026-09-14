package home

import "charm.land/lipgloss/v2"

var (
	itemStyle = lipgloss.NewStyle().
			Padding(0, 1)

	selectedStyle = itemStyle.
			Bold(true).
			Foreground(lipgloss.Color("205")).
			SetString("> ") // préfixe optionnel, cf. remarque plus bas
)
