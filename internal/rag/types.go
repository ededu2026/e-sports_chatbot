package rag

type Document struct {
	ID       string
	Title    string
	Content  string
	Source   string
	Category string
}

type SearchResult struct {
	ID      string
	Title   string
	Content string
	Source  string
	Score   float64
}
