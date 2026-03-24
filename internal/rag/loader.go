package rag

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type wikiSummary struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Extract     string `json:"extract"`
}

func LoadKnowledgeBase(root string) ([]Document, error) {
	paths := []struct {
		dir      string
		category string
	}{
		{dir: filepath.Join(root, "knowledge_base", "curated"), category: "curated"},
		{dir: filepath.Join(root, "knowledge_base", "sources", "wikipedia"), category: "wikipedia"},
	}

	var documents []Document
	for _, item := range paths {
		files, err := walkFiles(item.dir)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			doc, err := loadDocument(file, item.category)
			if err != nil {
				continue
			}
			if strings.TrimSpace(doc.Content) == "" {
				continue
			}
			documents = append(documents, doc)
		}
	}
	return documents, nil
}

func loadDocument(path, category string) (Document, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Document{}, err
	}

	id := slugFromPath(path)
	switch filepath.Ext(path) {
	case ".md":
		content := strings.TrimSpace(string(raw))
		title := firstMarkdownHeading(content)
		if title == "" {
			title = id
		}
		return Document{
			ID:       id,
			Title:    title,
			Content:  content,
			Source:   path,
			Category: category,
		}, nil
	case ".json":
		var summary wikiSummary
		if err := json.Unmarshal(raw, &summary); err != nil {
			return Document{}, err
		}
		content := strings.TrimSpace(summary.Extract)
		if summary.Description != "" {
			content = strings.TrimSpace(summary.Description + "\n\n" + content)
		}
		return Document{
			ID:       id,
			Title:    strings.TrimSpace(summary.Title),
			Content:  content,
			Source:   path,
			Category: category,
		}, nil
	default:
		return Document{}, fmt.Errorf("unsupported file type")
	}
}

func walkFiles(root string) ([]string, error) {
	var matches []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		switch filepath.Ext(path) {
		case ".md", ".json":
			matches = append(matches, path)
		}
		return nil
	})
	return matches, err
}

func firstMarkdownHeading(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}

func slugFromPath(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}
