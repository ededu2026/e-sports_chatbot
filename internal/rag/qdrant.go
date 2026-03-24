package rag

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ededu2026/e-sports_chatbot/internal/config"
)

type QdrantClient struct {
	baseURL    string
	collection string
	http       *http.Client
}

func NewQdrantClient(cfg config.Config) *QdrantClient {
	return &QdrantClient{
		baseURL:    strings.TrimRight(cfg.QdrantURL, "/"),
		collection: cfg.QdrantCollection,
		http: &http.Client{
			Timeout: time.Duration(cfg.RequestTimeoutSecs) * time.Second,
		},
	}
}

func (q *QdrantClient) EnsureCollection(ctx context.Context, vectorSize int) error {
	payload := map[string]interface{}{
		"vectors": map[string]interface{}{
			"size":     vectorSize,
			"distance": "Cosine",
		},
	}
	return q.request(ctx, http.MethodPut, "/collections/"+q.collection, payload, nil)
}

func (q *QdrantClient) Upsert(ctx context.Context, docs []Document, vectors [][]float64) error {
	points := make([]map[string]interface{}, 0, len(docs))
	for i, doc := range docs {
		points = append(points, map[string]interface{}{
			"id":     numericPointID(doc.ID),
			"vector": vectors[i],
			"payload": map[string]interface{}{
				"doc_id":   doc.ID,
				"title":    doc.Title,
				"content":  doc.Content,
				"source":   doc.Source,
				"category": doc.Category,
			},
		})
	}

	payload := map[string]interface{}{
		"points": points,
	}
	return q.request(ctx, http.MethodPut, "/collections/"+q.collection+"/points", payload, nil)
}

func (q *QdrantClient) Search(ctx context.Context, vector []float64, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 4
	}

	payload := map[string]interface{}{
		"vector":       vector,
		"limit":        limit,
		"with_payload": true,
	}

	var raw struct {
		Result []struct {
			ID      interface{} `json:"id"`
			Score   float64     `json:"score"`
			Payload struct {
				DocID   string `json:"doc_id"`
				Title   string `json:"title"`
				Content string `json:"content"`
				Source  string `json:"source"`
			} `json:"payload"`
		} `json:"result"`
	}
	if err := q.request(ctx, http.MethodPost, "/collections/"+q.collection+"/points/search", payload, &raw); err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, len(raw.Result))
	for _, item := range raw.Result {
		results = append(results, SearchResult{
			ID:      firstNonEmpty(item.Payload.DocID, fmt.Sprint(item.ID)),
			Title:   item.Payload.Title,
			Content: item.Payload.Content,
			Source:  item.Payload.Source,
			Score:   item.Score,
		})
	}
	return results, nil
}

func numericPointID(input string) uint64 {
	sum := sha1.Sum([]byte(input))
	return binary.BigEndian.Uint64(sum[:8])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (q *QdrantClient) Health(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, q.baseURL+"/collections", nil)
	if err != nil {
		return false
	}
	resp, err := q.http.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 400
}

func (q *QdrantClient) request(ctx context.Context, method, path string, payload interface{}, out interface{}) error {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, q.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := q.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		if resp.StatusCode == http.StatusConflict && strings.Contains(path, "/collections/") && method == http.MethodPut {
			return nil
		}
		return fmt.Errorf("qdrant error (%d): %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return err
		}
	}
	return nil
}
