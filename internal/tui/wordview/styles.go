package wordview

import (
	"charm.land/lipgloss/v2"
	"github.com/wolf29f/hushtongue/internal/tui/styles"
)

var activeLangStyle = lipgloss.NewStyle().Foreground(styles.Accent)

var wordStyle = lipgloss.NewStyle().Bold(true).PaddingLeft(1)
