package wordview

import "github.com/wolf29f/hushtongue/internal/services/storage"

type switchKindMsg struct{}

type switchLangMsg struct{}

type wordDetailsMsg struct {
	Word storage.WordDetails
}
