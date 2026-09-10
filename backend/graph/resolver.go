package graph

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/auth"
	"github.com/scruffyprodigy/joinquest/internal/catalogstats"
	"github.com/scruffyprodigy/joinquest/internal/formingworker"
	"github.com/scruffyprodigy/joinquest/internal/gameclient"
	"github.com/scruffyprodigy/joinquest/internal/observe"
	"github.com/scruffyprodigy/joinquest/internal/pubsub"
	"github.com/scruffyprodigy/joinquest/internal/queuewait"
	"github.com/scruffyprodigy/joinquest/internal/spiritanimal"
	"github.com/scruffyprodigy/joinquest/internal/store"
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
	// QueueOptionsCache resolves GameMode.queueOptions and re-checks a join
	// against the roster the player was shown; nil uses a default 5s cache.
	QueueOptionsCache *gameclient.QueueOptionsCache
	// LiveCountsCache serves Game.playerActivity; nil queries the store on every field read.
	LiveCountsCache *catalogstats.Cache
	// Presence turns per-user subscription lifetimes into presence edges, and is what
	// makes a dropped socket a departure signal rather than a debug line. Nil disables
	// presence entirely, which is what tests without a database want; Track is
	// nil-safe, so the subscription resolvers need no special casing.
	Presence *PresenceTracker
	// WaitEstimates serves ModeQueue.estimatedWaitSeconds from one whole-catalog
	// snapshot, so a page of mode cards costs a single estimate rather than one
	// per card; nil builds the default cache over the median strategy. Swapping
	// in a different strategy — one that accounts for the time of day, say — is
	// meant to be the estimator inside this cache and nothing else. See
	// internal/queuewait.
	WaitEstimates *queuewait.Cache
	// Emitter carries operational signals that have no GraphQL surface — conditions a
	// caller cannot be told about because the call legitimately succeeded. nil emits
	// to the log, so a resolver never has to nil-check it.
	Emitter observe.Emitter
}

// waitEstimateTTL is how long one whole-catalog snapshot of wait estimates is
// served for. Longer than the live-counts TTL on purpose: a live count changes
// with every join, while this is a median over days and barely moves minute to
// minute.
const waitEstimateTTL = 30 * time.Second

// waitEstimates returns the resolver's wait-estimate cache, building the
// default one over the median strategy when none was injected.
func (r *Resolver) waitEstimates() *queuewait.Cache {
	if r.WaitEstimates != nil {
		return r.WaitEstimates
	}
	return queuewait.NewCache(newMedianEstimator(r.Store), waitEstimateTTL)
}

// newMedianEstimator builds the default strategy, with its window and sample
// floor taken from the environment.
func newMedianEstimator(st *store.Store) queuewait.MedianEstimator {
	return queuewait.MedianEstimator{
		Samples:    storeFills{st},
		Window:     waitEstimateWindow(),
		MinSamples: waitEstimateMinSamples(),
	}
}

// waitEstimateWindow and waitEstimateMinSamples let production retune the
// estimate without a deploy, the way LOBBY_STALE_PLAYING_MINUTES does for live
// counts. Both return zero on absent or nonsense input, leaving the queuewait
// package's own defaults in charge.
func waitEstimateWindow() time.Duration {
	if v, err := strconv.Atoi(os.Getenv("LOBBY_WAIT_ESTIMATE_WINDOW_DAYS")); err == nil && v > 0 {
		return time.Duration(v) * 24 * time.Hour
	}
	return 0
}

func waitEstimateMinSamples() int {
	if v, err := strconv.Atoi(os.Getenv("LOBBY_WAIT_ESTIMATE_MIN_SAMPLES")); err == nil && v > 0 {
		return v
	}
	return 0
}

// storeFills adapts the store's fill query to queuewait.Samples, keeping that
// generic name off the store itself — the same reason liveCountsSource exists.
type storeFills struct{ store *store.Store }

func (s storeFills) RecentFills(ctx context.Context, q queuewait.FillQuery) (map[queuewait.QueueKey][]queuewait.Fill, error) {
	if s.store == nil {
		return nil, fmt.Errorf("database store is not configured")
	}
	return s.store.RecentModeQueueFills(ctx, q)
}

// signals returns the resolver's emitter, defaulting to the log emitter.
func (r *Resolver) signals() observe.Emitter {
	if r.Emitter == nil {
		return observe.NewLogEmitter()
	}
	return r.Emitter
}

// NewResolver creates a resolver backed by the store and auth service.
func NewResolver(st *store.Store, authService *auth.Service, broker pubsub.Broker) *Resolver {
	return &Resolver{
		Store:             st,
		Auth:              authService,
		PubSub:            broker,
		EligibilityCache:  gameclient.NewEligibilityCache(gameclient.NewClient(), 5*time.Second),
		QueueOptionsCache: gameclient.NewQueueOptionsCache(gameclient.NewClient(), 5*time.Second),
		LiveCountsCache:   catalogstats.NewCache(liveCountsSource(st), 5*time.Second),
		WaitEstimates:     queuewait.NewCache(newMedianEstimator(st), waitEstimateTTL),
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
