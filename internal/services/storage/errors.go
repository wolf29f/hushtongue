package storage

import (
	"errors"
	"fmt"
)

// ErrSameLanguage is returned when linking two words of the same language.
var ErrSameLanguage = errors.New("words of the same language")

// ErrNotFound is returned for an id that doesn't exist, or belongs to another
// user: both look the same to the caller.
var ErrNotFound = errors.New("not found")

// ErrNameTaken is returned when naming a user like another one.
var ErrNameTaken = errors.New("name already taken")

// WordExistsError is returned when saving a word would collide with
// ExistingID, a stored word of the same language, normalized form and kind.
type WordExistsError struct {
	ExistingID int
}

func (e *WordExistsError) Error() string {
	return fmt.Sprintf("word already exists with id %d", e.ExistingID)
}
