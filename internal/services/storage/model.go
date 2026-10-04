package storage

// Word languages.
const (
	LangSource = "source"
	LangCon    = "con"
)

// Word kinds.
const (
	KindRoot   = "root"
	KindPrefix = "prefix"
	KindSuffix = "suffix"
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

// TranslationPart is a part of a decomposed source word with its con
// translation. An ID is 0 when the word isn't stored yet.
type TranslationPart struct {
	Kind     string
	SourceID int
	Source   string
	ConID    int
	Con      string
}

// Translation is a translation link, seen from one of its two words: Word
// is the word on the other side.
type Translation struct {
	ID   int
	Word Word
}
