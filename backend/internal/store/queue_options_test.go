package store

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/gameclient"
	"github.com/scruffyprodigy/joinquest/internal/prequeue"
)

// setupHelpersMode registers a two-seat mode that declares one pre-queue group
// taking exactly two picks — RPSLR's duel-helpers shape.
func setupHelpersMode(t *testing.T, st *Store, cleaner *TestCleaner) (*Game, *GameMode, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	slug := "helpers-" + uuid.NewString()
	manifest := &gameclient.Manifest{
		Modes: []gameclient.ModeManifest{{
			Key:          "duel-helpers",
			DisplayName:  "Helpers",
			SeatTemplate: json.RawMessage(`{"count":2}`),
			PreQueue: json.RawMessage(`{"groups":[
				{"key":"helpers","kind":"Loadout","label":"Choose your two helpers","min":2,"max":2}
			]}`),
		}},
		Status:     gameclient.StatusResponse{Game: "Helpers", Version: "1.0.0"},
		ETag:       `"helpers"`,
		RawJSON:    []byte(`{"modes":[{"key":"duel-helpers"}]}`),
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
	return result.Game, &modes[0], queues[0].ID
}

func helperPicks() []prequeue.Selection {
	return []prequeue.Selection{{GroupKey: "helpers", OptionIDs: []string{"ferrus", "tempered"}}}
}

func TestApplyManifestStoresPreQueueDeclaration(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)

	_, mode, _ := setupHelpersMode(t, st, cleaner)

	decl, err := prequeue.Parse(mode.PreQueue)
	if err != nil {
		t.Fatalf("prequeue.Parse: %v", err)
	}
	if decl == nil || len(decl.Groups) != 1 {
		t.Fatalf("stored declaration = %+v, want one group", decl)
	}
	if decl.Groups[0].Key != "helpers" || decl.Groups[0].Min != 2 || decl.Groups[0].Max != 2 {
		t.Fatalf("stored group = %+v, want helpers 2..2", decl.Groups[0])
	}
}

func TestJoinModeQueueWithOptionsStoresSelections(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	_, _, queueID := setupHelpersMode(t, st, cleaner)
	user, err := st.CreateUser(ctx, CreateUserParams{Email: "helpers-join@example.com"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	cleaner.TrackUser(user.ID)

	if _, err := st.JoinModeQueueWithOptions(ctx, queueID, user.ID, "", helperPicks(), nil); err != nil {
		t.Fatalf("JoinModeQueueWithOptions: %v", err)
	}

	entry, err := st.GetWaitingModeQueueEntry(ctx, queueID, user.ID)
	if err != nil {
		t.Fatalf("GetWaitingModeQueueEntry: %v", err)
	}
	if len(entry.QueueOptions) != 1 || entry.QueueOptions[0].GroupKey != "helpers" {
		t.Fatalf("stored options = %+v, want the helpers group", entry.QueueOptions)
	}
	if got := entry.QueueOptions[0].OptionIDs; len(got) != 2 || got[0] != "ferrus" || got[1] != "tempered" {
		t.Fatalf("stored ids = %v, want [ferrus tempered]", got)
	}
}

func TestJoinModeQueueWithoutOptionsStoresNone(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	_, _, queueID := setupHelpersMode(t, st, cleaner)
	user, err := st.CreateUser(ctx, CreateUserParams{Email: "helpers-none@example.com"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	cleaner.TrackUser(user.ID)

	if _, err := st.JoinModeQueue(ctx, queueID, user.ID, "", nil); err != nil {
		t.Fatalf("JoinModeQueue: %v", err)
	}

	entry, err := st.GetWaitingModeQueueEntry(ctx, queueID, user.ID)
	if err != nil {
		t.Fatalf("GetWaitingModeQueueEntry: %v", err)
	}
	if len(entry.QueueOptions) != 0 {
		t.Fatalf("stored options = %+v, want none", entry.QueueOptions)
	}
}

func TestFormedMatchCarriesQueueOptionsToSessionParticipants(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	_, _, queueID := setupHelpersMode(t, st, cleaner)

	first, err := st.CreateUser(ctx, CreateUserParams{Email: "helpers-p1@example.com"})
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	cleaner.TrackUser(first.ID)
	second, err := st.CreateUser(ctx, CreateUserParams{Email: "helpers-p2@example.com"})
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	cleaner.TrackUser(second.ID)

	if _, err := st.JoinModeQueueWithOptions(ctx, queueID, first.ID, "", helperPicks(), nil); err != nil {
		t.Fatalf("join first: %v", err)
	}
	secondPicks := []prequeue.Selection{{GroupKey: "helpers", OptionIDs: []string{"chimera", "grudge"}}}
	result, err := st.JoinModeQueueWithOptions(ctx, queueID, second.ID, "", secondPicks, nil)
	if err != nil {
		t.Fatalf("join second: %v", err)
	}
	mustReconcileForming(t, st, ctx, queueID)
	if result == nil {
		t.Fatal("join second returned no result")
	}

	session, err := st.GetMatchedSessionForUserAndModeQueue(ctx, queueID, second.ID)
	if err != nil {
		t.Fatalf("GetMatchedSessionForUserAndModeQueue: %v", err)
	}
	participants, err := st.ListSessionSeatAssignments(ctx, session.ID)
	if err != nil {
		t.Fatalf("ListSessionSeatAssignments: %v", err)
	}
	if len(participants) != 2 {
		t.Fatalf("participants = %d, want 2", len(participants))
	}

	byUser := map[uuid.UUID][]prequeue.Selection{}
	for _, p := range participants {
		byUser[p.UserID] = p.QueueOptions
	}
	firstPicked := byUser[first.ID]
	if len(firstPicked) != 1 || len(firstPicked[0].OptionIDs) != 2 || firstPicked[0].OptionIDs[0] != "ferrus" {
		t.Fatalf("first player's options = %+v, want ferrus+tempered", firstPicked)
	}
	secondPicked := byUser[second.ID]
	if len(secondPicked) != 1 || secondPicked[0].OptionIDs[0] != "chimera" {
		t.Fatalf("second player's options = %+v, want chimera+grudge", secondPicked)
	}
}

func TestSitAtTableWithOptionsCarriesSelectionsToTheSession(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	game, mode, _ := setupHelpersMode(t, st, cleaner)

	host, err := st.CreateUser(ctx, CreateUserParams{Email: "helpers-host@example.com"})
	if err != nil {
		t.Fatalf("create host: %v", err)
	}
	cleaner.TrackUser(host.ID)
	guest, err := st.CreateUser(ctx, CreateUserParams{Email: "helpers-guest@example.com"})
	if err != nil {
		t.Fatalf("create guest: %v", err)
	}
	cleaner.TrackUser(guest.ID)

	room, err := st.CreateRoom(ctx, host.ID)
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	if err := st.addRoomMemberDirect(ctx, room.ID, guest.ID); err != nil {
		t.Fatalf("add guest: %v", err)
	}
	table, err := st.CreateTable(ctx, room.ID, game.ID, mode.ID, host.ID)
	if err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	seats, err := st.ListGameModeSeats(ctx, mode.ID)
	if err != nil {
		t.Fatalf("ListGameModeSeats: %v", err)
	}

	if _, err := st.SitAtTableWithOptions(ctx, table.ID, host.ID, seats[0].SeatKey, helperPicks()); err != nil {
		t.Fatalf("SitAtTableWithOptions host: %v", err)
	}
	guestPicks := []prequeue.Selection{{GroupKey: "helpers", OptionIDs: []string{"chimera", "grudge"}}}
	if _, err := st.SitAtTableWithOptions(ctx, table.ID, guest.ID, seats[1].SeatKey, guestPicks); err != nil {
		t.Fatalf("SitAtTableWithOptions guest: %v", err)
	}

	started, err := st.StartTable(ctx, table.ID, host.ID)
	if err != nil {
		t.Fatalf("StartTable: %v", err)
	}
	participants, err := st.ListSessionSeatAssignments(ctx, started.SessionID)
	if err != nil {
		t.Fatalf("ListSessionSeatAssignments: %v", err)
	}

	byUser := map[uuid.UUID][]prequeue.Selection{}
	for _, p := range participants {
		byUser[p.UserID] = p.QueueOptions
	}
	if got := byUser[host.ID]; len(got) != 1 || got[0].OptionIDs[0] != "ferrus" {
		t.Fatalf("host options = %+v, want ferrus+tempered", got)
	}
	if got := byUser[guest.ID]; len(got) != 1 || got[0].OptionIDs[0] != "chimera" {
		t.Fatalf("guest options = %+v, want chimera+grudge", got)
	}
}
