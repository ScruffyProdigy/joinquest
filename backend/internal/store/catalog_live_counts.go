package store

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// A session only ends when the game server reports a result, so a game that crashes mid-match
// can leave rows sitting in 'active' forever. Ignore those instead of letting a catalog card
// claim players that went home hours ago.
const defaultStalePlayingMinutes = 240

func stalePlayingMaxAge() time.Duration {
	if v := os.Getenv("LOBBY_STALE_PLAYING_MINUTES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Minute
		}
	}
	return defaultStalePlayingMinutes * time.Minute
}

// GameLiveCounts is how many players a game has right now: seated in a live session, and
// waiting in its queues. They are separate numbers, not two views of the same players.
type GameLiveCounts struct {
	Playing int
	Queued  int
}

// CountLivePlayersByGame returns live counts for every game that has any, keyed by game id.
// Games with no activity are absent from the map — callers read a missing key as two zeroes.
//
// This is one grouped aggregate for the whole catalog rather than a query per card, so the
// cost does not grow with the number of games on the page.
func (s *Store) CountLivePlayersByGame(ctx context.Context) (map[uuid.UUID]GameLiveCounts, error) {
	cutoff := time.Now().Add(-stalePlayingMaxAge())

	rows, err := s.db.QueryContext(ctx, `
		SELECT game_id, SUM(playing)::int, SUM(queued)::int
		FROM (
		    SELECT s.game_id, COUNT(*) AS playing, 0 AS queued
		    FROM game_session_participants sp
		    INNER JOIN game_sessions s ON s.id = sp.session_id
		    WHERE s.status = 'active'
		      AND s.game_id IS NOT NULL
		      AND s.started_at >= $1
		      AND sp.left_at IS NULL
		      AND sp.finished_at IS NULL
		    GROUP BY s.game_id
		    UNION ALL
		    SELECT q.game_id, 0 AS playing, COUNT(*) AS queued
		    FROM game_queues q
		    WHERE q.status = 'waiting'
		      AND q.game_id IS NOT NULL
		    GROUP BY q.game_id
		) live
		GROUP BY game_id
	`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[uuid.UUID]GameLiveCounts)
	for rows.Next() {
		var gameID uuid.UUID
		var c GameLiveCounts
		if err := rows.Scan(&gameID, &c.Playing, &c.Queued); err != nil {
			return nil, err
		}
		counts[gameID] = c
	}
	return counts, rows.Err()
}
