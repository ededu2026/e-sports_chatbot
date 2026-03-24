package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port               string
	OllamaBaseURL      string
	Model              string
	EmbeddingModel     string
	SystemName         string
	MaxHistoryMessages int
	RequestTimeoutSecs int
	RedisAddr          string
	RedisPassword      string
	RedisDB            int
	RedisEnabled       bool
	QdrantURL          string
	QdrantCollection   string
}

func Load() Config {
	return Config{
		Port:               getEnv("PORT", "8080"),
		OllamaBaseURL:      getEnv("OLLAMA_BASE_URL", "http://localhost:11434"),
		Model:              getEnv("OLLAMA_MODEL", "mistral"),
		EmbeddingModel:     getEnv("EMBEDDING_MODEL", "nomic-embed-text"),
		SystemName:         getEnv("SYSTEM_NAME", "Esports Arena AI"),
		MaxHistoryMessages: getEnvInt("MAX_HISTORY_MESSAGES", 12),
		RequestTimeoutSecs: getEnvInt("REQUEST_TIMEOUT_SECONDS", 90),
		RedisAddr:          getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:      getEnv("REDIS_PASSWORD", ""),
		RedisDB:            getEnvInt("REDIS_DB", 0),
		RedisEnabled:       getEnvBool("REDIS_ENABLED", true),
		QdrantURL:          getEnv("QDRANT_URL", "http://localhost:6333"),
		QdrantCollection:   getEnv("QDRANT_COLLECTION", "esports_knowledge"),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func getEnvBool(key string, fallback bool) bool {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return fallback
	}

	switch strings.ToLower(raw) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
