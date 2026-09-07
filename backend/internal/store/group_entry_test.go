package store

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// The group screen's entry point is CreatePrivateTable: the player asks to play with
// friends and never sees room creation. One room per player globally still has to
// hold, so a second group must land in the room the player already has (JQ-132, AC #5).
func TestCreatePrivateTableReusesThePlayersExistingRoom(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	player, err := st.CreateUser(ctx, CreateUserParams{Email: "group-reuse-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("create player: %v", err)
	}
	cleaner.TrackUser(player.ID)

	game, mode, _ := setupWordHuntMode(t, st, cleaner)

	first, err := st.CreatePrivateTable(ctx, player.ID, game.ID, mode.ID)
	if err != nil {
		t.Fatalf("first CreatePrivateTable: %v", err)
	}

	second, err := st.CreatePrivateTable(ctx, player.ID, game.ID, mode.ID)
	if err != nil {
		t.Fatalf("second CreatePrivateTable: %v", err)
	}

	if first.RoomID != second.RoomID {
		t.Fatalf("expected both tables in one room, got %s and %s", first.RoomID, second.RoomID)
	}

	room, err := st.GetUserRoom(ctx, player.ID)
	if err != nil {
		t.Fatalf("GetUserRoom: %v", err)
	}
	if room.ID != first.RoomID {
		t.Fatalf("player's room is %s, tables are in %s", room.ID, first.RoomID)
	}
}

// Queue and table stay mutually exclusive for a player who arrives through the group
// screen (JQ-132, AC #6). The group page claims seats with sitAtTable directly, so the
// check cannot live only in the catalog's create-table button.
func TestAssertCanTakeTableSeatRefusesAPlayerWaitingInAQueue(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	player, err := st.CreateUser(ctx, CreateUserParams{Email: "group-queued-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("create player: %v", err)
	}
	cleaner.TrackUser(player.ID)

	_, _, queueID := setupWordHuntMode(t, st, cleaner)

	if err := st.AssertCanTakeTableSeat(ctx, player.ID); err != nil {
		t.Fatalf("a player in no queue may take a seat, got %v", err)
	}

	if _, err := st.JoinModeQueue(ctx, queueID, player.ID, "Guesser", nil); err != nil {
		t.Fatalf("JoinModeQueue: %v", err)
	}

	err = st.AssertCanTakeTableSeat(ctx, player.ID)
	if err == nil {
		t.Fatal("expected a waiting queue player to be refused a table seat")
	}
	if !errors.Is(err, ErrAlreadyQueued) {
		t.Fatalf("expected ErrAlreadyQueued, got %v", err)
	}
}
