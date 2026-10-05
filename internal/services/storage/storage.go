package storage

type Storage interface {
	Init() error
	AddWord(language, word string) error
	ListWords(language string) ([]Word, error)
	ListWordDetails(language string) ([]WordDetails, error)
	GetWord(id int) (WordDetails, error)
	SaveWord(word WordDetails) (WordDetails, error)
	MergeWords(fromID, intoID int) error
	DeleteWord(id int) error
	ChangeWordKind(id int, kind string) (WordDetails, error)
	ListTranslations(wordID int) ([]Translation, error)
	DeleteTranslation(id int) error
	AddTranslation(wordID, targetID int) error
	AddTranslationWord(wordID int, text string) error
	SaveGeneratedTranslation(wordID int, parts []TranslationPart) error
}
