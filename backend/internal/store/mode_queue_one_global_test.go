package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestJoinModeQueueSwitchesFromOtherQueueWhileWaiting(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	user, err := st.CreateUser(ctx, CreateUserParams{Email: "switch-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(user.ID)

	secondQueueID := uuid.MustParse("b3000000-0000-4000-8000-000000000099")
	secondModeID := uuid.MustParse("b2000000-0000-4000-8000-000000000099")
	_, err = st.db.ExecContext(ctx, `
		INSERT INTO game_modes (id, game_id, mode_key, display_name, min_players, max_players, seat_template, status)
		VALUES ($1, $2, 'alt', 'Alt mode', 2, 2, '{"count":2}'::jsonb, 'active')
		ON CONFLICT (game_id, mode_key) DO NOTHING
	`, secondModeID, DemoPrimaryGameID)
	if err != nil {
		t.Fatalf("insert mode: %v", err)
	}
	_, err = st.db.ExecContext(ctx, `
		INSERT INTO game_mode_seats (mode_id, seat_key, sort_order)
		VALUES ($1, '1', 0), ($1, '2', 1)
		ON CONFLICT (mode_id, seat_key) DO NOTHING
	`, secondModeID)
	if err != nil {
		t.Fatalf("insert mode: %v", err)
	}
	_, err = st.db.ExecContext(ctx, `
		INSERT INTO mode_queues (id, mode_id, name, players_to_start, status, is_default)
		VALUES ($1, $2, 'Alt queue', 2, 'active', false)
		ON CONFLICT (mode_id, name) DO NOTHING
	`, secondQueueID, secondModeID)
	if err != nil {
		t.Fatalf("insert queue: %v", err)
	}
	t.Cleanup(func() {
		_, _ = st.db.ExecContext(context.Background(), `DELETE FROM mode_queues WHERE id = $1`, secondQueueID)
		_, _ = st.db.ExecContext(context.Background(), `DELETE FROM game_modes WHERE id = $1`, secondModeID)
	})

	first, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, user.ID, "", nil)
	if err != nil {
		t.Fatalf("first join: %v", err)
	}
	if first.Status != QueueStatusWaiting {
		t.Fatalf("expected waiting, got %s", first.Status)
	}

	second, err := st.JoinModeQueue(ctx, secondQueueID, user.ID, "", nil)
	if err != nil {
		t.Fatalf("second join: %v", err)
	}
	if second.SwitchedFrom == nil || second.SwitchedFrom.ModeQueueID != DemoDefaultQueueID {
		t.Fatalf("expected switched from default queue, got %+v", second.SwitchedFrom)
	}
	if second.Status != QueueStatusWaiting {
		t.Fatalf("expected waiting in new queue, got %s", second.Status)
	}

	var waitingDefault int
	if err := st.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM game_queues
		WHERE user_id = $1 AND mode_queue_id = $2 AND status = 'waiting'
	`, user.ID, DemoDefaultQueueID).Scan(&waitingDefault); err != nil {
		t.Fatalf("count: %v", err)
	}
	if waitingDefault != 0 {
		t.Fatalf("expected no waiting row on first queue, got %d", waitingDefault)
	}
}
