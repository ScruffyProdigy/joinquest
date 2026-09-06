package store

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// newIntentTestUser creates a throwaway user tracked for cleanup.
func newIntentTestUser(t *testing.T, st *Store, cleaner *TestCleaner, prefix string) *User {
	t.Helper()
	user, err := st.CreateUser(context.Background(), CreateUserParams{
		Email: prefix + "-" + uuid.NewString() + "@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser(%s): %v", prefix, err)
	}
	cleaner.TrackUser(user.ID)
	return user
}

// matchTwoPlayers puts two fresh users through the demo queue until they hold a
// live session, returning the user under test.
func matchTwoPlayers(t *testing.T, st *Store, cleaner *TestCleaner, prefix string) (*User, uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	userA := newIntentTestUser(t, st, cleaner, prefix+"-a")
	userB := newIntentTestUser(t, st, cleaner, prefix+"-b")

	if _, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, userA.ID, "", nil); err != nil {
		t.Fatalf("join A: %v", err)
	}
	if _, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, userB.ID, "", nil); err != nil {
		t.Fatalf("join B: %v", err)
	}
	mustReconcileForming(t, st, ctx, DemoDefaultQueueID)

	intent, err := st.GetUserActiveIntent(ctx, userA.ID)
	if err != nil {
		t.Fatalf("GetUserActiveIntent: %v", err)
	}
	if intent == nil || !intent.Matched || intent.SessionID == nil {
		t.Fatalf("expected a live matched intent, got %+v", intent)
	}
	return userA, *intent.SessionID
}

// dropSessionMode reproduces the JQ-134 production state: the session outlives the
// catalog mode row it was started from, so game_sessions.mode_id goes NULL.
func dropSessionMode(t *testing.T, st *Store, sessionID uuid.UUID) {
	t.Helper()
	if _, err := st.db.ExecContext(context.Background(), `
		UPDATE game_sessions SET mode_id = NULL WHERE id = $1
	`, sessionID); err != nil {
		t.Fatalf("clear session mode_id: %v", err)
	}
}

// makeOrphanMatchedQueueRow leaves the user holding a matched game_queues row with
// no session behind it.
func makeOrphanMatchedQueueRow(t *testing.T, st *Store, cleaner *TestCleaner, prefix string) (*User, uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	user := newIntentTestUser(t, st, cleaner, prefix)
	if _, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, user.ID, "", nil); err != nil {
		t.Fatalf("join: %v", err)
	}
	var rowID uuid.UUID
	if err := st.db.QueryRowContext(ctx, `
		UPDATE game_queues
		SET status = 'matched', matched_at = NOW()
		WHERE user_id = $1 AND mode_queue_id = $2 AND status = 'waiting'
		RETURNING id
	`, user.ID, DemoDefaultQueueID).Scan(&rowID); err != nil {
		t.Fatalf("simulate matched row: %v", err)
	}
	return user, rowID
}

func queueRowStatus(t *testing.T, st *Store, rowID uuid.UUID) string {
	t.Helper()
	var status string
	if err := st.db.QueryRowContext(context.Background(), `
		SELECT status FROM game_queues WHERE id = $1
	`, rowID).Scan(&status); err != nil {
		t.Fatalf("read queue row status: %v", err)
	}
	return status
}

// TestLeaveActiveGameClearsIntentForModelessSession covers the reported bug: the
// player's session lost its catalog mode, so the participation lookup went blind
// and Leave game did nothing at all (JQ-134).
func TestLeaveActiveGameClearsIntentForModelessSession(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	user, sessionID := matchTwoPlayers(t, st, cleaner, "modeless")
	dropSessionMode(t, st, sessionID)

	intent, err := st.GetUserActiveIntent(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserActiveIntent before leave: %v", err)
	}
	if intent == nil || !intent.Matched {
		t.Fatalf("expected the playing banner to still show, got %+v", intent)
	}

	if _, err := st.LeaveActiveGame(ctx, user.ID); err != nil {
		t.Fatalf("LeaveActiveGame: %v", err)
	}

	after, err := st.GetUserActiveIntent(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserActiveIntent after leave: %v", err)
	}
	if after != nil {
		t.Fatalf("expected no intent after leaving, got %+v", after)
	}
}

// TestLeaveActiveGameCancelsMatchedQueueRowWithoutSession covers the third
// GetUserActiveIntent branch, which had no unwind path of its own.
func TestLeaveActiveGameCancelsMatchedQueueRowWithoutSession(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	user, rowID := makeOrphanMatchedQueueRow(t, st, cleaner, "orphan-matched")

	if _, err := st.LeaveActiveGame(ctx, user.ID); err != nil {
		t.Fatalf("LeaveActiveGame: %v", err)
	}

	if status := queueRowStatus(t, st, rowID); status != "cancelled" {
		t.Fatalf("queue row status = %q, want cancelled", status)
	}

	after, err := st.GetUserActiveIntent(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserActiveIntent after leave: %v", err)
	}
	if after != nil {
		t.Fatalf("expected no intent after leaving, got %+v", after)
	}
}

// TestLeaveActiveGameReportsNothingToLeave keeps "there was nothing to leave"
// distinguishable from "left successfully" for the resolver.
func TestLeaveActiveGameReportsNothingToLeave(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	user := newIntentTestUser(t, st, cleaner, "nothing-to-leave")

	if _, err := st.LeaveActiveGame(ctx, user.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("LeaveActiveGame err = %v, want ErrNotFound", err)
	}
}

// TestEveryActiveIntentStateHasALeavePath walks every state GetUserActiveIntent
// resolves from and asserts a leave path clears it. A fourth branch added without
// one should fail here.
func TestEveryActiveIntentStateHasALeavePath(t *testing.T) {
	cases := []struct {
		name          string
		setup         func(t *testing.T, st *Store, cleaner *TestCleaner) *User
		bannerVisible bool
		leave         func(t *testing.T, st *Store, user *User)
	}{
		{
			name: "active session participation",
			setup: func(t *testing.T, st *Store, cleaner *TestCleaner) *User {
				user, _ := matchTwoPlayers(t, st, cleaner, "branch-session")
				return user
			},
			bannerVisible: true,
			leave: func(t *testing.T, st *Store, user *User) {
				if _, err := st.LeaveActiveGame(context.Background(), user.ID); err != nil {
					t.Fatalf("LeaveActiveGame: %v", err)
				}
			},
		},
		{
			name: "waiting queue entry",
			setup: func(t *testing.T, st *Store, cleaner *TestCleaner) *User {
				user := newIntentTestUser(t, st, cleaner, "branch-waiting")
				if _, err := st.JoinModeQueue(context.Background(), DemoDefaultQueueID, user.ID, "", nil); err != nil {
					t.Fatalf("join: %v", err)
				}
				return user
			},
			bannerVisible: true,
			leave: func(t *testing.T, st *Store, user *User) {
				if _, err := st.LeaveModeQueue(context.Background(), DemoDefaultQueueID, user.ID); err != nil {
					t.Fatalf("LeaveModeQueue: %v", err)
				}
			},
		},
		{
			name: "matched queue row without session",
			setup: func(t *testing.T, st *Store, cleaner *TestCleaner) *User {
				user, _ := makeOrphanMatchedQueueRow(t, st, cleaner, "branch-matched")
				return user
			},
			// GetUserActiveIntent reconciles this row away on read; the leave path
			// still has to unwind it for callers that never read the intent first.
			bannerVisible: false,
			leave: func(t *testing.T, st *Store, user *User) {
				if _, err := st.LeaveActiveGame(context.Background(), user.ID); err != nil {
					t.Fatalf("LeaveActiveGame: %v", err)
				}
			},
		},
		{
			name: "active session participation without a catalog mode",
			setup: func(t *testing.T, st *Store, cleaner *TestCleaner) *User {
				user, sessionID := matchTwoPlayers(t, st, cleaner, "branch-modeless")
				dropSessionMode(t, st, sessionID)
				return user
			},
			bannerVisible: true,
			leave: func(t *testing.T, st *Store, user *User) {
				if _, err := st.LeaveActiveGame(context.Background(), user.ID); err != nil {
					t.Fatalf("LeaveActiveGame: %v", err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := openTestStore(t)
			cleaner := st.NewTestCleaner(t)
			ctx := context.Background()

			user := tc.setup(t, st, cleaner)

			// Only states the banner actually shows get read first: reading the
			// intent reconciles an orphan matched row away, which would leave the
			// leave path with nothing to prove.
			if tc.bannerVisible {
				before, err := st.GetUserActiveIntent(ctx, user.ID)
				if err != nil {
					t.Fatalf("GetUserActiveIntent before leave: %v", err)
				}
				if before == nil {
					t.Fatal("expected an intent banner before leaving")
				}
			}

			tc.leave(t, st, user)

			after, err := st.GetUserActiveIntent(ctx, user.ID)
			if err != nil {
				t.Fatalf("GetUserActiveIntent after leave: %v", err)
			}
			if after != nil {
				t.Fatalf("intent still set after leaving: %+v", after)
			}
		})
	}
}
