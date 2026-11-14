package services

import (
	"github.com/wolf29f/hushtongue/internal/services/composer"
	"github.com/wolf29f/hushtongue/internal/services/storage"
)

type Services struct {
	Storage  storage.Storage
	Composer *composer.Service
}

func New(storage storage.Storage) *Services {
	return &Services{
		Storage:  storage,
		Composer: composer.New(storage),
	}
}
