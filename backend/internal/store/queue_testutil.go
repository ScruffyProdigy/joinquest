package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// DemoDefaultQueueIDStr is the seeded default mode queue for demo game 001.
const DemoDefaultQueueIDStr = "a3000000-0000-4000-8000-000000000001"

// DemoDefaultQueueID is the parsed form of DemoDefaultQueueIDStr.
var DemoDefaultQueueID = uuid.MustParse(DemoDefaultQueueIDStr)

// DemoPrimaryGameIDStr is seeded catalog game 001 (migrations 000003 / 000009).
// Production uses this row for Word Hunt; integration tests repoint handoff URLs to localhost.
const DemoPrimaryGameIDStr = "a1000000-0000-4000-8000-000000000001"

// DemoPrimaryGameID is the parsed form of DemoPrimaryGameIDStr.
var DemoPrimaryGameID = uuid.MustParse(DemoPrimaryGameIDStr)

const DemoGameAPIBaseURL = "http://localhost:3001"

// RestorePrimaryGameHandoffURLs resets game 001 to local dev api_base_url after tests.
func (s *Store) RestorePrimaryGameHandoffURLs(ctx context.Context) error {
	return s.SetGameAPIBaseURLForTest(ctx, DemoPrimaryGameID, DemoGameAPIBaseURL)
}

// SetGameAPIBaseURLForTest updates api_base_url on a game row (integration tests only).
func (s *Store) SetGameAPIBaseURLForTest(ctx context.Context, gameID uuid.UUID, apiBaseURL string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE games SET api_base_url = $2 WHERE id = $1
	`, gameID, apiBaseURL)
	return err
}

// ClearWaitingQueueForGame removes waiting queue rows for a game (integration tests).
func (s *Store) ClearWaitingQueueForGame(ctx context.Context, gameID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM game_queues
		WHERE game_id = $1 AND status = 'waiting'
	`, gameID)
	return err
}

// mustReconcileForming runs the forming worker step (integration tests).
func mustReconcileForming(t *testing.T, st *Store, ctx context.Context, modeQueueID uuid.UUID) *FormingReconcileResult {
	t.Helper()
	rec, err := st.ReconcileFormingModeQueue(ctx, modeQueueID)
	if err != nil {
		t.Fatalf("ReconcileFormingModeQueue: %v", err)
	}
	return rec
}

// ClearWaitingModeQueue removes waiting rows for a mode queue (integration tests).
func (s *Store) ClearWaitingModeQueue(ctx context.Context, modeQueueID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM game_queues
		WHERE mode_queue_id = $1 AND status = 'waiting'
	`, modeQueueID)
	return err
}

// ResetModeQueueIntegrationState clears queue, party, and forming rows for a mode queue.
func (s *Store) ResetModeQueueIntegrationState(ctx context.Context, modeQueueID uuid.UUID) error {
	if _, err := s.db.ExecContext(ctx, `
		DELETE FROM game_queues WHERE mode_queue_id = $1
	`, modeQueueID); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `
		DELETE FROM forming_match_assignments
		WHERE forming_match_id IN (
			SELECT id FROM forming_matches WHERE mode_queue_id = $1
		)
	`, modeQueueID); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `
		DELETE FROM forming_matches WHERE mode_queue_id = $1
	`, modeQueueID); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `
		DELETE FROM party_members
		WHERE party_id IN (SELECT id FROM parties WHERE mode_queue_id = $1)
	`, modeQueueID); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM parties WHERE mode_queue_id = $1
	`, modeQueueID)
	return err
}
