package storage

type Storage interface {
	Init() error
	AddWord(language, word string) error
	ListWords(language string) ([]Word, error)
	GetWord(id int) (WordDetails, error)
	SaveWord(word WordDetails) (WordDetails, error)
}
