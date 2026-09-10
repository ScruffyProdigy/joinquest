package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/lfg/partytree"
)

// The reason a deferral is worth anything.
//
// Holding the fire accumulates candidates in the waiting pool, but placement is
// greedy and nothing ever moves a player already on the map -- so without a
// reselection step the hold expires on the identical lobby, having cost every
// player the wait and bought nothing. This is the step: while the fire is
// deferred, a placed party gives up its seats to a waiting party of the same
// shape when doing so tightens the lobby.
func TestADeferredLobbyTakesABetterCandidateFromTheWaitingPool(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	gameID, queueID := duelQueueForSkill(t, st, cleaner, ctx)
	seedArrivals(t, st, cleaner, ctx, gameID, queueID, 200)

	newUser := func(tag string) uuid.UUID {
		user, err := st.CreateUser(ctx, CreateUserParams{Email: tag + "-" + uuid.NewString() + "@example.com"})
		if err != nil {
			t.Fatalf("CreateUser %s: %v", tag, err)
		}
		cleaner.TrackUser(user.ID)
		return user.ID
	}

	anchor := newUser("anchor")
	mismatch := newUser("mismatch")
	better := newUser("better")

	rateUsers(t, st, ctx, gameID, map[uuid.UUID]float64{anchor: 25, mismatch: 45, better: 26})

	// The first two fill the map and are far enough apart to be deferred.
	for _, id := range []uuid.UUID{anchor, mismatch} {
		if _, err := st.JoinModeQueue(ctx, queueID, id, "", nil); err != nil {
			t.Fatalf("JoinModeQueue: %v", err)
		}
	}
	if rec := mustReconcileForming(t, st, ctx, queueID); rec.Fired {
		t.Fatalf("the wide lobby fired instead of deferring")
	}

	// A candidate who would tighten the lobby arrives while it is deferred.
	if _, err := st.JoinModeQueue(ctx, queueID, better, "", nil); err != nil {
		t.Fatalf("JoinModeQueue better: %v", err)
	}

	rec := mustReconcileForming(t, st, ctx, queueID)
	if !rec.Fired || rec.SessionID == nil {
		t.Fatalf("the lobby did not fire after a better candidate arrived: %+v", rec)
	}

	seats, err := st.ListSessionSeatAssignments(ctx, *rec.SessionID)
	if err != nil {
		t.Fatalf("ListSessionSeatAssignments: %v", err)
	}
	seated := map[uuid.UUID]bool{}
	for _, seat := range seats {
		seated[seat.UserID] = true
	}
	if !seated[anchor] || !seated[better] {
		t.Errorf("the formed match should hold the two close players, got %v", seated)
	}
	if seated[mismatch] {
		t.Errorf("the mismatched player was seated anyway; the swap did not happen")
	}

	// Displaced, not ejected: they keep their place in line for the next table.
	var stillWaiting int
	if err := st.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM game_queues
		WHERE mode_queue_id = $1 AND user_id = $2 AND status = 'waiting'
	`, queueID, mismatch).Scan(&stillWaiting); err != nil {
		t.Fatalf("check displaced player: %v", err)
	}
	if stillWaiting != 1 {
		t.Errorf("the displaced player has %d waiting rows, want 1; a swap must not eject anybody", stillWaiting)
	}
}

// A party is never silently split. Its members enter dispersion individually --
// a party spanning a wide range is itself a dispersion cost and is charged as
// one -- but they are seated and displaced together or not at all.
//
// The case: a wide two-person party holds two seats, and a single solo
// candidate would tighten the lobby if it could displace just one of them. It
// cannot. The shape does not match, so the lobby rides out its budget with the
// party intact rather than seating half of it.
func TestASwapNeverSplitsAPartyToTightenALobby(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	gameID, queueID := seatedQueueForSkill(t, st, cleaner, ctx, 4)
	seedArrivals(t, st, cleaner, ctx, gameID, queueID, 200)

	newUser := func(tag string) uuid.UUID {
		user, err := st.CreateUser(ctx, CreateUserParams{Email: tag + "-" + uuid.NewString() + "@example.com"})
		if err != nil {
			t.Fatalf("CreateUser %s: %v", tag, err)
		}
		cleaner.TrackUser(user.ID)
		return user.ID
	}

	wideA, wideB := newUser("wide-a"), newUser("wide-b")
	soloA, soloB := newUser("solo-a"), newUser("solo-b")
	candidate := newUser("candidate")

	// The party spans the whole scale; everybody else sits at the centre.
	rateUsers(t, st, ctx, gameID, map[uuid.UUID]float64{
		wideA: 5, wideB: 45, soloA: 25, soloB: 25, candidate: 25,
	})

	if _, err := st.JoinModeQueue(ctx, queueID, wideA, "", &JoinPartyInput{
		Tree: partytree.Node{Role: "Player", Members: []string{wideA.String(), wideB.String()}},
		Members: []JoinPartyMemberInput{
			{UserID: wideA, QueuePath: ""},
			{UserID: wideB, QueuePath: ""},
		},
	}); err != nil {
		t.Fatalf("party join: %v", err)
	}
	for _, id := range []uuid.UUID{soloA, soloB} {
		if _, err := st.JoinModeQueue(ctx, queueID, id, "", nil); err != nil {
			t.Fatalf("JoinModeQueue: %v", err)
		}
	}
	mustReconcileForming(t, st, ctx, queueID)

	// A single close player who would tighten the lobby -- if half the party
	// could be evicted for them.
	if _, err := st.JoinModeQueue(ctx, queueID, candidate, "", nil); err != nil {
		t.Fatalf("JoinModeQueue candidate: %v", err)
	}
	mustReconcileForming(t, st, ctx, queueID)

	seatedA, seatedB := partyMemberIsSeated(t, st, ctx, queueID, wideA), partyMemberIsSeated(t, st, ctx, queueID, wideB)
	if seatedA != seatedB {
		t.Errorf("the party was split: wideA seated = %v, wideB seated = %v", seatedA, seatedB)
	}
	// Both, specifically -- not neither. No candidate matches the party's
	// shape, so the correct outcome is that nothing moves. Asserting only that
	// the two agree would also accept a swap that vacated both seats and
	// filled neither, which loses a seat rather than protecting a party.
	if !seatedA || !seatedB {
		t.Errorf("no swap was possible, so the party should still hold its seats; wideA = %v, wideB = %v", seatedA, seatedB)
	}
}

// partyMemberIsSeated reports whether the user currently holds a seat on the
// queue's forming map.
func partyMemberIsSeated(t *testing.T, st *Store, ctx context.Context, queueID, userID uuid.UUID) bool {
	t.Helper()
	var count int
	if err := st.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM forming_match_assignments fma
		INNER JOIN forming_matches fm ON fm.id = fma.forming_match_id
		WHERE fm.mode_queue_id = $1 AND fma.user_id = $2
	`, queueID, userID).Scan(&count); err != nil {
		t.Fatalf("check seated: %v", err)
	}
	return count > 0
}
