package server

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ededu2026/e-sports_chatbot/internal/chat"
	"github.com/ededu2026/e-sports_chatbot/internal/config"
	"github.com/ededu2026/e-sports_chatbot/internal/guardrails"
	"github.com/ededu2026/e-sports_chatbot/internal/ollama"
	"github.com/ededu2026/e-sports_chatbot/internal/rag"
	"github.com/ededu2026/e-sports_chatbot/internal/storage"
)

//go:embed web/*
var webFS embed.FS

type Server struct {
	cfg    config.Config
	ollama *ollama.Client
	store  storage.Store
	rag    *rag.Service
}

func New(cfg config.Config) *Server {
	root, err := os.Getwd()
	if err != nil {
		root = "."
	}
	ollamaClient := ollama.New(cfg)
	return &Server{
		cfg:    cfg,
		ollama: ollamaClient,
		store:  storage.New(cfg),
		rag:    rag.NewService(root, cfg, ollamaClient),
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	staticFS, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/ingest", s.handleIngest)
	mux.HandleFunc("/api/conversations", s.handleConversations)
	mux.HandleFunc("/api/conversations/", s.handleConversationByID)
	mux.HandleFunc("/api/chat/stream", s.handleChatStream)
	mux.HandleFunc("/api/chat", s.handleChat)
	mux.Handle("/web/", http.StripPrefix("/web/", http.FileServer(http.FS(staticFS))))
	mux.HandleFunc("/", s.handleIndex)
	return withCORS(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ollamaReachable, modelAvailable := s.ollama.CheckModelAvailability(ctx)
	qdrantReachable := s.rag.Health(ctx)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":           "ok",
		"model":            s.cfg.Model,
		"embedding_model":  s.cfg.EmbeddingModel,
		"name":             s.cfg.SystemName,
		"ollama_reachable": ollamaReachable,
		"model_available":  modelAvailable,
		"storage_mode":     s.store.Mode(),
		"qdrant_reachable": qdrantReachable,
	})
}

func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(s.cfg.RequestTimeoutSecs)*time.Second)
	defer cancel()

	count, err := s.rag.Ingest(ctx)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":             "ok",
		"ingested_documents": count,
		"collection":         s.cfg.QdrantCollection,
	})
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req chat.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}

	req.Message = strings.TrimSpace(req.Message)
	req.History = trimHistory(req.History, s.cfg.MaxHistoryMessages)
	conversationID := strings.TrimSpace(req.ConversationID)
	if conversationID == "" {
		conversationID = storage.NewConversationID()
	}

	decision := guardrails.Evaluate(req.Message, req.History)
	if decision.Allowed && strings.TrimSpace(req.Message) != "" && isGreetingOnly(req.Message) {
		response := chat.Response{
			Answer:         guardrails.Greeting(decision.Language),
			Language:       decision.Language,
			Blocked:        false,
			DomainAllowed:  true,
			ConversationID: conversationID,
		}
		s.persistConversation(r.Context(), conversationID, req.Message, response.Answer, req.History)
		writeJSON(w, http.StatusOK, response)
		return
	}

	if !decision.Allowed && decision.Reason != "out_of_scope" {
		response := chat.Response{
			Answer:         guardrails.Refusal(decision.Language, decision.Reason),
			Language:       decision.Language,
			Reason:         decision.Reason,
			Blocked:        true,
			DomainAllowed:  false,
			ConversationID: conversationID,
		}
		s.persistConversation(r.Context(), conversationID, req.Message, response.Answer, req.History)
		writeJSON(w, http.StatusOK, response)
		return
	}

	domainAllowed := decision.Allowed
	if !domainAllowed {
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()

		ollamaReachable, modelAvailable := s.ollama.CheckModelAvailability(ctx)
		if ollamaReachable && modelAvailable {
			allowed, err := s.ollama.ClassifyEsports(ctx, req.Message)
			if err != nil {
				log.Printf("classifier error: %v", err)
			}
			domainAllowed = err == nil && allowed
		}
	}

	if !domainAllowed {
		response := chat.Response{
			Answer:         guardrails.Refusal(decision.Language, "out_of_scope"),
			Language:       decision.Language,
			Reason:         "out_of_scope",
			Blocked:        true,
			DomainAllowed:  false,
			ConversationID: conversationID,
		}
		s.persistConversation(r.Context(), conversationID, req.Message, response.Answer, req.History)
		writeJSON(w, http.StatusOK, response)
		return
	}

	cacheKey := storage.CacheKey(req.Message, req.History)
	if cached, ok, err := s.store.GetCachedAnswer(r.Context(), cacheKey); err == nil && ok {
		cached.ConversationID = conversationID
		cached.Cached = true
		s.persistConversation(r.Context(), conversationID, req.Message, cached.Answer, req.History)
		writeJSON(w, http.StatusOK, cached)
		return
	} else if err != nil {
		log.Printf("cache lookup error: %v", err)
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(s.cfg.RequestTimeoutSecs)*time.Second)
	defer cancel()

	retrievedContext, err := s.rag.ContextBlock(ctx, req.Message, 4)
	if err != nil {
		log.Printf("rag retrieval error: %v", err)
		retrievedContext = ""
	}
	if retrievedContext == "" && isEntityLookup(req.Message) {
		response := chat.Response{
			Answer:         groundedFallback(decision.Language),
			Language:       decision.Language,
			Model:          s.ollama.Model(),
			DomainAllowed:  true,
			ConversationID: conversationID,
		}
		s.persistConversation(r.Context(), conversationID, req.Message, response.Answer, req.History)
		writeJSON(w, http.StatusOK, response)
		return
	}

	answer, err := s.ollama.GenerateAnswer(ctx, req.Message, req.History, decision.Language, retrievedContext)
	if err != nil {
		log.Printf("generation error: %v", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": "The local model is unavailable right now. Please make sure Ollama is running and the model is installed.",
		})
		return
	}

	response := chat.Response{
		Answer:         answer,
		Language:       decision.Language,
		Model:          s.ollama.Model(),
		DomainAllowed:  true,
		ConversationID: conversationID,
	}
	if err := s.store.SetCachedAnswer(r.Context(), cacheKey, response); err != nil {
		log.Printf("cache write error: %v", err)
	}
	s.persistConversation(r.Context(), conversationID, req.Message, response.Answer, req.History)
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	var req chat.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeStreamEvent(w, chat.StreamEvent{Type: "error", Error: "invalid JSON payload"})
		flusher.Flush()
		return
	}

	req.Message = strings.TrimSpace(req.Message)
	req.History = trimHistory(req.History, s.cfg.MaxHistoryMessages)
	conversationID := strings.TrimSpace(req.ConversationID)
	if conversationID == "" {
		conversationID = storage.NewConversationID()
	}

	decision := guardrails.Evaluate(req.Message, req.History)
	if decision.Allowed && strings.TrimSpace(req.Message) != "" && isGreetingOnly(req.Message) {
		response := chat.Response{
			Answer:         guardrails.Greeting(decision.Language),
			Language:       decision.Language,
			Blocked:        false,
			DomainAllowed:  true,
			ConversationID: conversationID,
		}
		s.persistConversation(r.Context(), conversationID, req.Message, response.Answer, req.History)
		writeStreamEvent(w, streamDoneEvent(response))
		flusher.Flush()
		return
	}

	if !decision.Allowed && decision.Reason != "out_of_scope" {
		response := chat.Response{
			Answer:         guardrails.Refusal(decision.Language, decision.Reason),
			Language:       decision.Language,
			Reason:         decision.Reason,
			Blocked:        true,
			DomainAllowed:  false,
			ConversationID: conversationID,
		}
		s.persistConversation(r.Context(), conversationID, req.Message, response.Answer, req.History)
		writeStreamEvent(w, streamDoneEvent(response))
		flusher.Flush()
		return
	}

	domainAllowed := decision.Allowed
	if !domainAllowed {
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()

		ollamaReachable, modelAvailable := s.ollama.CheckModelAvailability(ctx)
		if ollamaReachable && modelAvailable {
			allowed, err := s.ollama.ClassifyEsports(ctx, req.Message)
			if err != nil {
				log.Printf("classifier error: %v", err)
			}
			domainAllowed = err == nil && allowed
		}
	}

	if !domainAllowed {
		response := chat.Response{
			Answer:         guardrails.Refusal(decision.Language, "out_of_scope"),
			Language:       decision.Language,
			Reason:         "out_of_scope",
			Blocked:        true,
			DomainAllowed:  false,
			ConversationID: conversationID,
		}
		s.persistConversation(r.Context(), conversationID, req.Message, response.Answer, req.History)
		writeStreamEvent(w, streamDoneEvent(response))
		flusher.Flush()
		return
	}

	cacheKey := storage.CacheKey(req.Message, req.History)
	if cached, ok, err := s.store.GetCachedAnswer(r.Context(), cacheKey); err == nil && ok {
		cached.ConversationID = conversationID
		cached.Cached = true
		s.persistConversation(r.Context(), conversationID, req.Message, cached.Answer, req.History)
		writeStreamEvent(w, streamDoneEvent(cached))
		flusher.Flush()
		return
	} else if err != nil {
		log.Printf("cache lookup error: %v", err)
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(s.cfg.RequestTimeoutSecs)*time.Second)
	defer cancel()

	retrievedContext, err := s.rag.ContextBlock(ctx, req.Message, 4)
	if err != nil {
		log.Printf("rag retrieval error: %v", err)
		retrievedContext = ""
	}
	if retrievedContext == "" && isEntityLookup(req.Message) {
		response := chat.Response{
			Answer:         groundedFallback(decision.Language),
			Language:       decision.Language,
			Model:          s.ollama.Model(),
			DomainAllowed:  true,
			ConversationID: conversationID,
		}
		s.persistConversation(r.Context(), conversationID, req.Message, response.Answer, req.History)
		writeStreamEvent(w, streamDoneEvent(response))
		flusher.Flush()
		return
	}

	writeStreamEvent(w, chat.StreamEvent{Type: "start", ConversationID: conversationID, Model: s.ollama.Model()})
	flusher.Flush()

	answer, err := s.ollama.GenerateAnswerStream(ctx, req.Message, req.History, decision.Language, retrievedContext, func(chunk string) error {
		writeStreamEvent(w, chat.StreamEvent{Type: "token", Content: chunk})
		flusher.Flush()
		return nil
	})
	if err != nil {
		log.Printf("generation error: %v", err)
		writeStreamEvent(w, chat.StreamEvent{Type: "error", Error: "The local model is unavailable right now. Please make sure Ollama is running and the model is installed."})
		flusher.Flush()
		return
	}

	response := chat.Response{
		Answer:         answer,
		Language:       decision.Language,
		Model:          s.ollama.Model(),
		DomainAllowed:  true,
		ConversationID: conversationID,
	}
	if err := s.store.SetCachedAnswer(r.Context(), cacheKey, response); err != nil {
		log.Printf("cache write error: %v", err)
	}
	s.persistConversation(r.Context(), conversationID, req.Message, response.Answer, req.History)
	writeStreamEvent(w, streamDoneEvent(response))
	flusher.Flush()
}

func (s *Server) handleConversations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	conversations, err := s.store.ListConversations(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load conversations"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"conversations": conversations,
	})
}

func (s *Server) handleConversationByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/conversations/")
	id = strings.TrimSpace(id)
	if id == "" {
		http.NotFound(w, r)
		return
	}

	conversation, ok, err := s.store.GetConversation(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load conversation"})
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}

	writeJSON(w, http.StatusOK, conversation)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.ServeFileFS(w, r, webFS, "web/index.html")
}

func trimHistory(history []chat.Message, max int) []chat.Message {
	if max <= 0 || len(history) <= max {
		return history
	}
	return history[len(history)-max:]
}

func (s *Server) persistConversation(ctx context.Context, conversationID, userMessage, assistantAnswer string, priorHistory []chat.Message) {
	messages := make([]chat.Message, 0, len(priorHistory)+2)
	messages = append(messages, priorHistory...)
	messages = append(messages, chat.Message{Role: "user", Content: userMessage})
	messages = append(messages, chat.Message{Role: "assistant", Content: assistantAnswer})

	conversation := chat.Conversation{
		ID:        conversationID,
		Title:     storage.TitleFromMessage(firstUserMessage(messages)),
		Messages:  messages,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	if err := s.store.SaveConversation(ctx, conversation); err != nil {
		log.Printf("conversation save error: %v", err)
	}
}

func firstUserMessage(messages []chat.Message) string {
	for _, msg := range messages {
		if msg.Role == "user" {
			return msg.Content
		}
	}
	return ""
}

func isGreetingOnly(input string) bool {
	normalized := strings.TrimSpace(strings.ToLower(input))
	switch normalized {
	case "hello", "hi", "hey", "good morning", "good afternoon", "good evening",
		"ola", "olá", "oi", "bom dia", "boa tarde", "boa noite",
		"hola", "buenos dias", "bonjour", "salut", "hallo", "ciao",
		"привет", "здравствуйте", "مرحبا", "اهلا", "नमस्ते", "你好", "您好", "こんにちは", "안녕", "안녕하세요":
		return true
	default:
		return false
	}
}

func isEntityLookup(input string) bool {
	normalized := strings.TrimSpace(strings.ToLower(input))
	prefixes := []string{
		"who is ",
		"who's ",
		"who was ",
		"quem e ",
		"quem é ",
		"quem foi ",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(normalized, prefix) {
			return true
		}
	}
	return false
}

func groundedFallback(language string) string {
	switch language {
	case "pt":
		return "Ainda nao tenho contexto grounded suficiente na base para responder com seguranca sobre essa entidade de eSports."
	case "es":
		return "Todavia no tengo suficiente contexto grounded en la base para responder con seguridad sobre esa entidad de eSports."
	default:
		return "I do not have enough grounded information in the knowledge base yet to answer safely about that esports entity."
	}
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeStreamEvent(w http.ResponseWriter, event chat.StreamEvent) {
	raw, _ := json.Marshal(event)
	_, _ = w.Write(append(raw, '\n'))
}

func streamDoneEvent(response chat.Response) chat.StreamEvent {
	return chat.StreamEvent{
		Type:           "done",
		Answer:         response.Answer,
		Language:       response.Language,
		Reason:         response.Reason,
		Blocked:        response.Blocked,
		Model:          response.Model,
		DomainAllowed:  response.DomainAllowed,
		ConversationID: response.ConversationID,
		Cached:         response.Cached,
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
