package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// The group screen's entry point is CreatePrivateTable: the player asks to play with
// friends and never sees room creation. Every click opens a NEW room (JQ-253).
//
// This reverses TestCreatePrivateTableReusesThePlayersExistingRoom, which asserted the
// opposite on the strength of JQ-132 AC #5 — "one room per player globally". That
// invariant is still true and still tested below; what was wrong was reading it as a
// reason to hand the player back a room they were already in. Reuse plus rooms that
// nothing ever tore down meant asking to play with friends could drop you into last
// night's room, with an invite code already shared with people who are not here.
func TestCreatePrivateTableAlwaysOpensAFreshRoom(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	player, err := st.CreateUser(ctx, CreateUserParams{Email: "group-fresh-" + uuid.NewString() + "@example.com"})
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

	if first.RoomID == second.RoomID {
		t.Fatalf("both clicks landed in room %s; the second should have opened a new one",
			first.RoomID)
	}

	// One room per player globally still holds — the point is which room it is, not how
	// many. createRoomTx leaves the old room before inserting the new membership, so the
	// unique index on room_members.user_id is never contested.
	room, err := st.GetUserRoom(ctx, player.ID)
	if err != nil {
		t.Fatalf("GetUserRoom: %v", err)
	}
	if room.ID != second.RoomID {
		t.Fatalf("player's room is %s, want the newest one %s", room.ID, second.RoomID)
	}

	// The abandoned room emptied when they left it, so it closed behind them rather
	// than lingering to be re-entered.
	if got := roomStatus(t, st, ctx, first.RoomID); got != RoomStatusClosed {
		t.Fatalf("abandoned room status = %q, want %q", got, RoomStatusClosed)
	}
}

// Only room CREATION changed. An invite code still finds exactly the room it names —
// otherwise "always a fresh room" would mean nobody could ever join anybody.
func TestJoinRoomStillEntersTheRoomTheCodeNames(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	host, err := st.CreateUser(ctx, CreateUserParams{Email: "group-host-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("create host: %v", err)
	}
	cleaner.TrackUser(host.ID)
	friend, err := st.CreateUser(ctx, CreateUserParams{Email: "group-friend-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("create friend: %v", err)
	}
	cleaner.TrackUser(friend.ID)

	game, mode, _ := setupWordHuntMode(t, st, cleaner)

	table, err := st.CreatePrivateTable(ctx, host.ID, game.ID, mode.ID)
	if err != nil {
		t.Fatalf("CreatePrivateTable: %v", err)
	}
	hostRoom, err := st.GetRoomByID(ctx, table.RoomID)
	if err != nil {
		t.Fatalf("GetRoomByID: %v", err)
	}

	joined, err := st.JoinRoom(ctx, friend.ID, hostRoom.InviteCode)
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}
	if joined.ID != hostRoom.ID {
		t.Fatalf("friend joined %s, want the host's room %s", joined.ID, hostRoom.ID)
	}

	// And a friend who already had a room of their own still lands in the host's,
	// not a fresh one.
	other, err := st.CreateUser(ctx, CreateUserParams{Email: "group-other-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("create other: %v", err)
	}
	cleaner.TrackUser(other.ID)
	if _, err := st.CreatePrivateTable(ctx, other.ID, game.ID, mode.ID); err != nil {
		t.Fatalf("other CreatePrivateTable: %v", err)
	}
	rejoined, err := st.JoinRoom(ctx, other.ID, hostRoom.InviteCode)
	if err != nil {
		t.Fatalf("JoinRoom for a player who had a room: %v", err)
	}
	if rejoined.ID != hostRoom.ID {
		t.Fatalf("joined %s, want the host's room %s", rejoined.ID, hostRoom.ID)
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
