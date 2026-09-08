package store

import (
	"context"
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
// guarantee has to hold on that mutation and not only in the catalog's button.
//
// Exclusion here is resolved rather than refused: sitting down cancels the waiting
// queue entry (leaveUserWaitingQueuesTx), so a player who was looking for a group and
// then takes a seat with friends ends up in exactly one of the two. Asserting on the
// end state rather than on an error keeps this honest if the mechanism moves.
func TestSittingAtATableCancelsAWaitingQueueEntry(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	player, err := st.CreateUser(ctx, CreateUserParams{Email: "group-queued-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("create player: %v", err)
	}
	cleaner.TrackUser(player.ID)

	game, mode, queueID := setupWordHuntMode(t, st, cleaner)

	table, err := st.CreatePrivateTable(ctx, player.ID, game.ID, mode.ID)
	if err != nil {
		t.Fatalf("CreatePrivateTable: %v", err)
	}

	if _, err := st.JoinModeQueue(ctx, queueID, player.ID, "Guesser", nil); err != nil {
		t.Fatalf("JoinModeQueue: %v", err)
	}
	if waiting := countWaitingQueueRows(t, st, player.ID); waiting != 1 {
		t.Fatalf("expected the player to be waiting in one queue, got %d", waiting)
	}

	seats, err := st.ListGameModeSeats(ctx, mode.ID)
	if err != nil {
		t.Fatalf("ListGameModeSeats: %v", err)
	}
	if len(seats) == 0 {
		t.Fatal("mode has no seats")
	}
	if _, err := st.SitAtTable(ctx, table.ID, player.ID, seats[0].SeatKey); err != nil {
		t.Fatalf("SitAtTable: %v", err)
	}

	if waiting := countWaitingQueueRows(t, st, player.ID); waiting != 0 {
		t.Fatalf("taking a seat should leave no waiting queue entry, got %d", waiting)
	}

	seated, err := st.GetUserTableSeat(ctx, player.ID)
	if err != nil {
		t.Fatalf("GetUserTableSeat: %v", err)
	}
	if seated == nil || seated.TableID != table.ID {
		t.Fatalf("expected the player seated at %s, got %+v", table.ID, seated)
	}
}

func countWaitingQueueRows(t *testing.T, st *Store, userID uuid.UUID) int {
	t.Helper()
	var count int
	if err := st.db.QueryRowContext(context.Background(), `
		SELECT COUNT(*) FROM game_queues WHERE user_id = $1 AND status = 'waiting'
	`, userID).Scan(&count); err != nil {
		t.Fatalf("count waiting queue rows: %v", err)
	}
	return count
}
