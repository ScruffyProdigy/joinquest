package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const oauthStateKeyPrefix = "oauth:state:"

type storedOAuthState struct {
	Provider     string    `json:"provider"`
	Mode         OAuthMode `json:"mode"`
	UserID       string    `json:"userId,omitempty"`
	ConfirmMerge bool      `json:"confirmMerge,omitempty"`
	Next         string    `json:"next,omitempty"`
	PKCEVerifier string    `json:"pkceVerifier,omitempty"`
}

type OAuthStateStore interface {
	Save(ctx context.Context, state OAuthState, pkceVerifier string, ttl time.Duration) (string, error)
	Load(ctx context.Context, id string) (OAuthState, string, error)
	Delete(ctx context.Context, id string) error
}

type memoryOAuthStateStore struct {
	mu    sync.Mutex
	items map[string]storedOAuthState
}

func NewOAuthStateStoreFromEnv() (OAuthStateStore, error) {
	url := strings.TrimSpace(os.Getenv("REDIS_URL"))
	if url == "" {
		return newMemoryOAuthStateStore(), nil
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("auth: parse REDIS_URL for oauth state: %w", err)
	}
	opts.DialTimeout = 2 * time.Second
	opts.ReadTimeout = 2 * time.Second
	opts.WriteTimeout = 2 * time.Second
	opts.PoolTimeout = 3 * time.Second
	client := redis.NewClient(opts)
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("auth: ping redis for oauth state: %w", err)
	}
	return &redisOAuthStateStore{client: client}, nil
}

func newMemoryOAuthStateStore() *memoryOAuthStateStore {
	store := &memoryOAuthStateStore{items: make(map[string]storedOAuthState)}
	go store.reapExpired()
	return store
}

func (s *memoryOAuthStateStore) Save(_ context.Context, state OAuthState, pkceVerifier string, ttl time.Duration) (string, error) {
	id := uuid.NewString()
	payload, err := encodeStoredOAuthState(state, pkceVerifier)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	s.items[id] = payload
	s.mu.Unlock()
	time.AfterFunc(ttl, func() {
		s.mu.Lock()
		delete(s.items, id)
		s.mu.Unlock()
	})
	return id, nil
}

func (s *memoryOAuthStateStore) Load(_ context.Context, id string) (OAuthState, string, error) {
	s.mu.Lock()
	payload, ok := s.items[id]
	s.mu.Unlock()
	if !ok {
		return OAuthState{}, "", ErrOAuthInvalidState
	}
	return decodeStoredOAuthState(payload)
}

func (s *memoryOAuthStateStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	delete(s.items, id)
	s.mu.Unlock()
	return nil
}

func (s *memoryOAuthStateStore) reapExpired() {}

type redisOAuthStateStore struct {
	client *redis.Client
}

func (s *redisOAuthStateStore) Save(ctx context.Context, state OAuthState, pkceVerifier string, ttl time.Duration) (string, error) {
	id := uuid.NewString()
	payload, err := encodeStoredOAuthState(state, pkceVerifier)
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	if err := s.client.Set(ctx, oauthStateKeyPrefix+id, raw, ttl).Err(); err != nil {
		return "", err
	}
	return id, nil
}

func (s *redisOAuthStateStore) Load(ctx context.Context, id string) (OAuthState, string, error) {
	key := oauthStateKeyPrefix + strings.TrimSpace(id)
	raw, err := s.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return OAuthState{}, "", ErrOAuthInvalidState
	}
	if err != nil {
		return OAuthState{}, "", err
	}
	var payload storedOAuthState
	if err := json.Unmarshal(raw, &payload); err != nil {
		return OAuthState{}, "", ErrOAuthInvalidState
	}
	return decodeStoredOAuthState(payload)
}

func (s *redisOAuthStateStore) Delete(ctx context.Context, id string) error {
	key := oauthStateKeyPrefix + strings.TrimSpace(id)
	return s.client.Del(ctx, key).Err()
}

func encodeStoredOAuthState(state OAuthState, pkceVerifier string) (storedOAuthState, error) {
	item := storedOAuthState{
		Provider:     state.Provider,
		Mode:         state.Mode,
		ConfirmMerge: state.ConfirmMerge,
		Next:         NormalizeOAuthNextKey(state.Next),
		PKCEVerifier: strings.TrimSpace(pkceVerifier),
	}
	if state.UserID != uuid.Nil {
		item.UserID = state.UserID.String()
	}
	return item, nil
}

func decodeStoredOAuthState(payload storedOAuthState) (OAuthState, string, error) {
	state := OAuthState{
		Provider:     strings.ToLower(strings.TrimSpace(payload.Provider)),
		Mode:         payload.Mode,
		ConfirmMerge: payload.ConfirmMerge,
		Next:         NormalizeOAuthNextKey(payload.Next),
	}
	if payload.UserID != "" {
		parsed, err := uuid.Parse(payload.UserID)
		if err != nil {
			return OAuthState{}, "", ErrOAuthInvalidState
		}
		state.UserID = parsed
	}
	return state, strings.TrimSpace(payload.PKCEVerifier), nil
}
