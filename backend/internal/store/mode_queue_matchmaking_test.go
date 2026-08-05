package store

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/playhub/internal/gameclient"
)

func TestJoinModeQueueIdempotentWhileWaiting(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	queueID := DemoDefaultQueueID
	user, err := st.CreateUser(ctx, CreateUserParams{Email: "idem-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(user.ID)

	first, err := st.JoinModeQueue(ctx, queueID, user.ID, "", nil)
	if err != nil {
		t.Fatalf("first join: %v", err)
	}
	if first.Status != QueueStatusWaiting {
		t.Fatalf("expected waiting, got %s", first.Status)
	}

	second, err := st.JoinModeQueue(ctx, queueID, user.ID, "", nil)
	if err != nil {
		t.Fatalf("second join: %v", err)
	}
	if !second.AlreadyInQueue {
		t.Fatalf("expected AlreadyInQueue on re-join")
	}
}

func TestJoinModeQueueReplacesStaleWaitingRowForSameGame(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	queueID := DemoDefaultQueueID
	user, err := st.CreateUser(ctx, CreateUserParams{Email: "stale-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(user.ID)

	// Simulate a legacy waiting row (no mode_queue_id) that blocks the per-game unique index.
	_, err = st.db.ExecContext(ctx, `
		INSERT INTO game_queues (game_id, user_id, status)
		VALUES ($1, $2, 'waiting')
	`, DemoPrimaryGameID, user.ID)
	if err != nil {
		t.Fatalf("insert legacy queue row: %v", err)
	}
	result, err := st.JoinModeQueue(ctx, queueID, user.ID, "", nil)
	if err != nil {
		t.Fatalf("JoinModeQueue after legacy row: %v", err)
	}
	if result.Status != QueueStatusWaiting {
		t.Fatalf("expected waiting, got %s", result.Status)
	}

	var waitingCount int
	if err := st.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM game_queues
		WHERE user_id = $1 AND status = 'waiting' AND mode_queue_id = $2
	`, user.ID, queueID).Scan(&waitingCount); err != nil {
		t.Fatalf("count waiting: %v", err)
	}
	if waitingCount != 1 {
		t.Fatalf("expected 1 mode-queue waiting row, got %d", waitingCount)
	}
}

func TestJoinModeQueueRejectsWhenAlreadyMatched(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	queueID := DemoDefaultQueueID

	userA, err := st.CreateUser(ctx, CreateUserParams{Email: "matched-a-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser A: %v", err)
	}
	cleaner.TrackUser(userA.ID)

	userB, err := st.CreateUser(ctx, CreateUserParams{Email: "matched-b-" + uuid.NewString() + "@example.com"})
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

	_, err = st.JoinModeQueue(ctx, queueID, userA.ID, "", nil)
	if err == nil {
		t.Fatal("expected error when joining queue while already matched")
	}
}

func TestJoinModeQueueCompositionMatchmaking(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	slug := "comp-" + uuid.NewString()
	manifest := &gameclient.Manifest{
		Modes: []gameclient.ModeManifest{{
			Key:          "roles",
			DisplayName:  "Roles",
			SeatTemplate: json.RawMessage(`{"Team":{"count":1,"DPS":{"count":1},"Tank":{"count":1}}}`),
		}},
		Status:     gameclient.StatusResponse{Game: "Comp Test", Version: "1.0.0"},
		ETag:       `"comp"`,
		RawJSON:    []byte(`{"modes":[{"key":"roles"}]}`),
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

	modes, err := st.ListGameModesByGameID(ctx, result.Game.ID)
	if err != nil {
		t.Fatalf("ListGameModesByGameID: %v", err)
	}
	queues, err := st.ListModeQueuesByModeID(ctx, modes[0].ID)
	if err != nil {
		t.Fatalf("ListModeQueuesByModeID: %v", err)
	}
	queueID := queues[0].ID

	userDPS, err := st.CreateUser(ctx, CreateUserParams{Email: "dps-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser dps: %v", err)
	}
	cleaner.TrackUser(userDPS.ID)
	userTank, err := st.CreateUser(ctx, CreateUserParams{Email: "tank-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser tank: %v", err)
	}
	cleaner.TrackUser(userTank.ID)

	first, err := st.JoinModeQueue(ctx, queueID, userDPS.ID, "DPS", nil)
	if err != nil {
		t.Fatalf("join dps: %v", err)
	}
	if first.Status != QueueStatusWaiting {
		t.Fatalf("expected waiting, got %s", first.Status)
	}

	if _, err := st.JoinModeQueue(ctx, queueID, userTank.ID, "Tank", nil); err != nil {
		t.Fatalf("join tank: %v", err)
	}
	rec := mustReconcileForming(t, st, ctx, queueID)
	if !rec.Fired || rec.SessionID == nil {
		t.Fatalf("expected match, got %+v", rec)
	}

	participants, err := st.ListSessionSeatAssignments(ctx, *rec.SessionID)
	if err != nil {
		t.Fatalf("ListSessionParticipants: %v", err)
	}
	if len(participants) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(participants))
	}
	seatKeys := map[string]struct{}{}
	for _, p := range participants {
		seatKeys[p.SeatKey] = struct{}{}
	}
	if _, ok := seatKeys["Team-1-DPS-1"]; !ok {
		t.Fatalf("expected DPS seat, got %+v", seatKeys)
	}
	if _, ok := seatKeys["Team-1-Tank-1"]; !ok {
		t.Fatalf("expected Tank seat, got %+v", seatKeys)
	}
}

func TestJoinModeQueueWordHuntFiresAtPartialCohortSizes(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	slug := "wordhunt-" + uuid.NewString()
	manifest := &gameclient.Manifest{
		Modes: []gameclient.ModeManifest{{
			Key:         "party",
			DisplayName: "Word Hunt Party",
			Min:         4,
			Max:         9,
			SeatTemplate: json.RawMessage(`{
				"ClueGiver":{"displayName":"Clue Giver","name":["Red","Blue","Green"],"min":2,"max":3,"sizeForQueue":2},
				"Guesser":{"count":6,"min":2,"max":6,"sizeForQueue":4}
			}`),
		}},
		Status:     gameclient.StatusResponse{Game: "Word Hunt", Version: "1.0.0"},
		ETag:       `"wh"`,
		RawJSON:    []byte(`{"modes":[{"key":"party"}]}`),
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

	modes, err := st.ListGameModesByGameID(ctx, result.Game.ID)
	if err != nil {
		t.Fatalf("ListGameModesByGameID: %v", err)
	}
	queues, err := st.ListModeQueuesByModeID(ctx, modes[0].ID)
	if err != nil {
		t.Fatalf("ListModeQueuesByModeID: %v", err)
	}
	if queues[0].PlayersToStart != 6 {
		t.Fatalf("players_to_start = %d, want 6", queues[0].PlayersToStart)
	}
	queueID := queues[0].ID

	users := make([]uuid.UUID, 6)
	for i := range users {
		user, err := st.CreateUser(ctx, CreateUserParams{Email: fmt.Sprintf("wh-%d-%s@example.com", i, uuid.NewString())})
		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		cleaner.TrackUser(user.ID)
		users[i] = user.ID
	}

	paths := []string{"ClueGiver", "ClueGiver", "Guesser", "Guesser", "Guesser", "Guesser"}
	var rec *FormingReconcileResult
	for i, path := range paths {
		if _, err = st.JoinModeQueue(ctx, queueID, users[i], path, nil); err != nil {
			t.Fatalf("join %d (%s): %v", i, path, err)
		}
		rec = mustReconcileForming(t, st, ctx, queueID)
		if i < len(paths)-1 && rec.Fired {
			t.Fatalf("expected waiting after join %d, got fired session %s", i, rec.SessionID)
		}
	}
	if !rec.Fired || rec.SessionID == nil {
		t.Fatalf("expected match on 6th join, got %+v", rec)
	}

	participants, err := st.ListSessionSeatAssignments(ctx, *rec.SessionID)
	if err != nil {
		t.Fatalf("ListSessionSeatAssignments: %v", err)
	}
	if len(participants) != 6 {
		t.Fatalf("expected 6 participants, got %d", len(participants))
	}
	seatKeys := map[string]struct{}{}
	for _, p := range participants {
		seatKeys[p.SeatKey] = struct{}{}
	}
	for _, key := range []string{"ClueGiver-Red", "ClueGiver-Blue", "Guesser-1", "Guesser-2", "Guesser-3", "Guesser-4"} {
		if _, ok := seatKeys[key]; !ok {
			t.Fatalf("missing seat %q, got %+v", key, seatKeys)
		}
	}
	if _, ok := seatKeys["ClueGiver-Green"]; ok {
		t.Fatalf("Green should not be assigned at fire size 2 CG")
	}
}
