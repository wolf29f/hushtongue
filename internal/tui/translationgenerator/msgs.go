package translationgenerator

import (
	"github.com/wolf29f/hushtongue/internal/services/storage"
	"github.com/wolf29f/hushtongue/internal/services/translation"
)

type loadedMsg struct {
	word      storage.WordDetails
	proposals []translation.Proposal
}
