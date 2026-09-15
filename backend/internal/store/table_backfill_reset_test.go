package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// A table that reaches its match through matchmaking has to live the same life as one
// started with StartTable: started and pointed at its session while the match runs, forming
// and cleared when it ends. Nothing wrote the first half for a backfilled table, so the
// reset — which selects on exactly (session_id, started) — found nothing to undo and the
// group came home to last round's seats (JQ-298).
func mustTable(t *testing.T, st *Store, ctx context.Context, tableID uuid.UUID) *RoomTable {
	t.Helper()
	table, err := st.GetRoomTableByID(ctx, tableID)
	if err != nil {
		t.Fatalf("GetRoomTableByID(%s): %v", tableID, err)
	}
	return table
}

func assertTableStarted(t *testing.T, st *Store, ctx context.Context, label string, tableID, sessionID uuid.UUID) {
	t.Helper()
	table := mustTable(t, st, ctx, tableID)
	if table.Status != TableStatusStarted {
		t.Fatalf("%s: status = %q during the match, want %q", label, table.Status, TableStatusStarted)
	}
	if table.SessionID == nil || *table.SessionID != sessionID {
		t.Fatalf("%s: session = %v during the match, want %s", label, table.SessionID, sessionID)
	}
	seated, err := st.ListTableSeats(ctx, tableID)
	if err != nil {
		t.Fatalf("%s: ListTableSeats: %v", label, err)
	}
	if len(seated) != 0 {
		t.Fatalf("%s: %d seats still attached while the match runs, want none", label, len(seated))
	}
}

func assertTableReset(t *testing.T, st *Store, ctx context.Context, label string, tableID, sessionID uuid.UUID) {
	t.Helper()
	table := mustTable(t, st, ctx, tableID)
	if table.Status != TableStatusForming {
		t.Fatalf("%s: status = %q after the match, want %q", label, table.Status, TableStatusForming)
	}
	if table.SessionID != nil {
		t.Fatalf("%s: session = %s after the match, want none", label, *table.SessionID)
	}
	if table.RegroupSessionID == nil || *table.RegroupSessionID != sessionID {
		t.Fatalf("%s: regroup session = %v after the match, want %s", label, table.RegroupSessionID, sessionID)
	}
}

func seatedSet(t *testing.T, st *Store, ctx context.Context, tableID uuid.UUID) map[uuid.UUID]struct{} {
	t.Helper()
	ids := seatedUserIDs(t, st, ctx, tableID)
	out := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		out[id] = struct{}{}
	}
	return out
}

// The ticket's repro: two kings in separate rooms each press "Find more players" for a 1v1,
// and the two tables are matched into one session. Both tables belong to that match, so both
// follow it all the way through.
func TestBackfilledTablesFollowTheirMatchAndReset(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	game, mode := setupDuelMode(t, st, cleaner)
	queues, err := st.ListModeQueuesByModeID(ctx, mode.ID)
	if err != nil {
		t.Fatalf("ListModeQueuesByModeID: %v", err)
	}
	queueID := queues[0].ID

	seats, err := st.ListGameModeSeats(ctx, mode.ID)
	if err != nil {
		t.Fatalf("ListGameModeSeats: %v", err)
	}
	firstSeat := seats[0].SeatKey

	userA := mustUser(t, st, cleaner, "reset-a")
	userB := mustUser(t, st, cleaner, "reset-b")
	tableA := mustGroupTable(t, st, game, mode, userA, map[*User]string{userA: firstSeat})
	tableB := mustGroupTable(t, st, game, mode, userB, map[*User]string{userB: firstSeat})

	if _, err := st.StartTableBackfill(ctx, tableA.ID, userA.ID, queueID); err != nil {
		t.Fatalf("backfill A: %v", err)
	}
	if rec := mustReconcileForming(t, st, ctx, queueID); rec.Fired {
		t.Fatal("expected the first table to wait for an opponent")
	}
	if _, err := st.StartTableBackfill(ctx, tableB.ID, userB.ID, queueID); err != nil {
		t.Fatalf("backfill B: %v", err)
	}
	rec := mustReconcileForming(t, st, ctx, queueID)
	if !rec.Fired || rec.SessionID == nil {
		t.Fatalf("expected the two tables to complete a match, got %+v", rec)
	}
	sessionID := *rec.SessionID

	assertTableStarted(t, st, ctx, "table A", tableA.ID, sessionID)
	assertTableStarted(t, st, ctx, "table B", tableB.ID, sessionID)

	if err := st.CompleteSession(ctx, sessionID, time.Now()); err != nil {
		t.Fatalf("CompleteSession: %v", err)
	}

	assertTableReset(t, st, ctx, "table A", tableA.ID, sessionID)
	assertTableReset(t, st, ctx, "table B", tableB.ID, sessionID)

	// A duel has one seat class and nothing to pick, so each table re-seats the players
	// who arrived from it — and only those. The opponent played the same match from
	// another room and is not a member of this one (JQ-232, JQ-291).
	if seated := seatedSet(t, st, ctx, tableA.ID); len(seated) != 1 {
		t.Fatalf("table A: %d seats after the match, want just its own player", len(seated))
	} else if _, ok := seated[userA.ID]; !ok {
		t.Fatalf("table A: seated somebody other than its own player")
	}
	if seated := seatedSet(t, st, ctx, tableB.ID); len(seated) != 1 {
		t.Fatalf("table B: %d seats after the match, want just its own player", len(seated))
	} else if _, ok := seated[userB.ID]; !ok {
		t.Fatalf("table B: seated somebody other than its own player")
	}
}

// The reset used to read its table with QueryRowContext, so a session built from two tables
// had one of them picked arbitrarily and the other left stale for good. A 3v3 formed from two
// rooms is the ordinary way that happens.
func TestBothTablesOfATwoRoomMatchAreReset(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	game, mode, queueID := setupTeamMode(t, st, cleaner)
	seatKeys := []string{"Team-1-Seat-1", "Team-1-Seat-2", "Team-1-Seat-3"}

	newGroup := func(tag string) ([]*User, *RoomTable) {
		users := make([]*User, 3)
		seating := map[*User]string{}
		for i := range users {
			users[i] = mustUser(t, st, cleaner, tag)
			seating[users[i]] = seatKeys[i]
		}
		return users, mustGroupTable(t, st, game, mode, users[0], seating)
	}

	home, homeTable := newGroup("both-home")
	away, awayTable := newGroup("both-away")

	if _, err := st.StartTableBackfill(ctx, homeTable.ID, home[0].ID, queueID); err != nil {
		t.Fatalf("home group asks: %v", err)
	}
	if rec := mustReconcileForming(t, st, ctx, queueID); rec.Fired {
		t.Fatal("expected the first group to wait for opponents")
	}
	if _, err := st.StartTableBackfill(ctx, awayTable.ID, away[0].ID, queueID); err != nil {
		t.Fatalf("away group asks: %v", err)
	}
	rec := mustReconcileForming(t, st, ctx, queueID)
	if !rec.Fired || rec.SessionID == nil {
		t.Fatalf("expected the two groups to complete a match, got %+v", rec)
	}
	sessionID := *rec.SessionID

	assertTableStarted(t, st, ctx, "home table", homeTable.ID, sessionID)
	assertTableStarted(t, st, ctx, "away table", awayTable.ID, sessionID)

	if err := st.CompleteSession(ctx, sessionID, time.Now()); err != nil {
		t.Fatalf("CompleteSession: %v", err)
	}

	assertTableReset(t, st, ctx, "home table", homeTable.ID, sessionID)
	assertTableReset(t, st, ctx, "away table", awayTable.ID, sessionID)

	// Each side goes home with its own three, not with all six.
	for _, group := range []struct {
		label   string
		tableID uuid.UUID
		members []*User
		others  []*User
	}{
		{"home table", homeTable.ID, home, away},
		{"away table", awayTable.ID, away, home},
	} {
		seated := seatedSet(t, st, ctx, group.tableID)
		if len(seated) != len(group.members) {
			t.Fatalf("%s: %d seats after the match, want %d", group.label, len(seated), len(group.members))
		}
		for _, member := range group.members {
			if _, ok := seated[member.ID]; !ok {
				t.Fatalf("%s: its own player was not seated back", group.label)
			}
		}
		for _, other := range group.others {
			if _, ok := seated[other.ID]; ok {
				t.Fatalf("%s: seated a player from the other room", group.label)
			}
		}
	}
}

// Where the mode has something to choose, the table comes back empty — which is what makes
// "no stale seats" visible on its own, without a re-seat filling the same chairs back in.
// No regroup claim is involved: nobody presses "Another round" in this test, and JQ-291's
// clear-on-adopt is what used to hide the stale seats when somebody did.
func TestBackfilledTableComesBackEmptyWhenThereIsSomethingToChoose(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	game, mode := setupRolesMode(t, st, cleaner)
	if !ModeOffersPreMatchChoice(mode) {
		t.Fatal("setupRolesMode is supposed to offer a choice")
	}
	queues, err := st.ListModeQueuesByModeID(ctx, mode.ID)
	if err != nil {
		t.Fatalf("ListModeQueuesByModeID: %v", err)
	}
	queueID := queues[0].ID

	seats, err := st.ListGameModeSeats(ctx, mode.ID)
	if err != nil {
		t.Fatalf("ListGameModeSeats: %v", err)
	}

	// One player sits the clue giver's chair and asks for a guesser; a stranger takes it
	// from the catalog queue.
	king := mustUser(t, st, cleaner, "empty-king")
	stranger := mustUser(t, st, cleaner, "empty-stranger")
	table := mustGroupTable(t, st, game, mode, king, map[*User]string{king: seats[0].SeatKey})

	if _, err := st.StartTableBackfill(ctx, table.ID, king.ID, queueID); err != nil {
		t.Fatalf("StartTableBackfill: %v", err)
	}
	if rec := mustReconcileForming(t, st, ctx, queueID); rec.Fired {
		t.Fatal("expected the table to wait for a guesser")
	}
	strangerPath := ""
	if seats[1].QueuePath != nil {
		strangerPath = *seats[1].QueuePath
	}
	if _, err := st.JoinModeQueue(ctx, queueID, stranger.ID, strangerPath, nil); err != nil {
		t.Fatalf("JoinModeQueue: %v", err)
	}
	rec := mustReconcileForming(t, st, ctx, queueID)
	if !rec.Fired || rec.SessionID == nil {
		t.Fatalf("expected the stranger to complete the match, got %+v", rec)
	}
	sessionID := *rec.SessionID

	assertTableStarted(t, st, ctx, "table", table.ID, sessionID)

	if err := st.CompleteSession(ctx, sessionID, time.Now()); err != nil {
		t.Fatalf("CompleteSession: %v", err)
	}

	assertTableReset(t, st, ctx, "table", table.ID, sessionID)
	if seated := seatedSet(t, st, ctx, table.ID); len(seated) != 0 {
		t.Fatalf("table: %d seats left over from last round, want none", len(seated))
	}

	// And the stranger was never handed the room they played against. Their table seat
	// view is the room's invite code and id, which they have no business holding.
	started, err := st.GetUserStartedTableSession(ctx, stranger.ID)
	if err != nil {
		t.Fatalf("GetUserStartedTableSession: %v", err)
	}
	if started != nil {
		t.Fatalf("the catalog joiner was given room table %s", started.TableID)
	}
}
