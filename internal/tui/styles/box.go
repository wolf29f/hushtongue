package styles

import "charm.land/lipgloss/v2"

var (
	// Accent is the highlight color used for focus and active elements.
	Accent = lipgloss.Color("5")

	// Box is the default bordered box style.
	Box = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2)

	// BoxFocused is the bordered box style used when the box has focus.
	BoxFocused = Box.BorderForeground(Accent)
)
