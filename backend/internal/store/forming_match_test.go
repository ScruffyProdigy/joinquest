package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/playhub/internal/gameclient"
	"github.com/scruffyprodigy/playhub/internal/lfg/partytree"
)

func TestJoinModeQueueSplitPartyWordHunt(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	slug := "wh-split-" + uuid.NewString()
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
		ETag:       `"wh-split"`,
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
	queueID := queues[0].ID

	pat, err := st.CreateUser(ctx, CreateUserParams{Email: "pat-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser pat: %v", err)
	}
	cleaner.TrackUser(pat.ID)
	bro, err := st.CreateUser(ctx, CreateUserParams{Email: "bro-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser bro: %v", err)
	}
	cleaner.TrackUser(bro.ID)

	solos := make([]uuid.UUID, 4)
	for i := range solos {
		user, err := st.CreateUser(ctx, CreateUserParams{Email: fmt.Sprintf("solo-%d-%s@example.com", i, uuid.NewString())})
		if err != nil {
			t.Fatalf("CreateUser solo: %v", err)
		}
		cleaner.TrackUser(user.ID)
		solos[i] = user.ID
	}

	partyResult, err := st.JoinModeQueue(ctx, queueID, pat.ID, "ClueGiver", &JoinPartyInput{
		Tree: partytree.Node{Children: []partytree.Node{
			{Role: "ClueGiver", Members: []string{pat.ID.String()}},
			{Role: "Guesser", Members: []string{bro.ID.String()}},
		}},
		Members: []JoinPartyMemberInput{
			{UserID: pat.ID, QueuePath: "ClueGiver"},
			{UserID: bro.ID, QueuePath: "Guesser"},
		},
	})
	if err != nil {
		t.Fatalf("split party join: %v", err)
	}
	if partyResult.Status != QueueStatusWaiting {
		t.Fatalf("expected waiting after partial party, got %s", partyResult.Status)
	}

	soloPaths := []string{"ClueGiver", "Guesser", "Guesser", "Guesser"}
	var rec *FormingReconcileResult
	for i, path := range soloPaths {
		if _, err = st.JoinModeQueue(ctx, queueID, solos[i], path, nil); err != nil {
			t.Fatalf("solo join %d: %v", i, err)
		}
		rec = mustReconcileForming(t, st, ctx, queueID)
		if i < len(soloPaths)-1 && rec.Fired {
			t.Fatalf("expected waiting after solo %d, got fired", i)
		}
	}
	if !rec.Fired || rec.SessionID == nil {
		t.Fatalf("expected match, got %+v", rec)
	}

	participants, err := st.ListSessionSeatAssignments(ctx, *rec.SessionID)
	if err != nil {
		t.Fatalf("ListSessionSeatAssignments: %v", err)
	}
	if len(participants) != 6 {
		t.Fatalf("expected 6 participants, got %d", len(participants))
	}

	byUser := map[uuid.UUID]string{}
	for _, p := range participants {
		byUser[p.UserID] = p.SeatKey
	}
	if !strings.HasPrefix(byUser[pat.ID], "ClueGiver-") {
		t.Fatalf("pat seat = %q, want ClueGiver-*", byUser[pat.ID])
	}
	if byUser[bro.ID] != "Guesser-1" {
		t.Fatalf("bro seat = %q, want Guesser-1", byUser[bro.ID])
	}
}
