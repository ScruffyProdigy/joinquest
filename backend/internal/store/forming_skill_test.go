package store

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/gameclient"
)

// duelQueueForSkill registers a two-seat mode and returns its queue.
func duelQueueForSkill(t *testing.T, st *Store, cleaner *TestCleaner, ctx context.Context) (uuid.UUID, uuid.UUID) {
	t.Helper()

	slug := "skilldu-" + uuid.NewString()
	manifest := &gameclient.Manifest{
		Modes: []gameclient.ModeManifest{{
			Key:          "duel",
			DisplayName:  "Duel",
			SeatTemplate: json.RawMessage(`{"Player":{"count":2,"min":2,"max":2,"sizeForQueue":2}}`),
		}},
		Status:     gameclient.StatusResponse{Game: "Skill Duel", Version: "1.0.0"},
		ETag:       `"skilldu"`,
		RawJSON:    []byte(`{"modes":[{"key":"duel"}]}`),
		SHA256Hash: uuid.NewString(),
	}
	result, err := st.RegisterGame(ctx, RegisterGameParams{
		Slug:       slug,
		IconURL:    "/games/default.svg",
		HeroURL:    "/games/default-hero.svg",
		APIBaseURL: "https://api.example.com/" + slug,
	}, manifest)
	if err != nil {
		t.Fatalf("RegisterGame: %v", err)
	}
	cleaner.TrackGame(result.Game.ID)
	queue, err := st.GetDefaultModeQueueForGame(ctx, result.Game.ID)
	if err != nil {
		t.Fatalf("GetDefaultModeQueueForGame: %v", err)
	}
	return result.Game.ID, queue.ID
}

// rateUser pins a player's mu for this game and mode.
func rateUser(t *testing.T, st *Store, ctx context.Context, gameID uuid.UUID, userID uuid.UUID, mu float64) {
	t.Helper()
	if err := st.SaveRatings(ctx, gameID, "duel", "test-engine@1", time.Now(), map[string]RatingValue{
		PlayerRatingKey(userID): {Mu: mu, Sigma: 1, MatchesPlayed: 20},
	}, nil); err != nil {
		t.Fatalf("SaveRatings: %v", err)
	}
}

// seedArrivals raises the measured arrival rate on a line by writing rows that
// joined recently. Status does not matter to the arrival leg, only joined_at.
func seedArrivals(t *testing.T, st *Store, cleaner *TestCleaner, ctx context.Context, gameID, queueID uuid.UUID, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		user, err := st.CreateUser(ctx, CreateUserParams{Email: fmt.Sprintf("arr-%d-%s@example.com", i, uuid.NewString())})
		if err != nil {
			t.Fatalf("CreateUser arrival: %v", err)
		}
		cleaner.TrackUser(user.ID)
		if _, err := st.db.ExecContext(ctx, `
			INSERT INTO game_queues (game_id, user_id, status, mode_queue_id, joined_at)
			VALUES ($1, $2, 'cancelled', $3, NOW() - ($4 || ' seconds')::interval)
		`, gameID, user.ID, queueID, (i*3)%800); err != nil {
			t.Fatalf("seed arrival: %v", err)
		}
	}
}

// The load-bearing pair. On a line busy enough that waiting could pay, a lobby
// whose players sit at opposite ends of the scale does not fire -- and it fires
// anyway once the oldest waiter has spent the budget, because skill matching
// must never be a way to never fill a queue.
func TestABusyQueueDefersAWideLobbyAndFiresAnywayOnceTheBudgetIsSpent(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	gameID, queueID := duelQueueForSkill(t, st, cleaner, ctx)
	seedArrivals(t, st, cleaner, ctx, gameID, queueID, 200)

	low, err := st.CreateUser(ctx, CreateUserParams{Email: "low-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser low: %v", err)
	}
	cleaner.TrackUser(low.ID)
	high, err := st.CreateUser(ctx, CreateUserParams{Email: "high-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser high: %v", err)
	}
	cleaner.TrackUser(high.ID)

	// Far outside both the band and the hard cap.
	rateUser(t, st, ctx, gameID, low.ID, 5)
	rateUser(t, st, ctx, gameID, high.ID, 45)

	for _, id := range []uuid.UUID{low.ID, high.ID} {
		if _, err := st.JoinModeQueue(ctx, queueID, id, "", nil); err != nil {
			t.Fatalf("JoinModeQueue: %v", err)
		}
	}

	if rec := mustReconcileForming(t, st, ctx, queueID); rec.Fired {
		t.Fatalf("a wide lobby fired on a busy queue with budget left; want a deferral")
	}

	// Age the oldest waiter past the ceiling. Nothing else changes.
	if _, err := st.db.ExecContext(ctx, `
		UPDATE game_queues
		SET joined_at = NOW() - interval '60 seconds'
		WHERE mode_queue_id = $1 AND status = 'waiting'
	`, queueID); err != nil {
		t.Fatalf("age the waiters: %v", err)
	}

	rec := mustReconcileForming(t, st, ctx, queueID)
	if !rec.Fired || rec.SessionID == nil {
		t.Fatalf("the budget is spent and the lobby still did not fire: %+v", rec)
	}
}

// The property that makes this need no launch flag: a queue nobody else is
// joining behaves exactly as it did before skill matching existed, however far
// apart its two players are.
func TestAThinQueueFiresAWideLobbyImmediately(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	gameID, queueID := duelQueueForSkill(t, st, cleaner, ctx)

	low, err := st.CreateUser(ctx, CreateUserParams{Email: "tlow-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(low.ID)
	high, err := st.CreateUser(ctx, CreateUserParams{Email: "thigh-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(high.ID)

	rateUser(t, st, ctx, gameID, low.ID, 5)
	rateUser(t, st, ctx, gameID, high.ID, 45)

	for _, id := range []uuid.UUID{low.ID, high.ID} {
		if _, err := st.JoinModeQueue(ctx, queueID, id, "", nil); err != nil {
			t.Fatalf("JoinModeQueue: %v", err)
		}
	}

	rec := mustReconcileForming(t, st, ctx, queueID)
	if !rec.Fired || rec.SessionID == nil {
		t.Fatalf("a thin queue must fire unbiased, got %+v", rec)
	}
}

// The per-mode opt-out, end to end: the same busy queue and the same wide lobby
// that defers above must fire immediately when the mode has switched skill
// matching off. A mode with a meaningless skill signal should not pay for one.
func TestAModeWithSkillMatchingOffFiresAWideLobbyOnABusyQueue(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	gameID, queueID := duelQueueForSkill(t, st, cleaner, ctx)
	seedArrivals(t, st, cleaner, ctx, gameID, queueID, 200)

	queue, err := st.GetModeQueueByID(ctx, queueID)
	if err != nil {
		t.Fatalf("GetModeQueueByID: %v", err)
	}
	if _, err := st.db.ExecContext(ctx, `
		UPDATE game_modes SET skill_matching_enabled = false WHERE id = $1
	`, queue.ModeID); err != nil {
		t.Fatalf("disable skill matching: %v", err)
	}

	low, err := st.CreateUser(ctx, CreateUserParams{Email: "olow-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(low.ID)
	high, err := st.CreateUser(ctx, CreateUserParams{Email: "ohigh-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(high.ID)

	rateUser(t, st, ctx, gameID, low.ID, 5)
	rateUser(t, st, ctx, gameID, high.ID, 45)

	for _, id := range []uuid.UUID{low.ID, high.ID} {
		if _, err := st.JoinModeQueue(ctx, queueID, id, "", nil); err != nil {
			t.Fatalf("JoinModeQueue: %v", err)
		}
	}

	rec := mustReconcileForming(t, st, ctx, queueID)
	if !rec.Fired || rec.SessionID == nil {
		t.Fatalf("skill matching is off and the lobby still did not fire: %+v", rec)
	}
}
