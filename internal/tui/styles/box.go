package styles

import "charm.land/lipgloss/v2"

var (
	// Box is the default bordered box style.
	Box = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2)

	// BoxFocused is the bordered box style used when the box has focus.
	BoxFocused = Box.BorderForeground(lipgloss.Color("5"))
)
