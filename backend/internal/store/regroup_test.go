package store

import (
	"context"
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
