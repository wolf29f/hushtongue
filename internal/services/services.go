package services

import "github.com/wolf29f/hushtongue/internal/services/storage"

type Services struct {
	Storage storage.Storage
}

func New(storage storage.Storage) *Services {
	return &Services{
		Storage: storage,
	}
}
