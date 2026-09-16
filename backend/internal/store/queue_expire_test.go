package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestExpireStaleMatchedModeQueueClearsOldMatch(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	queueID := DemoDefaultQueueID

	userA, err := st.CreateUser(ctx, CreateUserParams{Email: "expire-a-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser A: %v", err)
	}
	cleaner.TrackUser(userA.ID)

	userB, err := st.CreateUser(ctx, CreateUserParams{Email: "expire-b-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser B: %v", err)
	}
	cleaner.TrackUser(userB.ID)

	if _, err := st.JoinModeQueue(ctx, queueID, userA.ID, "", nil); err != nil {
		t.Fatalf("join A: %v", err)
	}
	if _, err := st.JoinModeQueue(ctx, queueID, userB.ID, "", nil); err != nil {
		t.Fatalf("join B: %v", err)
	}
	mustReconcileForming(t, st, ctx, queueID)

	_, err = st.db.ExecContext(ctx, `
		UPDATE game_queues
		SET matched_at = $2
		WHERE mode_queue_id = $1 AND user_id = $3 AND status = 'matched'
	`, queueID, time.Now().Add(-2*time.Hour), userA.ID)
	if err != nil {
		t.Fatalf("backdate matched_at: %v", err)
	}

	if err := expireStaleMatchedModeQueue(ctx, st.db, queueID, userA.ID); err != nil {
		t.Fatalf("expire stale: %v", err)
	}

	view, err := st.GetUserModeQueueView(ctx, queueID, userA.ID)
	if err != nil {
		t.Fatalf("GetUserModeQueueView: %v", err)
	}
	if view.Matched {
		t.Fatalf("expected stale match cleared, got %+v", view)
	}
}
