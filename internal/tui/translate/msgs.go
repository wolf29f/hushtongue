package translate

import (
	"github.com/wolf29f/hushtongue/internal/services/lexicon"
)

type lexiconLoadedMsg struct {
	from    string
	lexicon *lexicon.Lexicon
}

type switchDirectionPressedMsg struct{}

type switchDirectionConfirmedMsg struct{}

type memorizePressedMsg struct{}

type translationDeleteConfirmedMsg struct {
	id int
}

type clipboardReadMsg struct {
	text string
}

type copiedMsg struct{}
