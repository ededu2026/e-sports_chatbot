package main

import (
	"context"
	"log"
	"time"

	"github.com/ededu2026/e-sports_chatbot/internal/config"
	"github.com/ededu2026/e-sports_chatbot/internal/ollama"
	"github.com/ededu2026/e-sports_chatbot/internal/rag"
)

func main() {
	cfg := config.Load()
	ollamaClient := ollama.New(cfg)
	service := rag.NewService(rag.ProjectRoot(), cfg, ollamaClient)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.RequestTimeoutSecs)*time.Second)
	defer cancel()

	count, err := service.Ingest(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("ingested %d knowledge base documents into qdrant", count)
}
