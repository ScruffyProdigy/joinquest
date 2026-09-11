package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// JQ-299. The presence gate is written per player; a party is placed and vacated
// all-or-nothing. Where the two met, a group of three with two members away read as
// two independent absences, gave up two of its three chairs, and could never take
// them back — partyAssignedOnFormingTx answers "is any member assigned", so the
// party was skipped by syncWaitingPartiesOnFormingTx for as long as the third
// member stayed seated. The group was then split across the very match it queued
// together for.
//
// The rule these tests state: a party counts as present if any member is present,
// and is placed or vacated as one unit. Groups are deliberately more lenient than
// solo players — a present member can tell an absent one that the game has started,
// and being in contact with each other is what made them a group in the first place.

// mustBackfillingGroup seats `size` friends at a table in a 3v3 and has the king ask
// the lobby for the rest of the match, leaving the whole group on the filling map.
// This is where the repro starts.
func mustBackfillingGroup(t *testing.T, st *Store, cleaner *TestCleaner, size int) ([]*User, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	game, mode, queueID := setupTeamMode(t, st, cleaner)

	seatKeys := []string{"Team-1-Seat-1", "Team-1-Seat-2", "Team-1-Seat-3"}
	friends := make([]*User, size)
	seating := map[*User]string{}
	for i := range friends {
		friends[i] = mustUser(t, st, cleaner, "jq299-friend")
		seating[friends[i]] = seatKeys[i]
	}
	table := mustGroupTable(t, st, game, mode, friends[0], seating)

	if _, err := st.StartTableBackfill(ctx, table.ID, friends[0].ID, queueID); err != nil {
		t.Fatalf("StartTableBackfill: %v", err)
	}
	if rec := mustReconcileForming(t, st, ctx, queueID); rec.Fired {
		t.Fatal("fixture fired with only the group present, so nothing here is on a filling match")
	}
	for i, friend := range friends {
		if n := assignedSeatCount(t, st, ctx, friend.ID); n != 1 {
			t.Fatalf("fixture never placed friend %d on the forming map (%d seats), so this test proves nothing", i, n)
		}
	}
	return friends, queueID
}

// queueStrangers puts `n` unrelated players in the queue, each on their own, so the
// map completes and the fire path runs.
func queueStrangers(t *testing.T, st *Store, cleaner *TestCleaner, ctx context.Context, queueID uuid.UUID, n int) []*User {
	t.Helper()
	strangers := make([]*User, n)
	for i := range strangers {
		strangers[i] = mustUser(t, st, cleaner, "jq299-stranger")
		if _, err := st.JoinModeQueue(ctx, queueID, strangers[i].ID, "", nil); err != nil {
			t.Fatalf("stranger %d joining: %v", i, err)
		}
	}
	return strangers
}

// assertMatchedTogether is the whole point of a group queuing as a group: everybody
// reached the match, the friends are on one side, and no stranger is among them.
func assertMatchedTogether(t *testing.T, st *Store, ctx context.Context, sessionID uuid.UUID, friends, strangers []*User) {
	t.Helper()
	assignments, err := st.ListSessionSeatAssignments(ctx, sessionID)
	if err != nil {
		t.Fatalf("ListSessionSeatAssignments: %v", err)
	}
	if want := len(friends) + len(strangers); len(assignments) != want {
		t.Fatalf("match seated %d players, want %d", len(assignments), want)
	}
	sideByUser := map[uuid.UUID]string{}
	for _, a := range assignments {
		sideByUser[a.UserID] = sideOf(a.SeatKey)
	}
	groupSide, ok := sideByUser[friends[0].ID]
	if !ok {
		t.Fatal("the king did not reach the match")
	}
	for i, friend := range friends {
		side, ok := sideByUser[friend.ID]
		if !ok {
			t.Fatalf("friend %d never reached the match; the group was left behind by it", i)
		}
		if side != groupSide {
			t.Fatalf("the group was split across sides: %s and %s", groupSide, side)
		}
	}
	for i, stranger := range strangers {
		if sideByUser[stranger.ID] == groupSide {
			t.Fatalf("stranger %d was seated on the group's own side (%s)", i, groupSide)
		}
	}
}

// The lenient rule, asserted directly so a later tightening cannot pass silently: a
// group is placed and fires while some of it is not watching, because the members who
// are watching can say so.
//
// The two-away case is the ticket's repro. Before the fix it vacated two of the three
// chairs, stranded those two in the queue behind a party that could never be placed
// again, and fired the match with their friend and two strangers in their place.
func TestAGroupIsPlacedAndFiresWhileSomeOfItIsAway(t *testing.T) {
	cases := []struct {
		name      string
		awayCount int
	}{
		{name: "one of three away", awayCount: 1},
		{name: "two of three away", awayCount: 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := openTestStore(t)
			cleaner := st.NewTestCleaner(t)
			ctx := context.Background()

			friends, queueID := mustBackfillingGroup(t, st, cleaner, 3)
			for _, friend := range friends[1 : 1+tc.awayCount] {
				goAway(t, st, ctx, friend.ID)
			}

			strangers := queueStrangers(t, st, cleaner, ctx, queueID, 3)

			rec := mustReconcileForming(t, st, ctx, queueID)
			if !rec.Fired || rec.SessionID == nil {
				t.Fatalf("the match did not fire with a partly-away group in it, got %+v", rec)
			}
			assertMatchedTogether(t, st, ctx, *rec.SessionID, friends, strangers)
		})
	}
}

// A group with nobody watching is a real absence, and gets exactly what a solo player
// gets: one hold, then its chairs back to the pool, then its place in the queue. The
// last step is the one that was broken — the party was released a member at a time, so
// whatever was left of it on the map made it unplaceable for good.
func TestAGroupWithEveryoneAwayIsHeldAndVacatedAsOneUnit(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	friends, queueID := mustBackfillingGroup(t, st, cleaner, 3)
	for _, friend := range friends {
		goAway(t, st, ctx, friend.ID)
	}
	strangers := queueStrangers(t, st, cleaner, ctx, queueID, 3)

	// One hold between the three of them, not three absences tripping the
	// two-at-once rule against itself: every chair is still theirs.
	if rec := mustReconcileForming(t, st, ctx, queueID); rec.Fired {
		t.Fatal("the match fired with a group in it that nobody was watching")
	}
	for i, friend := range friends {
		if n := assignedSeatCount(t, st, ctx, friend.ID); n != 1 {
			t.Fatalf("friend %d lost their chair immediately (%d seats); a group gets the single hold a solo player gets", i, n)
		}
	}

	// Out of time. All three chairs go back to the pool together.
	backdateHold(t, st, ctx, queueID, HoldUnreachableFloor+time.Second)
	mustReconcileForming(t, st, ctx, queueID)
	for i, friend := range friends {
		if n := assignedSeatCount(t, st, ctx, friend.ID); n != 0 {
			t.Fatalf("friend %d kept a chair past the window (%d seats); the party is vacated as a unit", i, n)
		}
		if n := waitingRowCount(t, st, ctx, friend.ID); n != 1 {
			t.Fatalf("friend %d was dropped from the queue (%d waiting rows); vacating a chair is not ejecting the player", i, n)
		}
	}

	// And they are seated again when they come back — the thing the partly-released
	// party could never do.
	for _, friend := range friends {
		comeBack(t, st, ctx, friend.ID)
	}
	rec := mustReconcileForming(t, st, ctx, queueID)
	if !rec.Fired || rec.SessionID == nil {
		t.Fatalf("the group was never placed again after coming back, got %+v", rec)
	}
	assertMatchedTogether(t, st, ctx, *rec.SessionID, friends, strangers)
}
