package store

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
)

const defaultStaleMatchMinutes = 30

// staleMatchMaxAge returns how long a matched queue row may persist before auto-expiry.
func staleMatchMaxAge() time.Duration {
	if v := os.Getenv("LOBBY_STALE_MATCH_MINUTES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Minute
		}
	}
	return defaultStaleMatchMinutes * time.Minute
}

// expireStaleMatchedModeQueue cancels a match the player never acted on.
//
// The cutoff is built in SQL rather than here so that it and matched_at are read
// off the same clock. A cutoff computed in this process is the app server's clock,
// and the difference between the two machines would be added to the window.
func expireStaleMatchedModeQueue(ctx context.Context, exec sqlExecContext, modeQueueID, userID uuid.UUID) error {
	_, err := exec.ExecContext(ctx, `
		UPDATE game_queues
		SET status = 'cancelled'
		WHERE mode_queue_id = $1
		  AND user_id = $2
		  AND status = 'matched'
		  AND (matched_at IS NULL OR NOW() - matched_at >= make_interval(secs => $3))
	`, modeQueueID, userID, staleMatchMaxAge().Seconds())
	return err
}

func (s *Store) cancelUserMatchedModeQueue(ctx context.Context, modeQueueID, userID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE game_queues
		SET status = 'cancelled'
		WHERE mode_queue_id = $1 AND user_id = $2 AND status = 'matched'
	`, modeQueueID, userID)
	return err
}
