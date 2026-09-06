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
