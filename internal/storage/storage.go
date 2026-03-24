package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ededu2026/e-sports_chatbot/internal/chat"
	"github.com/ededu2026/e-sports_chatbot/internal/config"
	"github.com/redis/go-redis/v9"
)

const chatCacheTTL = 10 * time.Minute
const cacheVersion = "v2"

type Store interface {
	GetConversation(context.Context, string) (chat.Conversation, bool, error)
	SaveConversation(context.Context, chat.Conversation) error
	ListConversations(context.Context, int) ([]chat.Conversation, error)
	GetCachedAnswer(context.Context, string) (chat.Response, bool, error)
	SetCachedAnswer(context.Context, string, chat.Response) error
	Mode() string
}

func New(cfg config.Config) Store {
	if cfg.RedisEnabled {
		store := newRedisStore(cfg)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := store.ping(ctx); err == nil {
			return store
		}
	}
	return newMemoryStore()
}

func CacheKey(message string, history []chat.Message) string {
	type payload struct {
		Message string         `json:"message"`
		History []chat.Message `json:"history"`
	}
	raw, _ := json.Marshal(payload{
		Message: strings.TrimSpace(message),
		History: history,
	})
	hash := sha256.Sum256(append([]byte(cacheVersion+":"), raw...))
	return hex.EncodeToString(hash[:])
}

type memoryStore struct {
	mu            sync.RWMutex
	conversations map[string]chat.Conversation
	cache         map[string]cachedItem
}

type cachedItem struct {
	Response  chat.Response
	ExpiresAt time.Time
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		conversations: map[string]chat.Conversation{},
		cache:         map[string]cachedItem{},
	}
}

func (m *memoryStore) Mode() string {
	return "memory"
}

func (m *memoryStore) GetConversation(_ context.Context, id string) (chat.Conversation, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	conv, ok := m.conversations[id]
	return conv, ok, nil
}

func (m *memoryStore) SaveConversation(_ context.Context, conv chat.Conversation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.conversations[conv.ID] = conv
	return nil
}

func (m *memoryStore) ListConversations(_ context.Context, limit int) ([]chat.Conversation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conversations := make([]chat.Conversation, 0, len(m.conversations))
	for _, conv := range m.conversations {
		conversations = append(conversations, stripMessages(conv))
	}

	sort.Slice(conversations, func(i, j int) bool {
		return conversations[i].UpdatedAt > conversations[j].UpdatedAt
	})

	if limit > 0 && len(conversations) > limit {
		conversations = conversations[:limit]
	}
	return conversations, nil
}

func (m *memoryStore) GetCachedAnswer(_ context.Context, key string) (chat.Response, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	item, ok := m.cache[key]
	if !ok {
		return chat.Response{}, false, nil
	}
	if time.Now().After(item.ExpiresAt) {
		delete(m.cache, key)
		return chat.Response{}, false, nil
	}
	return item.Response, true, nil
}

func (m *memoryStore) SetCachedAnswer(_ context.Context, key string, response chat.Response) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cache[key] = cachedItem{
		Response:  response,
		ExpiresAt: time.Now().Add(chatCacheTTL),
	}
	return nil
}

type redisStore struct {
	client *redis.Client
}

func newRedisStore(cfg config.Config) *redisStore {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	return &redisStore{client: client}
}

func (r *redisStore) Mode() string {
	return "redis"
}

func (r *redisStore) ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *redisStore) GetConversation(ctx context.Context, id string) (chat.Conversation, bool, error) {
	raw, err := r.client.Get(ctx, conversationKey(id)).Result()
	if err == redis.Nil {
		return chat.Conversation{}, false, nil
	}
	if err != nil {
		return chat.Conversation{}, false, err
	}

	var conv chat.Conversation
	if err := json.Unmarshal([]byte(raw), &conv); err != nil {
		return chat.Conversation{}, false, err
	}
	return conv, true, nil
}

func (r *redisStore) SaveConversation(ctx context.Context, conv chat.Conversation) error {
	raw, err := json.Marshal(conv)
	if err != nil {
		return err
	}

	pipe := r.client.TxPipeline()
	pipe.Set(ctx, conversationKey(conv.ID), raw, 0)
	pipe.ZAdd(ctx, conversationsIndexKey(), redis.Z{
		Score:  float64(parseUnix(conv.UpdatedAt)),
		Member: conv.ID,
	})
	_, err = pipe.Exec(ctx)
	return err
}

func (r *redisStore) ListConversations(ctx context.Context, limit int) ([]chat.Conversation, error) {
	if limit <= 0 {
		limit = 20
	}

	ids, err := r.client.ZRevRange(ctx, conversationsIndexKey(), 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}

	conversations := make([]chat.Conversation, 0, len(ids))
	for _, id := range ids {
		conv, ok, err := r.GetConversation(ctx, id)
		if err != nil || !ok {
			continue
		}
		conversations = append(conversations, stripMessages(conv))
	}
	return conversations, nil
}

func (r *redisStore) GetCachedAnswer(ctx context.Context, key string) (chat.Response, bool, error) {
	raw, err := r.client.Get(ctx, cacheKey(key)).Result()
	if err == redis.Nil {
		return chat.Response{}, false, nil
	}
	if err != nil {
		return chat.Response{}, false, err
	}

	var response chat.Response
	if err := json.Unmarshal([]byte(raw), &response); err != nil {
		return chat.Response{}, false, err
	}
	return response, true, nil
}

func (r *redisStore) SetCachedAnswer(ctx context.Context, key string, response chat.Response) error {
	raw, err := json.Marshal(response)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, cacheKey(key), raw, chatCacheTTL).Err()
}

func conversationKey(id string) string {
	return "conversation:" + id
}

func conversationsIndexKey() string {
	return "conversations:index"
}

func cacheKey(key string) string {
	return "chat-cache:" + key
}

func stripMessages(conv chat.Conversation) chat.Conversation {
	conv.Messages = nil
	return conv
}

func parseUnix(value string) int64 {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Now().Unix()
	}
	return t.Unix()
}

func TitleFromMessage(message string) string {
	trimmed := strings.Join(strings.Fields(strings.TrimSpace(message)), " ")
	if trimmed == "" {
		return "New conversation"
	}
	runes := []rune(trimmed)
	if len(runes) > 48 {
		return string(runes[:48]) + "..."
	}
	return trimmed
}

func NewConversationID() string {
	return fmt.Sprintf("conv_%d", time.Now().UnixNano())
}
