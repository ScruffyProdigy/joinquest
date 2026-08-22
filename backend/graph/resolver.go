package graph

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/playhub/internal/auth"
	"github.com/scruffyprodigy/playhub/internal/formingworker"
	"github.com/scruffyprodigy/playhub/internal/gameclient"
	"github.com/scruffyprodigy/playhub/internal/pubsub"
	"github.com/scruffyprodigy/playhub/internal/spiritanimal"
	"github.com/scruffyprodigy/playhub/internal/store"
)

func parseUUID(id, label string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid %s", label)
	}
	return parsed, nil
}

type Resolver struct {
	Store             *store.Store
	Auth              *auth.Service
	PubSub            pubsub.Broker
	ManifestFetcher   *gameclient.ManifestFetcher
	SpiritAnimal      *spiritanimal.Runner
	FormingWorker     *formingworker.Worker
	// GameProvisioner pushes match rosters to game APIs; nil uses the default HTTP client.
	GameProvisioner gameclient.MatchProvisioner
	// EligibilityCache resolves GameMode.eligibility; nil uses a default 5s in-memory cache.
	EligibilityCache *gameclient.EligibilityCache
}

// NewResolver creates a resolver backed by the store and auth service.
func NewResolver(st *store.Store, authService *auth.Service, broker pubsub.Broker) *Resolver {
	return &Resolver{
		Store:   st,
		Auth:    authService,
		PubSub:  broker,
	}
}

func (r *Resolver) requireStore() (*store.Store, error) {
	if r == nil || r.Store == nil {
		return nil, fmt.Errorf("database store is not configured")
	}
	return r.Store, nil
}

func (r *Resolver) requireAuth() (*auth.Service, error) {
	if r == nil || r.Auth == nil {
		return nil, fmt.Errorf("auth service is not configured")
	}
	return r.Auth, nil
}

func requireAuthUserID(ctx context.Context) (uuid.UUID, error) {
	userIDStr, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return uuid.Nil, fmt.Errorf("authentication required")
	}
	return parseUUID(userIDStr, "user id")
}
