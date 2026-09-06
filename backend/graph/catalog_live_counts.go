package graph

import (
	"context"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/playhub/internal/catalogstats"
)

// liveCounts reads one game's live counts from the whole-catalog snapshot, so a page of cards
// costs one aggregate rather than one per card. Falls back to querying the store directly when
// no cache is configured (tests and hand-built resolvers).
func (r *Resolver) liveCounts(ctx context.Context, gameID uuid.UUID) (catalogstats.Counts, error) {
	if r.LiveCountsCache != nil {
		return r.LiveCountsCache.For(ctx, gameID)
	}
	st, err := r.requireStore()
	if err != nil {
		return catalogstats.Counts{}, err
	}
	rows, err := st.CountLivePlayersByGame(ctx)
	if err != nil {
		return catalogstats.Counts{}, err
	}
	row := rows[gameID]
	return catalogstats.Counts{Playing: row.Playing, Queued: row.Queued}, nil
}
