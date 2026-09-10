package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// roomAndDisconnect puts the user in a fresh room and drops their last socket,
// returning the room and the stamp the grace window is running from.
func roomAndDisconnect(t *testing.T, st *Store, ctx context.Context, userID uuid.UUID) (*Room, time.Time) {
	t.Helper()
	room, err := st.CreateRoom(ctx, userID)
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	if _, err := st.PresenceConnected(ctx, userID); err != nil {
		t.Fatalf("connect: %v", err)
	}
	dropped, err := st.PresenceDisconnected(ctx, userID)
	if err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	if dropped.DisconnectedAt == nil {
		t.Fatal("disconnect did not stamp")
	}
	return room, *dropped.DisconnectedAt
}

func roomMemberCount(t *testing.T, st *Store, ctx context.Context, roomID uuid.UUID) int {
	t.Helper()
	var n int
	if err := st.db.QueryRowContext(ctx,
		`SELECT count(*) FROM room_members WHERE room_id = $1`, roomID,
	).Scan(&n); err != nil {
		t.Fatalf("count room members: %v", err)
	}
	return n
}

func roomStatus(t *testing.T, st *Store, ctx context.Context, roomID uuid.UUID) string {
	t.Helper()
	var status string
	if err := st.db.QueryRowContext(ctx,
		`SELECT status FROM rooms WHERE id = $1`, roomID,
	).Scan(&status); err != nil {
		t.Fatalf("read room status: %v", err)
	}
	return status
}

// backdateDisconnect ages a player's disconnect stamp so the sweep's predicate reaches
// it. Faster and more honest than sleeping through a real window, and it is the stamp —
// not elapsed wall time — that both the predicate and the eviction guard read.
func backdateDisconnect(t *testing.T, st *Store, ctx context.Context, userID uuid.UUID, by time.Duration) {
	t.Helper()
	if _, err := st.db.ExecContext(ctx,
		`UPDATE user_presence SET disconnected_at = NOW() - ($2 * INTERVAL '1 second') WHERE user_id = $1`,
		userID, by.Seconds(),
	); err != nil {
		t.Fatalf("backdate disconnect: %v", err)
	}
}

// The room window is deliberately NOT the queue's. Pinning the number here is pinning
// the reasoning in DefaultRoomDisconnectGrace: 30s is where the client's own reconnect
// budget runs out (10 retries at min(500ms*n, 5s) ≈ 27.5s), so we hold a place for
// exactly as long as the player's client is still asking for it. A change to either
// number without the other is the bug this catches.
func TestDefaultRoomDisconnectGraceIsThirtySeconds(t *testing.T) {
	if DefaultRoomDisconnectGrace != 30*time.Second {
		t.Fatalf("room grace window: got %s, want 30s", DefaultRoomDisconnectGrace)
	}
	if DefaultRoomDisconnectGrace >= DefaultQueueDisconnectGrace {
		t.Fatalf("room grace %s should be shorter than the queue's %s: a room has no waiting"+
			" players to compensate, so showing the truth beats holding the slot",
			DefaultRoomDisconnectGrace, DefaultQueueDisconnectGrace)
	}
}

// A backgrounded phone is not someone who left. The reconnect clears the stamp, and the
// timer that was armed against the old one must then find nothing to act on — that
// guard, not a second lookup, is what keeps the player in the room.
func TestReconnectInsideTheWindowKeepsRoomMembership(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	userID := newPresenceUser(t, st, cleaner, ctx)

	room, stamp := roomAndDisconnect(t, st, ctx, userID)

	if _, err := st.PresenceConnected(ctx, userID); err != nil {
		t.Fatalf("reconnect: %v", err)
	}

	// The timer armed before the reconnect still fires; it carries the old stamp.
	result, err := st.EvictDisconnectedRoomMember(ctx, userID, stamp)
	if err != nil {
		t.Fatalf("EvictDisconnectedRoomMember: %v", err)
	}
	if result.Acted {
		t.Fatal("a reconnected player was evicted from their room")
	}
	if got := roomMemberCount(t, st, ctx, room.ID); got != 1 {
		t.Fatalf("room members = %d, want the reconnected player still in it", got)
	}
	if got := roomStatus(t, st, ctx, room.ID); got != RoomStatusOpen {
		t.Fatalf("room status = %q, want %q", got, RoomStatusOpen)
	}
}

func TestDisconnectPastTheWindowRemovesTheRoomMember(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	host := newPresenceUser(t, st, cleaner, ctx)
	friend := newPresenceUser(t, st, cleaner, ctx)

	room, stamp := roomAndDisconnect(t, st, ctx, host)
	if err := st.addRoomMemberDirect(ctx, room.ID, friend); err != nil {
		t.Fatalf("add friend: %v", err)
	}

	result, err := st.EvictDisconnectedRoomMember(ctx, host, stamp)
	if err != nil {
		t.Fatalf("EvictDisconnectedRoomMember: %v", err)
	}
	if !result.Acted {
		t.Fatal("expected the disconnected player to be removed")
	}
	if result.RoomID != room.ID {
		t.Fatalf("evicted from %s, want %s", result.RoomID, room.ID)
	}
	// The friend is still here, so the room is not gone — the caller needs to tell a
	// roster change from a room ending, because only one of those is a departure the
	// remaining members should see.
	if result.RoomClosed {
		t.Fatal("room closed while a member was still in it")
	}
	if got := roomMemberCount(t, st, ctx, room.ID); got != 1 {
		t.Fatalf("room members = %d, want just the friend", got)
	}
	if got := roomStatus(t, st, ctx, room.ID); got != RoomStatusOpen {
		t.Fatalf("room status = %q, want %q", got, RoomStatusOpen)
	}
}

// The other half of the ticket: rooms clean themselves up. A deliberate leave is
// immediate — no window applies to somebody who said they were going.
func TestLastMemberLeavingClosesTheRoom(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	userID := newPresenceUser(t, st, cleaner, ctx)

	room, err := st.CreateRoom(ctx, userID)
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	left, err := st.LeaveRoom(ctx, userID)
	if err != nil {
		t.Fatalf("LeaveRoom: %v", err)
	}
	if !left {
		t.Fatal("LeaveRoom reported nothing to leave")
	}
	if got := roomStatus(t, st, ctx, room.ID); got != RoomStatusClosed {
		t.Fatalf("room status = %q, want %q", got, RoomStatusClosed)
	}
	if _, err := st.GetUserRoom(ctx, userID); err == nil {
		t.Fatal("a closed room is still the player's room")
	}
}

// The last member losing presence empties the room the same way, and the room goes with
// them rather than lingering for nobody.
func TestPresenceLossOfTheLastMemberClosesTheRoom(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	userID := newPresenceUser(t, st, cleaner, ctx)

	room, stamp := roomAndDisconnect(t, st, ctx, userID)

	result, err := st.EvictDisconnectedRoomMember(ctx, userID, stamp)
	if err != nil {
		t.Fatalf("EvictDisconnectedRoomMember: %v", err)
	}
	if !result.Acted || !result.RoomClosed {
		t.Fatalf("result = %+v, want the last member removed and the room closed", result)
	}
	if got := roomStatus(t, st, ctx, room.ID); got != RoomStatusClosed {
		t.Fatalf("room status = %q, want %q", got, RoomStatusClosed)
	}
}

// The case that makes cleanup dangerous. Players in a game have their sockets on the
// game, not the lobby, so presence reads every one of them as gone — and an empty
// roster then looks exactly like an abandoned room. Closing it there would tear down
// the room a live match is still pointing at.
func TestRoomWithALiveSessionSurvivesAnEmptyMembership(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	game, mode := setupDuelMode(t, st, cleaner)
	host := newPresenceUser(t, st, cleaner, ctx)
	guest := newPresenceUser(t, st, cleaner, ctx)

	room, err := st.CreateRoom(ctx, host)
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	if err := st.addRoomMemberDirect(ctx, room.ID, guest); err != nil {
		t.Fatalf("add guest: %v", err)
	}
	table, err := st.CreateTable(ctx, room.ID, game.ID, mode.ID, host)
	if err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	modeSeats, err := st.ListGameModeSeats(ctx, mode.ID)
	if err != nil {
		t.Fatalf("ListGameModeSeats: %v", err)
	}
	if len(modeSeats) < 2 {
		t.Fatalf("duel mode has %d seats, want 2", len(modeSeats))
	}
	if _, err := st.SitAtTable(ctx, table.ID, host, modeSeats[0].SeatKey); err != nil {
		t.Fatalf("host sit: %v", err)
	}
	if _, err := st.SitAtTable(ctx, table.ID, guest, modeSeats[1].SeatKey); err != nil {
		t.Fatalf("guest sit: %v", err)
	}
	started, err := st.StartTable(ctx, table.ID, host)
	if err != nil {
		t.Fatalf("StartTable: %v", err)
	}

	// Both players lose the lobby socket, which is what going into a game looks like
	// from here.
	for _, userID := range []uuid.UUID{host, guest} {
		if _, err := st.PresenceConnected(ctx, userID); err != nil {
			t.Fatalf("connect %s: %v", userID, err)
		}
		dropped, err := st.PresenceDisconnected(ctx, userID)
		if err != nil {
			t.Fatalf("disconnect %s: %v", userID, err)
		}
		if _, err := st.EvictDisconnectedRoomMember(ctx, userID, *dropped.DisconnectedAt); err != nil {
			t.Fatalf("evict %s: %v", userID, err)
		}
	}

	if got := roomMemberCount(t, st, ctx, room.ID); got != 0 {
		t.Fatalf("room members = %d, want 0 — both players are in the game", got)
	}
	if got := roomStatus(t, st, ctx, room.ID); got != RoomStatusOpen {
		t.Fatalf("room status = %q, want %q: a live session is not an empty room",
			got, RoomStatusOpen)
	}

	// And once the match ends, the same empty room is genuinely finished. Nobody is
	// left to trigger a removal — the last member left long ago — so SweepEmptyRooms is
	// the only thing that ever reconsiders it.
	//
	// Note what completion does on the way past: resetRoomTableAfterSessionTx returns
	// the table to forming and re-seats both participants, neither of whom is a room
	// member any more. Those seats must not read as live play, or this room would stay
	// open forever on behalf of two people who are gone and cannot be evicted again.
	if err := st.CompleteSession(ctx, started.SessionID, time.Now()); err != nil {
		t.Fatalf("CompleteSession: %v", err)
	}
	if _, err := st.SweepEmptyRooms(ctx); err != nil {
		t.Fatalf("SweepEmptyRooms: %v", err)
	}
	if got := roomStatus(t, st, ctx, room.ID); got != RoomStatusClosed {
		t.Fatalf("room status after the match ended = %q, want %q", got, RoomStatusClosed)
	}
}

// The guard the sweep above must not overreach: while the session is still active, the
// same empty room is off limits. This is the sweep's version of the distinction
// roomHasLivePlayClause draws, and the one that would silently destroy in-flight games
// if the predicate ever drifted.
func TestSweepEmptyRoomsLeavesARoomWhoseGameIsStillRunning(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	game, mode := setupDuelMode(t, st, cleaner)
	host := newPresenceUser(t, st, cleaner, ctx)
	guest := newPresenceUser(t, st, cleaner, ctx)

	room, err := st.CreateRoom(ctx, host)
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	if err := st.addRoomMemberDirect(ctx, room.ID, guest); err != nil {
		t.Fatalf("add guest: %v", err)
	}
	table, err := st.CreateTable(ctx, room.ID, game.ID, mode.ID, host)
	if err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	modeSeats, err := st.ListGameModeSeats(ctx, mode.ID)
	if err != nil {
		t.Fatalf("ListGameModeSeats: %v", err)
	}
	if _, err := st.SitAtTable(ctx, table.ID, host, modeSeats[0].SeatKey); err != nil {
		t.Fatalf("host sit: %v", err)
	}
	if _, err := st.SitAtTable(ctx, table.ID, guest, modeSeats[1].SeatKey); err != nil {
		t.Fatalf("guest sit: %v", err)
	}
	if _, err := st.StartTable(ctx, table.ID, host); err != nil {
		t.Fatalf("StartTable: %v", err)
	}
	// Empty the roster without ending the game.
	if _, err := st.db.ExecContext(ctx, `DELETE FROM room_members WHERE room_id = $1`, room.ID); err != nil {
		t.Fatalf("empty the roster: %v", err)
	}

	if _, err := st.SweepEmptyRooms(ctx); err != nil {
		t.Fatalf("SweepEmptyRooms: %v", err)
	}
	if got := roomStatus(t, st, ctx, room.ID); got != RoomStatusOpen {
		t.Fatalf("room status = %q, want %q: the sweep closed a room with a live match in it",
			got, RoomStatusOpen)
	}
}

// The crash backstop. When the pod holding the timer dies the window still elapses, and
// the sweep is what notices — the same relationship queuesweep's disconnected sweep has
// to the queue timer.
func TestRoomSweepRemovesMembersWhoseTimerDiedWithTheirPod(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	userID := newPresenceUser(t, st, cleaner, ctx)

	room, _ := roomAndDisconnect(t, st, ctx, userID)
	backdateDisconnect(t, st, ctx, userID, 2*DefaultRoomDisconnectGrace)

	result, err := st.SweepStaleDisconnectedRoomMembers(ctx, DefaultRoomDisconnectGrace)
	if err != nil {
		t.Fatalf("SweepStaleDisconnectedRoomMembers: %v", err)
	}
	if result.MembersRemoved < 1 {
		t.Fatalf("removed = %d, want at least the abandoned membership", result.MembersRemoved)
	}
	if result.MembersAfter != 0 {
		t.Fatalf("remaining = %d, want 0 — rows surviving their own sweep mean the"+
			" predicate and the write disagree", result.MembersAfter)
	}
	if got := roomStatus(t, st, ctx, room.ID); got != RoomStatusClosed {
		t.Fatalf("room status = %q, want %q", got, RoomStatusClosed)
	}
}

// A member still inside their window is not the sweep's business — it is the backstop
// for elapsed windows, not a second, coarser removal path.
func TestRoomSweepLeavesMembersInsideTheirWindow(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	userID := newPresenceUser(t, st, cleaner, ctx)

	room, _ := roomAndDisconnect(t, st, ctx, userID)

	if _, err := st.SweepStaleDisconnectedRoomMembers(ctx, DefaultRoomDisconnectGrace); err != nil {
		t.Fatalf("SweepStaleDisconnectedRoomMembers: %v", err)
	}
	if got := roomMemberCount(t, st, ctx, room.ID); got != 1 {
		t.Fatalf("room members = %d, want the player still inside their window", got)
	}
}

// Retention is the second half of removal: closing is what players see, deleting is the
// row going away. A room that closed a moment ago is still within reach of the regroup
// path, so only aged rows may go.
func TestDeleteRetiredRoomsOnlyTakesRoomsPastRetention(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	recent := newPresenceUser(t, st, cleaner, ctx)
	aged := newPresenceUser(t, st, cleaner, ctx)

	recentRoom, err := st.CreateRoom(ctx, recent)
	if err != nil {
		t.Fatalf("CreateRoom recent: %v", err)
	}
	if _, err := st.LeaveRoom(ctx, recent); err != nil {
		t.Fatalf("LeaveRoom recent: %v", err)
	}

	agedRoom, err := st.CreateRoom(ctx, aged)
	if err != nil {
		t.Fatalf("CreateRoom aged: %v", err)
	}
	if _, err := st.LeaveRoom(ctx, aged); err != nil {
		t.Fatalf("LeaveRoom aged: %v", err)
	}
	if _, err := st.db.ExecContext(ctx,
		`UPDATE rooms SET updated_at = NOW() - INTERVAL '1 day' WHERE id = $1`, agedRoom.ID,
	); err != nil {
		t.Fatalf("age the closed room: %v", err)
	}

	if _, err := st.DeleteRetiredRooms(ctx, DefaultClosedRoomRetention); err != nil {
		t.Fatalf("DeleteRetiredRooms: %v", err)
	}

	if got := roomStatus(t, st, ctx, recentRoom.ID); got != RoomStatusClosed {
		t.Fatalf("recently closed room status = %q, want it still present as %q",
			got, RoomStatusClosed)
	}
	var stillThere bool
	if err := st.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM rooms WHERE id = $1)`, agedRoom.ID,
	).Scan(&stillThere); err != nil {
		t.Fatalf("check aged room: %v", err)
	}
	if stillThere {
		t.Fatal("a room closed a day ago survived the retention sweep")
	}
}

// An OPEN room is never deleted however old it is. Age is only meaningful once a room
// has been closed; a long-running room that people are still in is not a retention
// candidate.
func TestDeleteRetiredRoomsNeverTakesAnOpenRoom(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	userID := newPresenceUser(t, st, cleaner, ctx)

	room, err := st.CreateRoom(ctx, userID)
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	if _, err := st.db.ExecContext(ctx,
		`UPDATE rooms SET updated_at = NOW() - INTERVAL '30 days' WHERE id = $1`, room.ID,
	); err != nil {
		t.Fatalf("age the room: %v", err)
	}

	if _, err := st.DeleteRetiredRooms(ctx, DefaultClosedRoomRetention); err != nil {
		t.Fatalf("DeleteRetiredRooms: %v", err)
	}
	if got := roomStatus(t, st, ctx, room.ID); got != RoomStatusOpen {
		t.Fatalf("open room status = %q, want it untouched at %q", got, RoomStatusOpen)
	}
}
