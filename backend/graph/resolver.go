package graph

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/playhub/internal/auth"
	"github.com/scruffyprodigy/playhub/internal/catalogstats"
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
	Store           *store.Store
	Auth            *auth.Service
	PubSub          pubsub.Broker
	ManifestFetcher *gameclient.ManifestFetcher
	SpiritAnimal    *spiritanimal.Runner
	FormingWorker   *formingworker.Worker
	// GameProvisioner pushes match rosters to game APIs; nil uses the default HTTP client.
	GameProvisioner gameclient.MatchProvisioner
	// EligibilityCache resolves GameMode.eligibility; nil uses a default 5s in-memory cache.
	EligibilityCache *gameclient.EligibilityCache
	// LiveCountsCache serves Game.playerActivity; nil queries the store on every field read.
	LiveCountsCache *catalogstats.Cache
}

// NewResolver creates a resolver backed by the store and auth service.
func NewResolver(st *store.Store, authService *auth.Service, broker pubsub.Broker) *Resolver {
	return &Resolver{
		Store:            st,
		Auth:             authService,
		PubSub:           broker,
		EligibilityCache: gameclient.NewEligibilityCache(gameclient.NewClient(), 5*time.Second),
		LiveCountsCache:  catalogstats.NewCache(liveCountsSource(st), 5*time.Second),
	}
}

// liveCountsSource adapts the store's grouped aggregate to the cache's Source signature.
func liveCountsSource(st *store.Store) catalogstats.Source {
	return func(ctx context.Context) (map[uuid.UUID]catalogstats.Counts, error) {
		if st == nil {
			return nil, fmt.Errorf("database store is not configured")
		}
		rows, err := st.CountLivePlayersByGame(ctx)
		if err != nil {
			return nil, err
		}
		counts := make(map[uuid.UUID]catalogstats.Counts, len(rows))
		for gameID, row := range rows {
			counts[gameID] = catalogstats.Counts{Playing: row.Playing, Queued: row.Queued}
		}
		return counts, nil
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
