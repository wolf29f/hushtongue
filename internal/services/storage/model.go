package storage

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
