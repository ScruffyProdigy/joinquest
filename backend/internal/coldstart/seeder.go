package coldstart

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
)

// SeedStore is the port the serve path reads through.
type SeedStore interface {
	// LoadModePairStats returns every measured pair that could seed
	// targetMode, keyed by source mode.
	LoadModePairStats(ctx context.Context, gameID uuid.UUID, targetMode string) (map[string]PairStats, error)

	// ListSeedSources returns the converged ratings each user holds in other
	// modes of this game.
	ListSeedSources(ctx context.Context, gameID uuid.UUID, excludeMode string, userIDs []uuid.UUID, maxSigma float64) (map[uuid.UUID][]SourceRating, error)

	// RecordRatingSeed writes the audit row for a served seed, and does
	// nothing if one already exists.
	RecordRatingSeed(ctx context.Context, userID, gameID uuid.UUID, modeKey string, seed Seed, at time.Time) error
}

// Seeder answers "what should this player's rating be in a mode they have not
// played?" for a whole roster at once.
//
// # Why seeding happens on read
//
// A seed is never written into player_ratings. That table is a cache of a
// replay over rating_match_inputs, so a seed written there would be erased by
// the next replay — and worse, until it was, it would be indistinguishable
// from a rating the player had earned. Deriving the seed on read keeps the
// replay invariant intact, needs no invalidation, and stops applying by
// itself the moment the player's first result gives them a real rating.
//
// The cost is two extra queries on a roster read that contains unrated
// players, and only when seeding is enabled at all.
type Seeder struct {
	store   SeedStore
	enabled bool
	now     func() time.Time
}

// NewSeeder builds a Seeder. enabled is the flag from configuration: false
// means Seed returns nothing and touches no database, which is what "off by
// default" has to mean if it is to be worth anything.
func NewSeeder(store SeedStore, enabled bool) *Seeder {
	return &Seeder{store: store, enabled: enabled, now: time.Now}
}

// Enabled reports whether this Seeder will produce seeds. A nil Seeder is
// disabled, so a caller that was never given one does not have to check for
// nil separately from checking the flag.
func (s *Seeder) Enabled() bool {
	return s != nil && s.enabled
}

// Seed returns a seeded rating for each of userIDs that qualifies for one,
// and leaves out those that do not. A user absent from the result should be
// served the flat prior.
//
// Every seed served is recorded (see RecordRatingSeed) before it is returned.
// A failure to record is logged and the seed is still served: the audit trail
// is important enough to write on the read path, and not important enough to
// fail a game server's roster lookup over.
//
// Errors reading the pair statistics or the source ratings are returned. The
// caller — which is serving skill for a roster — can then decide, and today
// it treats them as it treats any other store failure rather than silently
// degrading to the prior, because a lookup that quietly answers "we know
// nothing about anyone" is a harder failure to notice than one that says so.
func (s *Seeder) Seed(ctx context.Context, gameID uuid.UUID, modeKey string, userIDs []uuid.UUID) (map[uuid.UUID]Seed, error) {
	if !s.Enabled() || len(userIDs) == 0 {
		return nil, nil
	}

	stats, err := s.store.LoadModePairStats(ctx, gameID, modeKey)
	if err != nil {
		return nil, err
	}
	// Asked before the per-player read: with no usable pair into this mode
	// there is no seed to be had for anyone, and the roster query would be
	// work done to reach that same conclusion one player at a time.
	if !anyUsable(stats) {
		return nil, nil
	}

	sources, err := s.store.ListSeedSources(ctx, gameID, modeKey, userIDs, ConvergedSigma)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, nil
	}

	at := s.now()
	out := make(map[uuid.UUID]Seed, len(sources))
	for _, userID := range userIDs {
		seed, ok := SeedFor(sources[userID], stats)
		if !ok {
			continue
		}
		out[userID] = seed
		if err := s.store.RecordRatingSeed(ctx, userID, gameID, modeKey, seed, at); err != nil {
			log.Printf("coldstart: record seed for %s in %s/%s: %v", userID, gameID, modeKey, err)
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func anyUsable(stats map[string]PairStats) bool {
	for _, p := range stats {
		if p.Usable() {
			return true
		}
	}
	return false
}
