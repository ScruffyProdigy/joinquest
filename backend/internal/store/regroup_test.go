package store

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestClaimRegroupTableConvergesOnOneTable(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	sessionID, userA, userB := seedMatchedSession(t, st, ctx, cleaner)
	if err := st.CompleteSession(ctx, sessionID, time.Now()); err != nil {
		t.Fatalf("CompleteSession: %v", err)
	}

	var wg sync.WaitGroup
	tables := make([]*RoomTable, 2)
	errs := make([]error, 2)
	for i, uid := range []uuid.UUID{userA, userB} {
		wg.Add(1)
		go func(i int, uid uuid.UUID) {
			defer wg.Done()
			tables[i], _, errs[i] = st.ClaimRegroupTable(ctx, sessionID, uid)
		}(i, uid)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("ClaimRegroupTable[%d]: %v", i, err)
		}
	}
	if tables[0].ID != tables[1].ID {
		t.Fatalf("players landed on different tables: %s vs %s", tables[0].ID, tables[1].ID)
	}

	seats, err := st.ListTableSeats(ctx, tables[0].ID)
	if err != nil {
		t.Fatalf("ListTableSeats: %v", err)
	}
	if len(seats) != 2 {
		t.Fatalf("seats = %d, want 2", len(seats))
	}
}

func TestClaimRegroupTableRefusesNonParticipant(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	sessionID, _, _ := seedMatchedSession(t, st, ctx, cleaner)
	if err := st.CompleteSession(ctx, sessionID, time.Now()); err != nil {
		t.Fatalf("CompleteSession: %v", err)
	}

	outsider, err := st.CreateUser(ctx, CreateUserParams{Email: "out-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(outsider.ID)

	if _, _, err := st.ClaimRegroupTable(ctx, sessionID, outsider.ID); err == nil {
		t.Fatal("expected ClaimRegroupTable to refuse a non-participant")
	}
}

func TestClaimRegroupTableRefusesUnfinishedSession(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	// Seeded but deliberately not completed. A player who finishes early reaches /return
	// while the match is still running for everyone else, and AcknowledgePlayerReturn
	// stamps finished_at and releases their matched queue row — so every guard downstream
	// of the claim lets them through, and only the status check can stop them.
	sessionID, userA, _ := seedMatchedSession(t, st, ctx, cleaner)
	if err := st.AcknowledgePlayerReturn(ctx, sessionID, userA, time.Now()); err != nil {
		t.Fatalf("AcknowledgePlayerReturn: %v", err)
	}

	if _, _, err := st.ClaimRegroupTable(ctx, sessionID, userA); !errors.Is(err, ErrSessionNotFinished) {
		t.Fatalf("ClaimRegroupTable on active session = %v, want ErrSessionNotFinished", err)
	}

	// Nothing was recorded either: a table stamped now would be overwritten by
	// resetRoomTableAfterSessionTx when CompleteSession runs, stranding this claimant on a
	// table of their own while every later claimant adopts the original.
	var regroupID *uuid.UUID
	if err := st.db.QueryRowContext(ctx, `
		SELECT regroup_table_id FROM game_sessions WHERE id = $1
	`, sessionID).Scan(&regroupID); err != nil {
		t.Fatalf("read regroup_table_id: %v", err)
	}
	if regroupID != nil {
		t.Fatalf("regroup_table_id = %s, want NULL until the match is finished", *regroupID)
	}

	// The refusal is about the session being unfinished, not about the caller: the same
	// claim succeeds once the match completes.
	if err := st.CompleteSession(ctx, sessionID, time.Now()); err != nil {
		t.Fatalf("CompleteSession: %v", err)
	}
	if _, _, err := st.ClaimRegroupTable(ctx, sessionID, userA); err != nil {
		t.Fatalf("ClaimRegroupTable after completion: %v", err)
	}
}

func TestClaimRegroupTableRefusesFullTableWithoutOptingIn(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	sessionID, userA, userB := seedMatchedSession(t, st, ctx, cleaner)
	if err := st.CompleteSession(ctx, sessionID, time.Now()); err != nil {
		t.Fatalf("CompleteSession: %v", err)
	}

	table, room, err := st.ClaimRegroupTable(ctx, sessionID, userA)
	if err != nil {
		t.Fatalf("ClaimRegroupTable A: %v", err)
	}

	// The catalog-origin regroup table is created in A's pre-existing room, so an
	// unrelated room member can take the last seat through the ordinary sit mutation.
	outsider, err := st.CreateUser(ctx, CreateUserParams{Email: "full-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser outsider: %v", err)
	}
	cleaner.TrackUser(outsider.ID)
	if _, err := st.JoinRoom(ctx, outsider.ID, room.InviteCode); err != nil {
		t.Fatalf("JoinRoom outsider: %v", err)
	}

	seatKey := onlyOpenSeatKey(t, st, ctx, table)
	if _, err := st.SitAtTable(ctx, table.ID, outsider.ID, seatKey); err != nil {
		t.Fatalf("SitAtTable outsider: %v", err)
	}

	if _, _, err := st.ClaimRegroupTable(ctx, sessionID, userB); !errors.Is(err, ErrTableFull) {
		t.Fatalf("ClaimRegroupTable B on full table = %v, want ErrTableFull", err)
	}

	var optedIn *time.Time
	if err := st.db.QueryRowContext(ctx, `
		SELECT regroup_opted_in_at FROM game_session_participants
		WHERE session_id = $1 AND user_id = $2
	`, sessionID, userB).Scan(&optedIn); err != nil {
		t.Fatalf("read regroup_opted_in_at: %v", err)
	}
	if optedIn != nil {
		t.Fatal("player was stamped as opted in without holding a seat")
	}

	seats, err := st.ListTableSeats(ctx, table.ID)
	if err != nil {
		t.Fatalf("ListTableSeats: %v", err)
	}
	for _, seat := range seats {
		if seat.UserID == userB {
			t.Fatal("player was seated at a table reported as full")
		}
	}
}

func TestGetRegroupRosterThreeStates(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	sessionID, userA, userB := seedMatchedSession(t, st, ctx, cleaner)
	if err := st.CompleteSession(ctx, sessionID, time.Now()); err != nil {
		t.Fatalf("CompleteSession: %v", err)
	}

	roster, err := st.GetRegroupRoster(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetRegroupRoster: %v", err)
	}
	if roster[userA] != RegroupPending || roster[userB] != RegroupPending {
		t.Fatalf("before anyone acts both should be PENDING, got %v / %v", roster[userA], roster[userB])
	}

	if _, _, err := st.ClaimRegroupTable(ctx, sessionID, userA); err != nil {
		t.Fatalf("ClaimRegroupTable: %v", err)
	}
	if _, err := st.DeclineRegroup(ctx, sessionID, userB, time.Now()); err != nil {
		t.Fatalf("DeclineRegroup: %v", err)
	}

	roster, err = st.GetRegroupRoster(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetRegroupRoster: %v", err)
	}
	if roster[userA] != RegroupIn {
		t.Errorf("userA = %v, want IN", roster[userA])
	}
	if roster[userB] != RegroupOut {
		t.Errorf("userB = %v, want OUT", roster[userB])
	}
}

// TestGetRegroupRosterDoesNotInferInFromSeat proves the crux of the design: a room-table
// group is re-seated by resetRoomTableAfterSessionTx the instant their match completes,
// before either player has clicked anything. If GetRegroupRoster ever derived IN from
// holding a seat at the regroup table (an earlier, wrong draft of this design), this test
// would catch it — both players hold seats here yet neither has opted in.
func TestGetRegroupRosterDoesNotInferInFromSeat(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	host, err := st.CreateUser(ctx, CreateUserParams{Email: "regroup-host-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser host: %v", err)
	}
	cleaner.TrackUser(host.ID)
	guest, err := st.CreateUser(ctx, CreateUserParams{Email: "regroup-guest-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser guest: %v", err)
	}
	cleaner.TrackUser(guest.ID)

	game, mode := setupDuelMode(t, st, cleaner)

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
	if len(seats) < 2 {
		t.Fatalf("need at least 2 seats, got %d", len(seats))
	}

	if _, err := st.SitAtTable(ctx, table.ID, host.ID, seats[0].SeatKey); err != nil {
		t.Fatalf("host sit: %v", err)
	}
	if _, err := st.SitAtTable(ctx, table.ID, guest.ID, seats[1].SeatKey); err != nil {
		t.Fatalf("guest sit: %v", err)
	}

	result, err := st.StartTable(ctx, table.ID, host.ID)
	if err != nil {
		t.Fatalf("StartTable: %v", err)
	}

	if err := st.CompleteSession(ctx, result.SessionID, time.Now()); err != nil {
		t.Fatalf("CompleteSession: %v", err)
	}

	seated, err := st.ListTableSeats(ctx, table.ID)
	if err != nil {
		t.Fatalf("ListTableSeats: %v", err)
	}
	if len(seated) != 2 {
		t.Fatalf("expected both players re-seated after completion, got %d", len(seated))
	}

	roster, err := st.GetRegroupRoster(ctx, result.SessionID)
	if err != nil {
		t.Fatalf("GetRegroupRoster: %v", err)
	}
	if roster[host.ID] != RegroupPending {
		t.Errorf("seated-but-unconfirmed host = %v, want PENDING", roster[host.ID])
	}
	if roster[guest.ID] != RegroupPending {
		t.Errorf("seated-but-unconfirmed guest = %v, want PENDING", roster[guest.ID])
	}
}

// onlyOpenSeatKey returns the remaining open seat key on the regroup table.
func onlyOpenSeatKey(t *testing.T, st *Store, ctx context.Context, table *RoomTable) string {
	t.Helper()
	modeSeats, err := st.ListGameModeSeats(ctx, table.ModeID)
	if err != nil {
		t.Fatalf("ListGameModeSeats: %v", err)
	}
	seated, err := st.ListTableSeats(ctx, table.ID)
	if err != nil {
		t.Fatalf("ListTableSeats: %v", err)
	}
	taken := make(map[string]bool, len(seated))
	for _, seat := range seated {
		taken[seat.SeatKey] = true
	}
	for _, seat := range modeSeats {
		if !taken[seat.SeatKey] {
			return seat.SeatKey
		}
	}
	t.Fatal("expected an open seat on the regroup table")
	return ""
}

// TestGetSessionIDByRegroupTableFollowsTheLatestMatch pins the ordering half of the reverse
// lookup. regroup_table_id has no uniqueness constraint and nothing ever clears it, so a
// group that plays a second match at the same table leaves TWO rows carrying that table id:
// ClaimRegroupTable stamps the first match's session, and CompleteSession's
// resetRoomTableAfterSessionTx stamps the second's without touching the first. An unordered
// SELECT is free to answer with either — here the stale row is physically first, so it
// reliably answers with the wrong match — and the roster then names players who already
// left while omitting anyone who backfilled since.
func TestGetSessionIDByRegroupTableFollowsTheLatestMatch(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	firstSession, userA, _ := seedMatchedSession(t, st, ctx, cleaner)
	if err := st.CompleteSession(ctx, firstSession, time.Now()); err != nil {
		t.Fatalf("CompleteSession: %v", err)
	}
	table, _, err := st.ClaimRegroupTable(ctx, firstSession, userA)
	if err != nil {
		t.Fatalf("ClaimRegroupTable: %v", err)
	}

	// One match points at the table so far, so the lookup is unambiguous.
	got, err := st.GetSessionIDByRegroupTable(ctx, table.ID)
	if err != nil {
		t.Fatalf("GetSessionIDByRegroupTable: %v", err)
	}
	if got == nil || *got != firstSession {
		t.Fatalf("lookup = %v, want the only match %s", got, firstSession)
	}

	// The group plays again at the same table. The second match starts later, ends there,
	// and is stamped with the same regroup table; the first row stays behind unchanged.
	var secondSession uuid.UUID
	if err := st.db.QueryRowContext(ctx, `
		INSERT INTO game_sessions (game_id, status, mode_id, started_at, ended_at, regroup_table_id)
		SELECT game_id, 'completed', mode_id, started_at + interval '1 hour', NOW(), $2
		FROM game_sessions
		WHERE id = $1
		RETURNING id
	`, firstSession, table.ID).Scan(&secondSession); err != nil {
		t.Fatalf("insert the second match: %v", err)
	}
	t.Cleanup(func() {
		_, _ = st.db.ExecContext(context.Background(), `DELETE FROM game_sessions WHERE id = $1`, secondSession)
	})

	got, err = st.GetSessionIDByRegroupTable(ctx, table.ID)
	if err != nil {
		t.Fatalf("GetSessionIDByRegroupTable after the second match: %v", err)
	}
	if got == nil {
		t.Fatal("lookup returned no session for a table two matches point at")
	}
	if *got == firstSession {
		t.Fatalf("lookup returned the stale first match %s; want the latest %s", firstSession, secondSession)
	}
	if *got != secondSession {
		t.Fatalf("lookup = %s, want the latest match %s", *got, secondSession)
	}
}

// TestGetSessionIDByRegroupTableNilForOrdinaryTable keeps the "no originating match" case an
// ordinary nil answer rather than an error: every table not reached through playAgain hits
// this path on every render.
func TestGetSessionIDByRegroupTableNilForOrdinaryTable(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	got, err := st.GetSessionIDByRegroupTable(ctx, uuid.New())
	if err != nil {
		t.Fatalf("GetSessionIDByRegroupTable = %v, want a nil id and no error", err)
	}
	if got != nil {
		t.Fatalf("lookup = %s, want nil for a table no match points at", *got)
	}
}

// TestClaimRegroupTableRebuildsWhenTheRoomClosed is the bricked-forever case. The table a
// match converged on outlives its room: once the last member leaves, leaveRoomTx closes the
// room but regroup_table_id still points at the surviving table. Adopting it would insert
// the next claimant into a closed room, and sitAtTableTx's isRoomMemberTx requires an open
// one — so the claim fails ErrNotFound, which the resolver reports as "you did not play in
// this match", permanently. A closed room has to read as unclaimed so a fresh table is built.
func TestClaimRegroupTableRebuildsWhenTheRoomClosed(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	sessionID, userA, userB := seedMatchedSession(t, st, ctx, cleaner)
	if err := st.CompleteSession(ctx, sessionID, time.Now()); err != nil {
		t.Fatalf("CompleteSession: %v", err)
	}

	abandoned, _, err := st.ClaimRegroupTable(ctx, sessionID, userA)
	if err != nil {
		t.Fatalf("ClaimRegroupTable A: %v", err)
	}

	// A is the only member of the room the claim built, so leaving closes it while the
	// table — and game_sessions.regroup_table_id — survive.
	if _, err := st.LeaveRoom(ctx, userA); err != nil {
		t.Fatalf("LeaveRoom A: %v", err)
	}
	room, err := st.GetRoomByID(ctx, abandoned.RoomID)
	if err != nil {
		t.Fatalf("GetRoomByID: %v", err)
	}
	if room.Status != RoomStatusClosed {
		t.Fatalf("room status = %q, want %q — the test's premise did not hold", room.Status, RoomStatusClosed)
	}
	survivor, err := st.GetRoomTableByID(ctx, abandoned.ID)
	if err != nil {
		t.Fatalf("GetRoomTableByID: %v — the table must outlive the room for this to bite", err)
	}
	if survivor.Status != TableStatusForming {
		t.Fatalf("table status = %q, want %q — the test's premise did not hold", survivor.Status, TableStatusForming)
	}

	table, _, err := st.ClaimRegroupTable(ctx, sessionID, userB)
	if err != nil {
		t.Fatalf("ClaimRegroupTable B after the room closed: %v", err)
	}
	if table.ID == abandoned.ID {
		t.Fatal("B was seated at the table in the closed room; want a fresh one")
	}

	seats, err := st.ListTableSeats(ctx, table.ID)
	if err != nil {
		t.Fatalf("ListTableSeats: %v", err)
	}
	if len(seats) != 1 || seats[0].UserID != userB {
		t.Fatalf("seats = %+v, want B alone on the rebuilt table", seats)
	}
}

// TestDeclineRegroupReportsTheFreedSeat pins the payload the resolver publishes
// tableUpdated from. Everyone at /room/{code} watches tableUpdated, not
// matchResultUpdated, so without this the decliner stays visibly seated.
func TestDeclineRegroupReportsTheFreedSeat(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	sessionID, userA, userB := seedMatchedSession(t, st, ctx, cleaner)
	if err := st.CompleteSession(ctx, sessionID, time.Now()); err != nil {
		t.Fatalf("CompleteSession: %v", err)
	}

	table, _, err := st.ClaimRegroupTable(ctx, sessionID, userA)
	if err != nil {
		t.Fatalf("ClaimRegroupTable A: %v", err)
	}
	if _, _, err := st.ClaimRegroupTable(ctx, sessionID, userB); err != nil {
		t.Fatalf("ClaimRegroupTable B: %v", err)
	}

	release, err := st.DeclineRegroup(ctx, sessionID, userB, time.Now())
	if err != nil {
		t.Fatalf("DeclineRegroup B: %v", err)
	}
	if release == nil {
		t.Fatal("DeclineRegroup freed a seat but reported no table to publish")
	}
	if release.TableID != table.ID {
		t.Errorf("release.TableID = %s, want %s", release.TableID, table.ID)
	}
	if release.RoomID != table.RoomID {
		t.Errorf("release.RoomID = %s, want %s", release.RoomID, table.RoomID)
	}

	// Declining twice frees nothing the second time, and must not ask the resolver to
	// publish a table change that did not happen.
	again, err := st.DeclineRegroup(ctx, sessionID, userB, time.Now())
	if err != nil {
		t.Fatalf("DeclineRegroup B again: %v", err)
	}
	if again != nil {
		t.Fatalf("second decline reported a freed seat: %+v", again)
	}
}

// TestGetRegroupTableIDStopsAdvertisingAClosedRoomsOffer is the read half of
// TestClaimRegroupTableRebuildsWhenTheRoomClosed, and the two used to disagree.
//
// Same setup: the table a match converged on outlives its room, and regroup_table_id still
// points at it. ClaimRegroupTable has always read that as unclaimed. GetRegroupTableID read
// the column bare, so regroupInviteCode kept putting the dead table's room code on every
// participant's results screen — an invite that rendered as live and dead-ended on use,
// because following it lands in ClaimRegroupTable, which refuses the table and builds
// another. Both now apply liveRegroupTableClause, so there is one answer to "where is this
// match's regroup offer" instead of two.
func TestGetRegroupTableIDStopsAdvertisingAClosedRoomsOffer(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	sessionID, userA, _ := seedMatchedSession(t, st, ctx, cleaner)
	if err := st.CompleteSession(ctx, sessionID, time.Now()); err != nil {
		t.Fatalf("CompleteSession: %v", err)
	}

	claimed, _, err := st.ClaimRegroupTable(ctx, sessionID, userA)
	if err != nil {
		t.Fatalf("ClaimRegroupTable: %v", err)
	}

	// While the offer is live the lookup must still answer, or this test would pass on a
	// function that had simply stopped working.
	live, err := st.GetRegroupTableID(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetRegroupTableID while live: %v", err)
	}
	if live == nil || *live != claimed.ID {
		t.Fatalf("live lookup = %v, want the claimed table %s", live, claimed.ID)
	}

	// A is the room's only member, so leaving closes it while the table survives.
	if _, err := st.LeaveRoom(ctx, userA); err != nil {
		t.Fatalf("LeaveRoom A: %v", err)
	}
	room, err := st.GetRoomByID(ctx, claimed.RoomID)
	if err != nil {
		t.Fatalf("GetRoomByID: %v", err)
	}
	if room.Status != RoomStatusClosed {
		t.Fatalf("room status = %q, want %q — the test's premise did not hold", room.Status, RoomStatusClosed)
	}
	if _, err := st.GetRoomTableByID(ctx, claimed.ID); err != nil {
		t.Fatalf("GetRoomTableByID: %v — the table must outlive the room for this to bite", err)
	}

	got, err := st.GetRegroupTableID(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetRegroupTableID after the room closed: %v", err)
	}
	if got != nil {
		t.Fatalf("lookup = %s, want nil: the offer died with its room, so no invite code"+
			" should reach the results screen", *got)
	}

	// A session that does not exist is still an error, not a quiet nil — the caller
	// distinguishes "no live offer" from "no such match".
	if _, err := st.GetRegroupTableID(ctx, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetRegroupTableID for an unknown session = %v, want ErrNotFound", err)
	}
}
