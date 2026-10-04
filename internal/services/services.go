package services

import (
	"github.com/wolf29f/hushtongue/internal/services/storage"
	"github.com/wolf29f/hushtongue/internal/services/translation"
)

type Services struct {
	Storage     storage.Storage
	Translation *translation.Service
}

func New(storage storage.Storage) *Services {
	return &Services{
		Storage:     storage,
		Translation: translation.New(storage),
	}
}
