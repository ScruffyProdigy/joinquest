package store

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/scruffyprodigy/joinquest/internal/rating"
)

func TestReplayOverRealHistoryIsReproducible(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	gameID, modeKey := seedThreeFinishedMatches(t, st, ctx, cleaner)
	e, _ := rating.NewWengLin("plackett-luce")
	r := rating.NewReplayer(e, st.RatingSource())

	if _, err := r.ReplayMode(ctx, gameID.String(), modeKey); err != nil {
		t.Fatalf("first replay: %v", err)
	}
	first, err := st.LoadPlayerRatings(ctx, gameID, modeKey)
	if err != nil {
		t.Fatalf("LoadPlayerRatings: %v", err)
	}

	if err := st.ClearRatings(ctx, gameID, modeKey); err != nil {
		t.Fatalf("ClearRatings: %v", err)
	}
	if _, err := r.ReplayMode(ctx, gameID.String(), modeKey); err != nil {
		t.Fatalf("second replay: %v", err)
	}
	second, _ := st.LoadPlayerRatings(ctx, gameID, modeKey)

	if !reflect.DeepEqual(first, second) {
		t.Errorf("replay from scratch differs:\nfirst  = %+v\nsecond = %+v", first, second)
	}
}

// seedThreeFinishedMatches creates a game and duel mode owned entirely by
// this test (never the shared demo game), records three finished matches
// against it, and returns the game id and mode key to replay. Using a
// test-owned game rather than the demo game means TrackGame's cascading
// delete also clears the rating_match_inputs rows this test appends —
// TestCleaner never deletes those rows itself (see package-level test
// hygiene notes), so a test that reused the demo game would leak input rows
// into every later replay of that mode.
func seedThreeFinishedMatches(t *testing.T, st *Store, ctx context.Context, cleaner *TestCleaner) (uuid.UUID, string) {
	t.Helper()

	game, mode := setupDuelMode(t, st, cleaner)
	queues, err := st.ListModeQueuesByModeID(ctx, mode.ID)
	if err != nil {
		t.Fatalf("ListModeQueuesByModeID: %v", err)
	}
	if len(queues) == 0 {
		t.Fatal("expected at least one mode queue for duel mode")
	}
	queueID := queues[0].ID

	base := time.Now().UTC()
	for i := 0; i < 3; i++ {
		userA, err := st.CreateUser(ctx, CreateUserParams{Email: "replay-a-" + uuid.NewString() + "@example.com"})
		if err != nil {
			t.Fatalf("CreateUser A (match %d): %v", i, err)
		}
		cleaner.TrackUser(userA.ID)

		userB, err := st.CreateUser(ctx, CreateUserParams{Email: "replay-b-" + uuid.NewString() + "@example.com"})
		if err != nil {
			t.Fatalf("CreateUser B (match %d): %v", i, err)
		}
		cleaner.TrackUser(userB.ID)

		if _, err := st.JoinModeQueue(ctx, queueID, userA.ID, "", nil); err != nil {
			t.Fatalf("join A (match %d): %v", i, err)
		}
		if _, err := st.JoinModeQueue(ctx, queueID, userB.ID, "", nil); err != nil {
			t.Fatalf("join B (match %d): %v", i, err)
		}
		mustReconcileForming(t, st, ctx, queueID)

		view, err := st.GetUserActiveIntent(ctx, userA.ID)
		if err != nil {
			t.Fatalf("GetUserActiveIntent (match %d): %v", i, err)
		}
		if view == nil || view.SessionID == nil {
			t.Fatalf("expected matched session for match %d", i)
		}
		sessionID := *view.SessionID

		in := RatingInput{
			SessionID: sessionID,
			GameID:    game.ID,
			ModeKey:   mode.ModeKey,
			RatedAt:   base.Add(time.Duration(i) * time.Second),
			Sides: []RatingSideRow{
				{Rank: 0, Entrants: []RatingEntrantRow{{Key: "player:" + userA.ID.String()}}},
				{Rank: 1, Entrants: []RatingEntrantRow{{Key: "player:" + userB.ID.String()}}},
			},
		}
		if err := st.AppendRatingInput(ctx, in); err != nil {
			t.Fatalf("AppendRatingInput (match %d): %v", i, err)
		}
	}

	return game.ID, mode.ModeKey
}
