package storage

type Storage interface {
	Init() error
	ListWords(language string) ([]Word, error)
}
