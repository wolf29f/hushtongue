package storage

// Word languages.
const (
	LangSource = "source"
	LangCon    = "con"
)

type Word struct {
	ID   int
	Text string
}

type WordDetails struct {
	ID         int
	Language   string // source or con
	Text       string
	Normalized string // read-only normalized form of the word
	Kind       string
}

// Translation is a translation link, seen from one of its two words: Word
// is the word on the other side.
type Translation struct {
	ID   int
	Word Word
}
