package storage

import "context"

// Storage stores the users' dictionaries. Every method acts on the
// dictionary of userID only: an id of another user's word or translation is
// reported as ErrNotFound.
type Storage interface {
	AddWord(ctx context.Context, userID int, language, text string) (WordDetails, error)
	ListWordDetails(ctx context.Context, userID int, language string) ([]WordDetails, error)
	GetWord(ctx context.Context, userID, id int) (WordDetails, error)
	SaveWord(ctx context.Context, userID int, word WordDetails) (WordDetails, error)
	MergeWords(ctx context.Context, userID, fromID, intoID int) error
	DeleteWord(ctx context.Context, userID, id int) error
	ChangeWordKind(ctx context.Context, userID, id int, kind string) (WordDetails, error)
	ListTranslations(ctx context.Context, userID, wordID int) ([]Translation, error)
	ListTranslationsByLanguage(ctx context.Context, userID int, language string) (map[int][]Translation, error)
	DeleteTranslation(ctx context.Context, userID, id int) error
	AddTranslation(ctx context.Context, userID, wordID, targetID int) error
	AddTranslationWord(ctx context.Context, userID, wordID int, text string) error
	SaveGeneratedTranslation(ctx context.Context, userID, wordID int, parts []TranslationPart) error
}
