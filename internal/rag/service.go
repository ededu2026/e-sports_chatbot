package rag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ededu2026/e-sports_chatbot/internal/config"
	"github.com/ededu2026/e-sports_chatbot/internal/ollama"
)

type Service struct {
	root    string
	qdrant  *QdrantClient
	ollama  *ollama.Client
	enabled bool
}

func NewService(root string, cfg config.Config, ollamaClient *ollama.Client) *Service {
	return &Service{
		root:    root,
		qdrant:  NewQdrantClient(cfg),
		ollama:  ollamaClient,
		enabled: true,
	}
}

func (s *Service) Ingest(ctx context.Context) (int, error) {
	docs, err := LoadKnowledgeBase(s.root)
	if err != nil {
		return 0, err
	}
	if len(docs) == 0 {
		return 0, fmt.Errorf("no knowledge base documents found")
	}

	vectors := make([][]float64, 0, len(docs))
	filteredDocs := make([]Document, 0, len(docs))
	for _, doc := range docs {
		vector, err := s.ollama.EmbedText(ctx, doc.Content)
		if err != nil {
			continue
		}
		filteredDocs = append(filteredDocs, doc)
		vectors = append(vectors, vector)
	}
	if len(filteredDocs) == 0 {
		return 0, fmt.Errorf("unable to embed any documents")
	}

	if err := s.qdrant.EnsureCollection(ctx, len(vectors[0])); err != nil {
		return 0, err
	}
	if err := s.qdrant.Upsert(ctx, filteredDocs, vectors); err != nil {
		return 0, err
	}
	return len(filteredDocs), nil
}

func (s *Service) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	lexicalResults, err := s.lexicalSearch(query, limit)
	if err == nil && len(lexicalResults) > 0 {
		return lexicalResults, nil
	}

	vector, err := s.ollama.EmbedText(ctx, query)
	if err != nil {
		return nil, err
	}
	return s.qdrant.Search(ctx, vector, limit)
}

func (s *Service) ContextBlock(ctx context.Context, query string, limit int) (string, error) {
	results, err := s.Search(ctx, query, limit)
	if err != nil {
		return "", err
	}
	if len(results) == 0 {
		return "", nil
	}

	var b strings.Builder
	b.WriteString("Use only the grounded esports facts below. If the answer is not supported by these facts, say you do not have enough grounded information.\n")
	for _, result := range results {
		b.WriteString("- Title: ")
		b.WriteString(result.Title)
		b.WriteString(" | Facts: ")
		b.WriteString(result.Content)
		b.WriteString("\n")
	}
	return b.String(), nil
}

func (s *Service) Health(ctx context.Context) bool {
	return s.qdrant.Health(ctx)
}

func ProjectRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return filepath.Clean(wd)
}

func (s *Service) lexicalSearch(query string, limit int) ([]SearchResult, error) {
	docs, err := LoadKnowledgeBase(s.root)
	if err != nil {
		return nil, err
	}

	query = normalize(query)
	type scored struct {
		doc   Document
		score float64
	}

	var matches []scored
	for _, doc := range docs {
		score := lexicalScore(query, doc)
		if score <= 0 {
			continue
		}
		matches = append(matches, scored{doc: doc, score: score})
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].score > matches[j].score
	})

	if limit <= 0 {
		limit = 4
	}
	if len(matches) > limit {
		matches = matches[:limit]
	}

	results := make([]SearchResult, 0, len(matches))
	for _, match := range matches {
		results = append(results, SearchResult{
			ID:      match.doc.ID,
			Title:   match.doc.Title,
			Content: match.doc.Content,
			Source:  match.doc.Source,
			Score:   match.score,
		})
	}
	return results, nil
}

func lexicalScore(query string, doc Document) float64 {
	title := normalize(doc.Title)
	content := normalize(doc.Content)
	id := normalize(doc.ID)

	score := 0.0
	switch {
	case strings.Contains(title, query):
		score += 10
	case strings.Contains(id, query):
		score += 8
	case strings.Contains(content, query):
		score += 4
	}

	entity := extractEntity(query)
	if entity != "" {
		switch {
		case strings.Contains(title, entity):
			score += 30
		case strings.Contains(id, entity):
			score += 25
		case strings.Contains(content, entity):
			score += 10
		}
	}

	return score
}

func extractEntity(query string) string {
	prefixes := []string{
		" who is ",
		" who is the ",
		" quem e ",
		" quem é ",
		" quem foi ",
		" who was ",
	}
	for _, prefix := range prefixes {
		if strings.Contains(query, prefix) {
			entity := strings.TrimSpace(strings.TrimPrefix(query, prefix))
			return normalize(entity)
		}
	}
	return ""
}

func normalize(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	input = strings.ReplaceAll(input, "-", " ")
	input = strings.ReplaceAll(input, "_", " ")
	input = strings.ReplaceAll(input, "?", "")
	input = strings.ReplaceAll(input, "!", "")
	input = strings.ReplaceAll(input, ".", "")
	input = strings.ReplaceAll(input, ",", "")
	return " " + strings.Join(strings.Fields(input), " ") + " "
}
