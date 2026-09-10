package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// seatWithoutAnsweringThePicker plants a seat carrying an empty selection, the
// state the pre-JQ-211 regroup path wrote by calling sitAtTableTx with nil.
// Going around SitAtTableWithOptions is the point: the seat guard would refuse
// this now, and what is under test is whether provision refuses it too.
func seatWithoutAnsweringThePicker(t *testing.T, st *Store, ctx context.Context, tableID, userID uuid.UUID, seatKey string) {
	t.Helper()
	if _, err := st.db.ExecContext(ctx, `
		INSERT INTO table_seats (table_id, user_id, seat_key, queue_options)
		VALUES ($1, $2, $3, '[]')
	`, tableID, userID, seatKey); err != nil {
		t.Fatalf("plant seat: %v", err)
	}
}

// The regression JQ-211 names. A group comes back from a match, regroups, and
// sits down again; the mode requires two helpers. Neither the seat claim nor the
// provision may accept a seat that answered nothing — the second half is what
// used to let regroup hand a game an empty selection while RPSLR's default
// loadout made it look like a normal match.
func TestRegroupCannotReachProvisionWithoutTheRequiredSelection(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	game, mode, _ := setupHelpersMode(t, st, cleaner)
	sessionID, table, host, guest := playRoomTableMatch(t, st, ctx, cleaner, game, mode, helperPicks())

	// A returning group is seated by nobody (JQ-232), so both come back to an
	// empty table and claim their seats again.
	if _, _, err := st.ClaimRegroupTable(ctx, sessionID, host.ID); err != nil {
		t.Fatalf("ClaimRegroupTable host: %v", err)
	}
	if _, _, err := st.ClaimRegroupTable(ctx, sessionID, guest.ID); err != nil {
		t.Fatalf("ClaimRegroupTable guest: %v", err)
	}
	seats, err := st.ListGameModeSeats(ctx, mode.ID)
	if err != nil {
		t.Fatalf("ListGameModeSeats: %v", err)
	}

	if _, err := st.SitAtTableWithOptions(ctx, table.ID, host.ID, seats[0].SeatKey, nil); err == nil {
		t.Fatal("SitAtTableWithOptions = nil, want a seat claim refused for answering no required picker")
	}

	// The same state, reached the way the old regroup path reached it: beneath the
	// seat claim entirely. Provision is the backstop and has to hold on its own.
	seatWithoutAnsweringThePicker(t, st, ctx, table.ID, host.ID, seats[0].SeatKey)
	seatWithoutAnsweringThePicker(t, st, ctx, table.ID, guest.ID, seats[1].SeatKey)

	if _, err := st.StartTable(ctx, table.ID, host.ID); err == nil {
		t.Fatal("StartTable = nil, want provision refused for a seat with no selection")
	}

	// And the refusal is a refusal, not a half-started match: the table is still
	// forming, with both players still on it to go and pick.
	after, err := st.GetRoomTableByID(ctx, table.ID)
	if err != nil {
		t.Fatalf("GetRoomTableByID: %v", err)
	}
	if after.Status != TableStatusForming {
		t.Fatalf("table status = %q, want it left forming", after.Status)
	}
	if got := seatCountAtTable(t, st, ctx, table.ID); got != 2 {
		t.Fatalf("seats after the refused start = %d, want both players still seated", got)
	}
}

// A selection that does satisfy the mode still provisions, so the guard is a
// guard and not a wall — the same path, one field different.
func TestStartTableProvisionsASeatThatAnsweredThePicker(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	game, mode, _ := setupHelpersMode(t, st, cleaner)
	sessionID, _, host, _ := playRoomTableMatch(t, st, ctx, cleaner, game, mode, helperPicks())

	table, _, err := st.ClaimRegroupTable(ctx, sessionID, host.ID)
	if err != nil {
		t.Fatalf("ClaimRegroupTable: %v", err)
	}
	seats, err := st.ListGameModeSeats(ctx, mode.ID)
	if err != nil {
		t.Fatalf("ListGameModeSeats: %v", err)
	}
	if _, err := st.SitAtTableWithOptions(ctx, table.ID, host.ID, seats[0].SeatKey, helperPicks()); err != nil {
		t.Fatalf("host sit: %v", err)
	}
	guest := newRejoinUser(t, st, ctx, cleaner, "second-round")
	if err := st.addRoomMemberDirect(ctx, table.RoomID, guest.ID); err != nil {
		t.Fatalf("add guest: %v", err)
	}
	if _, err := st.SitAtTableWithOptions(ctx, table.ID, guest.ID, seats[1].SeatKey, helperPicks()); err != nil {
		t.Fatalf("guest sit: %v", err)
	}

	started, err := st.StartTable(ctx, table.ID, host.ID)
	if err != nil {
		t.Fatalf("StartTable: %v", err)
	}
	participants, err := st.ListSessionSeatAssignments(ctx, started.SessionID)
	if err != nil {
		t.Fatalf("ListSessionSeatAssignments: %v", err)
	}
	for _, p := range participants {
		if len(p.QueueOptions) != 1 || len(p.QueueOptions[0].OptionIDs) != 2 {
			t.Fatalf("provisioned options for %s = %+v, want the two helpers they picked", p.UserID, p.QueueOptions)
		}
	}
}

// A solo player's picks are replayed on regroup (JQ-232), but only while they
// still stand. A manifest that moved between rounds leaves them seatless and
// picking again rather than seated with a selection that cannot start.
func TestClaimRegroupTableDoesNotReplaySelectionsTheModeNoLongerAccepts(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	_, mode, queueID := setupHelpersMode(t, st, cleaner)
	first := newRejoinUser(t, st, ctx, cleaner, "solo-a")
	second := newRejoinUser(t, st, ctx, cleaner, "solo-b")
	if _, err := st.JoinModeQueueWithOptions(ctx, queueID, first.ID, "", helperPicks(), nil); err != nil {
		t.Fatalf("join first: %v", err)
	}
	if _, err := st.JoinModeQueueWithOptions(ctx, queueID, second.ID, "", helperPicks(), nil); err != nil {
		t.Fatalf("join second: %v", err)
	}
	mustReconcileForming(t, st, ctx, queueID)
	session, err := st.GetMatchedSessionForUserAndModeQueue(ctx, queueID, first.ID)
	if err != nil {
		t.Fatalf("GetMatchedSessionForUserAndModeQueue: %v", err)
	}
	if err := st.CompleteSession(ctx, session.ID, time.Now()); err != nil {
		t.Fatalf("CompleteSession: %v", err)
	}

	// The game ships a new manifest between rounds: helpers now come with a deck,
	// and last round's two picks no longer answer the mode.
	if _, err := st.db.ExecContext(ctx, `
		UPDATE game_modes SET pre_queue = $2 WHERE id = $1
	`, mode.ID, `{"groups":[
		{"key":"helpers","kind":"Loadout","label":"Choose your two helpers","min":2,"max":2},
		{"key":"deck","kind":"Deck","label":"Choose a deck","min":1,"max":1}
	]}`); err != nil {
		t.Fatalf("update declaration: %v", err)
	}

	table, _, err := st.ClaimRegroupTable(ctx, session.ID, first.ID)
	if err != nil {
		t.Fatalf("ClaimRegroupTable: %v", err)
	}
	if got := seatCountAtTable(t, st, ctx, table.ID); got != 0 {
		t.Fatalf("seats after claim = %d, want 0 — a stale selection is not replayed", got)
	}
}
