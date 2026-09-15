package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// Every test here is the arrival half of the rejoin rule (JQ-306): a player who creates
// or joins is seated exactly where a returning player would be, and left alone exactly
// where a returning player would be left alone. The trigger is the same helper, so the
// pairs below are deliberate — each "seats them" case has a "leaves them" twin that
// differs only in whether the mode asks for a pick.

func newArrivalUser(t *testing.T, st *Store, ctx context.Context, cleaner *TestCleaner, tag string) *User {
	t.Helper()
	user, err := st.CreateUser(ctx, CreateUserParams{Email: "arrival-" + tag + "-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser %s: %v", tag, err)
	}
	cleaner.TrackUser(user.ID)
	return user
}

func seatedUserIDs(t *testing.T, st *Store, ctx context.Context, tableID uuid.UUID) []uuid.UUID {
	t.Helper()
	seats, err := st.ListTableSeats(ctx, tableID)
	if err != nil {
		t.Fatalf("ListTableSeats: %v", err)
	}
	ids := make([]uuid.UUID, 0, len(seats))
	for _, seat := range seats {
		ids = append(ids, seat.UserID)
	}
	return ids
}

func assertSeatedAt(t *testing.T, st *Store, ctx context.Context, tableID uuid.UUID, userID uuid.UUID) {
	t.Helper()
	for _, id := range seatedUserIDs(t, st, ctx, tableID) {
		if id == userID {
			return
		}
	}
	t.Fatalf("user %s is not seated at table %s; seats held by %v", userID, tableID,
		seatedUserIDs(t, st, ctx, tableID))
}

func assertNotSeatedAt(t *testing.T, st *Store, ctx context.Context, tableID uuid.UUID, userID uuid.UUID) {
	t.Helper()
	for _, id := range seatedUserIDs(t, st, ctx, tableID) {
		if id == userID {
			t.Fatalf("user %s was seated at table %s automatically; they should have been left to choose",
				userID, tableID)
		}
	}
}

func TestCreatePrivateTableSeatsTheCreatorWhenNothingToChoose(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	player := newArrivalUser(t, st, ctx, cleaner, "creator")
	game, mode := setupDuelMode(t, st, cleaner)

	table, err := st.CreatePrivateTable(ctx, player.ID, game.ID, mode.ID)
	if err != nil {
		t.Fatalf("CreatePrivateTable: %v", err)
	}

	assertSeatedAt(t, st, ctx, table.ID, player.ID)
	if got := len(seatedUserIDs(t, st, ctx, table.ID)); got != 1 {
		t.Fatalf("table has %d seats filled, want only the creator's", got)
	}
}

// The twin. Two seat classes is a role to pick, and picking a role for somebody is the
// thing this feature must never start doing.
func TestCreatePrivateTableLeavesTheCreatorUnseatedWhenThereIsAChoice(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	player := newArrivalUser(t, st, ctx, cleaner, "chooser")
	game, mode := setupRolesMode(t, st, cleaner)

	table, err := st.CreatePrivateTable(ctx, player.ID, game.ID, mode.ID)
	if err != nil {
		t.Fatalf("CreatePrivateTable: %v", err)
	}

	assertNotSeatedAt(t, st, ctx, table.ID, player.ID)
}

func TestJoinRoomSeatsTheArrivalWhenNothingToChoose(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	host := newArrivalUser(t, st, ctx, cleaner, "host")
	friend := newArrivalUser(t, st, ctx, cleaner, "friend")
	game, mode := setupDuelMode(t, st, cleaner)

	table, err := st.CreatePrivateTable(ctx, host.ID, game.ID, mode.ID)
	if err != nil {
		t.Fatalf("CreatePrivateTable: %v", err)
	}
	room, err := st.GetRoomByID(ctx, table.RoomID)
	if err != nil {
		t.Fatalf("GetRoomByID: %v", err)
	}

	if _, err := st.JoinRoom(ctx, friend.ID, room.InviteCode); err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}

	assertSeatedAt(t, st, ctx, table.ID, friend.ID)
	// The host keeps the seat they already had. An arrival takes an open seat; it never
	// shuffles the people already sitting down.
	assertSeatedAt(t, st, ctx, table.ID, host.ID)
}

func TestJoinRoomLeavesTheArrivalUnseatedWhenThereIsAChoice(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	host := newArrivalUser(t, st, ctx, cleaner, "host")
	friend := newArrivalUser(t, st, ctx, cleaner, "friend")
	game, mode := setupRolesMode(t, st, cleaner)

	table, err := st.CreatePrivateTable(ctx, host.ID, game.ID, mode.ID)
	if err != nil {
		t.Fatalf("CreatePrivateTable: %v", err)
	}
	room, err := st.GetRoomByID(ctx, table.RoomID)
	if err != nil {
		t.Fatalf("GetRoomByID: %v", err)
	}

	if _, err := st.JoinRoom(ctx, friend.ID, room.InviteCode); err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}

	assertNotSeatedAt(t, st, ctx, table.ID, friend.ID)
}

// Which table to sit at is a decision too, and it is one the room makes rather than the
// mode. A room with several forming tables hands the arrival the same screen it always
// did.
func TestJoinRoomDoesNotSeatWhenTheRoomHasSeveralTables(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	host := newArrivalUser(t, st, ctx, cleaner, "host")
	friend := newArrivalUser(t, st, ctx, cleaner, "friend")
	game, mode := setupDuelMode(t, st, cleaner)

	first, err := st.CreatePrivateTable(ctx, host.ID, game.ID, mode.ID)
	if err != nil {
		t.Fatalf("CreatePrivateTable: %v", err)
	}
	second, err := st.CreateTable(ctx, first.RoomID, game.ID, mode.ID, host.ID)
	if err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	room, err := st.GetRoomByID(ctx, first.RoomID)
	if err != nil {
		t.Fatalf("GetRoomByID: %v", err)
	}

	if _, err := st.JoinRoom(ctx, friend.ID, room.InviteCode); err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}

	assertNotSeatedAt(t, st, ctx, first.ID, friend.ID)
	assertNotSeatedAt(t, st, ctx, second.ID, friend.ID)
}

// A full table is not a failure to join. The arrival is in the room, watching, and the
// join itself must not fail because the convenience had nowhere to put them.
func TestJoinRoomSucceedsUnseatedWhenTheTableIsFull(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	host := newArrivalUser(t, st, ctx, cleaner, "host")
	friend := newArrivalUser(t, st, ctx, cleaner, "friend")
	latecomer := newArrivalUser(t, st, ctx, cleaner, "latecomer")
	game, mode := setupDuelMode(t, st, cleaner)

	table, err := st.CreatePrivateTable(ctx, host.ID, game.ID, mode.ID)
	if err != nil {
		t.Fatalf("CreatePrivateTable: %v", err)
	}
	room, err := st.GetRoomByID(ctx, table.RoomID)
	if err != nil {
		t.Fatalf("GetRoomByID: %v", err)
	}
	if _, err := st.JoinRoom(ctx, friend.ID, room.InviteCode); err != nil {
		t.Fatalf("JoinRoom friend: %v", err)
	}

	if _, err := st.JoinRoom(ctx, latecomer.ID, room.InviteCode); err != nil {
		t.Fatalf("JoinRoom latecomer: %v", err)
	}

	member, err := st.IsRoomMember(ctx, table.RoomID, latecomer.ID)
	if err != nil {
		t.Fatalf("IsRoomMember: %v", err)
	}
	if !member {
		t.Fatal("latecomer is not in the room; a full table must not fail the join")
	}
	assertNotSeatedAt(t, st, ctx, table.ID, latecomer.ID)
}

// Standing up is a decision, and the client calls joinRoom every time it opens a room by
// code — including on a refresh. Re-seating here would take the choice back.
func TestJoinRoomDoesNotReseatAPlayerWhoLeftTheirSeat(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	host := newArrivalUser(t, st, ctx, cleaner, "host")
	friend := newArrivalUser(t, st, ctx, cleaner, "friend")
	game, mode := setupDuelMode(t, st, cleaner)

	table, err := st.CreatePrivateTable(ctx, host.ID, game.ID, mode.ID)
	if err != nil {
		t.Fatalf("CreatePrivateTable: %v", err)
	}
	room, err := st.GetRoomByID(ctx, table.RoomID)
	if err != nil {
		t.Fatalf("GetRoomByID: %v", err)
	}
	if _, err := st.JoinRoom(ctx, friend.ID, room.InviteCode); err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}
	if _, err := st.LeaveTable(ctx, table.ID, friend.ID); err != nil {
		t.Fatalf("LeaveTable: %v", err)
	}

	if _, err := st.JoinRoom(ctx, friend.ID, room.InviteCode); err != nil {
		t.Fatalf("second JoinRoom: %v", err)
	}

	assertNotSeatedAt(t, st, ctx, table.ID, friend.ID)
}
