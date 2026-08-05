package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// Simulates players stuck waiting with queue rows and parties but no forming assignments
// (e.g. after a placement bug). Idempotent re-join should sync all waiting parties and fire.
func TestJoinModeQueueSyncHealsMissedFormingPlacements(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	queueID := DemoDefaultQueueID
	userA, err := st.CreateUser(ctx, CreateUserParams{Email: "sync-heal-a-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser A: %v", err)
	}
	cleaner.TrackUser(userA.ID)
	userB, err := st.CreateUser(ctx, CreateUserParams{Email: "sync-heal-b-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser B: %v", err)
	}
	cleaner.TrackUser(userB.ID)

	first, err := st.JoinModeQueue(ctx, queueID, userA.ID, "", nil)
	if err != nil {
		t.Fatalf("join A: %v", err)
	}
	if first.Status != QueueStatusWaiting {
		t.Fatalf("expected A waiting, got %s", first.Status)
	}

	if _, err := st.JoinModeQueue(ctx, queueID, userB.ID, "", nil); err != nil {
		t.Fatalf("join B: %v", err)
	}
	rec := mustReconcileForming(t, st, ctx, queueID)
	if !rec.Fired || rec.SessionID == nil {
		t.Fatalf("expected immediate match for two solos, got %+v", rec)
	}

	// Reset to stuck state: waiting queue rows, parties intact, empty seat assignments.
	if _, err := st.db.ExecContext(ctx, `
		UPDATE game_queues
		SET status = 'waiting', matched_at = NULL, forming_match_id = NULL
		WHERE mode_queue_id = $1 AND user_id IN ($2, $3)
	`, queueID, userA.ID, userB.ID); err != nil {
		t.Fatalf("reset queue rows: %v", err)
	}
	if _, err := st.db.ExecContext(ctx, `
		UPDATE parties SET status = 'waiting'
		WHERE mode_queue_id = $1
	`, queueID); err != nil {
		t.Fatalf("reset party status: %v", err)
	}
	var formingMatchID uuid.UUID
	if err := st.db.QueryRowContext(ctx, `
		SELECT id FROM forming_matches
		WHERE mode_queue_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, queueID).Scan(&formingMatchID); err != nil {
		t.Fatalf("forming match: %v", err)
	}
	if _, err := st.db.ExecContext(ctx, `
		UPDATE forming_matches
		SET status = 'filling', fired_at = NULL
		WHERE id = $1
	`, formingMatchID); err != nil {
		t.Fatalf("reset forming match: %v", err)
	}
	if _, err := st.db.ExecContext(ctx, `
		UPDATE forming_match_assignments
		SET user_id = NULL, party_id = NULL, source = 'party', table_id = NULL
		WHERE forming_match_id = $1
	`, formingMatchID); err != nil {
		t.Fatalf("clear assignments: %v", err)
	}

	if _, err := st.db.ExecContext(ctx, `
		UPDATE game_session_participants gsp
		SET finished_at = NOW()
		FROM game_sessions gs
		WHERE gsp.session_id = gs.id
		  AND gs.mode_queue_id = $1
		  AND gsp.user_id IN ($2, $3)
		  AND gsp.finished_at IS NULL
	`, queueID, userA.ID, userB.ID); err != nil {
		t.Fatalf("finish session participants: %v", err)
	}
	if _, err := st.db.ExecContext(ctx, `
		UPDATE game_sessions
		SET status = 'completed', ended_at = NOW()
		WHERE mode_queue_id = $1 AND status = 'active'
	`, queueID); err != nil {
		t.Fatalf("complete sessions: %v", err)
	}

	healed, err := st.JoinModeQueue(ctx, queueID, userA.ID, "", nil)
	if err != nil {
		t.Fatalf("idempotent re-join A: %v", err)
	}
	if !healed.AlreadyInQueue {
		t.Fatalf("expected AlreadyInQueue, got %+v", healed)
	}
	healRec := mustReconcileForming(t, st, ctx, queueID)
	if !healRec.Fired || healRec.SessionID == nil {
		t.Fatalf("expected sync heal to fire match, got %+v", healRec)
	}

	participants, err := st.ListSessionSeatAssignments(ctx, *healRec.SessionID)
	if err != nil {
		t.Fatalf("ListSessionSeatAssignments: %v", err)
	}
	if len(participants) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(participants))
	}
}
