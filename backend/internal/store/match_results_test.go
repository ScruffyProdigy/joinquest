package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRecordMatchResultPersistsWinnersAndStatus(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	sessionID, userA, userB := seedMatchedSession(t, st, ctx, cleaner)

	placement := 1
	if err := st.RecordPlayerFinish(ctx, sessionID, userA, "COMPLETED", &placement, map[string]any{"score": 3}); err != nil {
		t.Fatalf("RecordPlayerFinish: %v", err)
	}
	if err := st.RecordMatchResult(ctx, sessionID, "COMPLETED", []uuid.UUID{userA}, map[string]any{"rounds": 3}, time.Now()); err != nil {
		t.Fatalf("RecordMatchResult: %v", err)
	}

	result, err := st.GetMatchResult(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetMatchResult: %v", err)
	}
	if result.Status == nil || *result.Status != "COMPLETED" {
		t.Fatalf("status = %v, want COMPLETED", result.Status)
	}
	if len(result.Participants) != 2 {
		t.Fatalf("participants = %d, want 2", len(result.Participants))
	}
	for _, p := range result.Participants {
		switch p.UserID {
		case userA:
			if !p.IsWinner {
				t.Error("userA should be the winner")
			}
			if p.Placement == nil || *p.Placement != 1 {
				t.Errorf("userA placement = %v, want 1", p.Placement)
			}
			if p.Reason == nil || *p.Reason != "COMPLETED" {
				t.Errorf("userA reason = %v, want COMPLETED", p.Reason)
			}
		case userB:
			if p.IsWinner {
				t.Error("userB should not be the winner")
			}
			if p.Reason != nil {
				t.Errorf("userB reason = %v, want nil (game never reported them)", p.Reason)
			}
		}
	}
}

func TestRecordMatchResultIgnoresNonParticipantWinners(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	sessionID, userA, _ := seedMatchedSession(t, st, ctx, cleaner)

	outsider, err := st.CreateUser(ctx, CreateUserParams{Email: "outsider-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(outsider.ID)

	if err := st.RecordMatchResult(ctx, sessionID, "COMPLETED", []uuid.UUID{userA, outsider.ID}, nil, time.Now()); err != nil {
		t.Fatalf("RecordMatchResult: %v", err)
	}

	result, err := st.GetMatchResult(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetMatchResult: %v", err)
	}
	for _, p := range result.Participants {
		if p.UserID == outsider.ID {
			t.Fatal("non-participant appeared in the result roster")
		}
	}
	winners := 0
	for _, p := range result.Participants {
		if p.IsWinner {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("winners = %d, want 1 (the outsider must be dropped)", winners)
	}
}

// seedMatchedSession creates two users, matches them through the demo queue, and
// returns the resulting session id plus both user ids.
func seedMatchedSession(t *testing.T, st *Store, ctx context.Context, cleaner *TestCleaner) (uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	queueID := DemoDefaultQueueID

	userA, err := st.CreateUser(ctx, CreateUserParams{Email: "res-a-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser A: %v", err)
	}
	cleaner.TrackUser(userA.ID)
	userB, err := st.CreateUser(ctx, CreateUserParams{Email: "res-b-" + uuid.NewString() + "@example.com"})
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

	view, err := st.GetUserActiveIntent(ctx, userA.ID)
	if err != nil {
		t.Fatalf("GetUserActiveIntent: %v", err)
	}
	if view == nil || view.SessionID == nil {
		t.Fatal("expected a matched session")
	}
	return *view.SessionID, userA.ID, userB.ID
}
