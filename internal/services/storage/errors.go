package storage

import "errors"

// ErrSameLanguage is returned when linking two words of the same language.
var ErrSameLanguage = errors.New("words of the same language")
