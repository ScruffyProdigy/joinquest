package coldstart

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/scruffyprodigy/joinquest/internal/rating"
)

// seedStore is an in-memory SeedStore that also counts its calls, so a test
// can assert not just what was returned but what was asked of the database.
type seedStore struct {
	stats   map[string]PairStats
	sources map[uuid.UUID][]SourceRating

	statsCalls   int
	sourcesCalls int
	recorded     map[uuid.UUID]Seed
	recordErr    error
}

func newSeedStore() *seedStore {
	return &seedStore{
		stats:    make(map[string]PairStats),
		sources:  make(map[uuid.UUID][]SourceRating),
		recorded: make(map[uuid.UUID]Seed),
	}
}

func (s *seedStore) LoadModePairStats(context.Context, uuid.UUID, string) (map[string]PairStats, error) {
	s.statsCalls++
	return s.stats, nil
}

func (s *seedStore) ListSeedSources(_ context.Context, _ uuid.UUID, excludeMode string, userIDs []uuid.UUID, maxSigma float64) (map[uuid.UUID][]SourceRating, error) {
	s.sourcesCalls++
	out := make(map[uuid.UUID][]SourceRating, len(userIDs))
	for _, id := range userIDs {
		for _, src := range s.sources[id] {
			if src.ModeKey == excludeMode || src.Rating.Sigma > maxSigma {
				continue
			}
			out[id] = append(out[id], src)
		}
	}
	return out, nil
}

func (s *seedStore) RecordRatingSeed(_ context.Context, userID, _ uuid.UUID, _ string, seed Seed, _ time.Time) error {
	if s.recordErr != nil {
		return s.recordErr
	}
	s.recorded[userID] = seed
	return nil
}

func usablePair() PairStats {
	return PairStats{
		SourceMode:     "arena",
		TargetMode:     "duel",
		PairedPlayers:  200,
		Correlation:    0.85,
		SourceMean:     25,
		SourceSD:       5,
		TargetMean:     25,
		TargetSD:       5,
		ResidualSD:     5,
		FlatResidualSD: 8,
	}
}

// Off by default has to mean off, not "computes a seed and discards it". A
// disabled seeder must not reach the database at all, which is what makes the
// flag safe to ship dark.
func TestDisabledSeederTouchesNothing(t *testing.T) {
	st := newSeedStore()
	st.stats["arena"] = usablePair()
	st.sources[uuid.New()] = []SourceRating{{ModeKey: "arena", Rating: rating.Rating{Mu: 32, Sigma: 2}}}

	seeds, err := NewSeeder(st, false).Seed(context.Background(), uuid.New(), "duel", []uuid.UUID{uuid.New()})
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if len(seeds) != 0 {
		t.Errorf("disabled seeder returned %d seed(s), want none", len(seeds))
	}
	if st.statsCalls != 0 || st.sourcesCalls != 0 {
		t.Errorf("disabled seeder made %d stats and %d source queries, want none",
			st.statsCalls, st.sourcesCalls)
	}
}

// A nil Seeder is a disabled one, so a caller that was never given one needs
// no nil check of its own.
func TestNilSeederIsDisabled(t *testing.T) {
	var s *Seeder
	if s.Enabled() {
		t.Error("nil Seeder reports Enabled")
	}
	seeds, err := s.Seed(context.Background(), uuid.New(), "duel", []uuid.UUID{uuid.New()})
	if err != nil || len(seeds) != 0 {
		t.Errorf("nil Seeder returned %d seed(s), err %v; want none and nil", len(seeds), err)
	}
}

func TestSeederSeedsAndRecordsProvenance(t *testing.T) {
	userID, gameID := uuid.New(), uuid.New()
	st := newSeedStore()
	st.stats["arena"] = usablePair()
	st.sources[userID] = []SourceRating{{ModeKey: "arena", Rating: rating.Rating{Mu: 35, Sigma: 2}}}

	seeds, err := NewSeeder(st, true).Seed(context.Background(), gameID, "duel", []uuid.UUID{userID})
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}

	seed, ok := seeds[userID]
	if !ok {
		t.Fatal("no seed for a player with a converged rating in a well-measured source mode")
	}
	if seed.Rating.Mu <= 25 {
		t.Errorf("seeded mu = %v, want above the population mean for an above-average player", seed.Rating.Mu)
	}
	if seed.Rating.Sigma >= rating.UnratedSigma {
		t.Errorf("seeded sigma = %v, want tighter than the flat prior %v", seed.Rating.Sigma, rating.UnratedSigma)
	}

	recorded, ok := st.recorded[userID]
	if !ok {
		t.Fatal("seed was served but not recorded; nothing to audit later")
	}
	if recorded != seed {
		t.Errorf("recorded %+v, served %+v", recorded, seed)
	}
}

// With no usable pair into the target mode, no per-player query is worth
// making: the answer is the flat prior for everyone regardless of what any
// individual has played.
func TestSeederSkipsRosterQueryWithoutAUsablePair(t *testing.T) {
	thin := usablePair()
	thin.PairedPlayers = MinPairedPlayers - 1

	st := newSeedStore()
	st.stats["arena"] = thin

	seeds, err := NewSeeder(st, true).Seed(context.Background(), uuid.New(), "duel", []uuid.UUID{uuid.New()})
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if len(seeds) != 0 {
		t.Errorf("returned %d seed(s) from an unusable pair, want none", len(seeds))
	}
	if st.sourcesCalls != 0 {
		t.Errorf("queried roster sources %d time(s) with no usable pair, want none", st.sourcesCalls)
	}
}

// The audit trail is worth writing on the read path and not worth failing a
// game server's roster lookup over.
func TestSeederServesSeedEvenWhenRecordingFails(t *testing.T) {
	userID := uuid.New()
	st := newSeedStore()
	st.stats["arena"] = usablePair()
	st.sources[userID] = []SourceRating{{ModeKey: "arena", Rating: rating.Rating{Mu: 30, Sigma: 2}}}
	st.recordErr = fmt.Errorf("audit table unavailable")

	seeds, err := NewSeeder(st, true).Seed(context.Background(), uuid.New(), "duel", []uuid.UUID{userID})
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if _, ok := seeds[userID]; !ok {
		t.Error("a failed audit write suppressed the seed; want the seed served and the failure logged")
	}
}

// A player whose only other rating is in the mode being seeded is not
// seedable from it — and the exclusion is the store's job, checked here so
// the seeder does not quietly depend on the caller having done it.
func TestSeederIgnoresTheTargetModeAsItsOwnSource(t *testing.T) {
	userID := uuid.New()
	pair := usablePair()
	pair.SourceMode = "duel"

	st := newSeedStore()
	st.stats["duel"] = pair
	st.sources[userID] = []SourceRating{{ModeKey: "duel", Rating: rating.Rating{Mu: 35, Sigma: 2}}}

	seeds, err := NewSeeder(st, true).Seed(context.Background(), uuid.New(), "duel", []uuid.UUID{userID})
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if len(seeds) != 0 {
		t.Errorf("seeded a player into %q from %q, want no seed", "duel", "duel")
	}
}

func TestEnabledFromEnvDefaultsToOff(t *testing.T) {
	t.Setenv(EnabledEnv, "")
	if EnabledFromEnv() {
		t.Error("cold-start seeding is on with the flag unset; it must default to off")
	}

	t.Setenv(EnabledEnv, "true")
	if !EnabledFromEnv() {
		t.Error("cold-start seeding is off with the flag set to true")
	}
}

func TestRefitIntervalFromEnv(t *testing.T) {
	t.Setenv(RefitIntervalEnv, "")
	if got := RefitIntervalFromEnv(); got != DefaultRefitInterval {
		t.Errorf("interval = %v, want the default %v", got, DefaultRefitInterval)
	}

	t.Setenv(RefitIntervalEnv, "90s")
	if got := RefitIntervalFromEnv(); got != 90*time.Second {
		t.Errorf("interval = %v, want 90s", got)
	}

	// An unparseable or non-positive value falls back rather than disabling
	// the refit, which would leave seeding measuring nothing.
	t.Setenv(RefitIntervalEnv, "not-a-duration")
	if got := RefitIntervalFromEnv(); got != DefaultRefitInterval {
		t.Errorf("interval = %v for an invalid value, want the default %v", got, DefaultRefitInterval)
	}
}
