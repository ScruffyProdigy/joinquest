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
	if _, err := st.RecordPlayerFinish(ctx, sessionID, userA, "COMPLETED", &placement, map[string]any{"score": 3}); err != nil {
		t.Fatalf("RecordPlayerFinish: %v", err)
	}
	if _, err := st.RecordMatchResult(ctx, sessionID, "COMPLETED", []uuid.UUID{userA}, map[string]any{"rounds": 3}, time.Now()); err != nil {
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

	if _, err := st.RecordMatchResult(ctx, sessionID, "COMPLETED", []uuid.UUID{userA, outsider.ID}, nil, time.Now()); err != nil {
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

// scenarioKeysFromMetadata is a pure function over whatever JSON a game put
// in its result metadata, and §14 of the developer integration guide
// publishes a contract about exactly these branches: a string, an array of
// strings, and the shapes that silently contribute nothing — a number, an
// object, a boolean, an empty string, a whitespace-only string, an empty
// array. "Silently" is what makes them worth pinning: a game that trips one
// gets a successful reportMatchResult and an unrated match, with nothing in
// the response to say why, so the only thing standing behind the documented
// behaviour is this table.
func TestScenarioKeysFromMetadata(t *testing.T) {
	cases := []struct {
		name     string
		metadata map[string]any
		want     []string
	}{
		{"absent", map[string]any{}, nil},
		{"nil metadata", nil, nil},
		{"string", map[string]any{"scenarios": "hard"}, []string{"hard"}},
		{"string is trimmed", map[string]any{"scenarios": "  hard  "}, []string{"hard"}},
		{"empty string", map[string]any{"scenarios": ""}, nil},
		{"whitespace-only string", map[string]any{"scenarios": "   "}, nil},
		{"array", map[string]any{"scenarios": []any{"hard", "night"}}, []string{"hard", "night"}},
		{"empty array", map[string]any{"scenarios": []any{}}, []string{}},
		{"array drops blanks and non-strings", map[string]any{"scenarios": []any{"hard", "", "  ", 7, nil, "night"}}, []string{"hard", "night"}},
		{"number", map[string]any{"scenarios": float64(3)}, nil},
		{"object", map[string]any{"scenarios": map[string]any{"tier": "hard"}}, nil},
		{"boolean", map[string]any{"scenarios": true}, nil},
		{"null", map[string]any{"scenarios": nil}, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := scenarioKeysFromMetadata(tc.metadata)
			// len-and-elements rather than reflect.DeepEqual: an empty array
			// yields an empty non-nil slice and an absent key yields nil, and
			// both mean the same thing to BuildSides (no scenario keys, so a
			// co-op match goes unrated). Pinning that distinction would be
			// pinning an implementation detail the caller cannot observe.
			if len(got) != len(tc.want) {
				t.Fatalf("scenarioKeysFromMetadata(%v) = %v, want %v", tc.metadata, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("scenarioKeysFromMetadata(%v) = %v, want %v", tc.metadata, got, tc.want)
				}
			}
		})
	}
}
