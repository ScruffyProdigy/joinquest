package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// twoPartyMatch is the shape a team mode ordinarily takes: two groups who each queued from
// their own room, matched against each other (JQ-291). It is not an exotic setup — for a
// 3v3, a group of three creating a room and pressing "Find more players" against another
// group of three is the mainline path.
type twoPartyMatch struct {
	SessionID uuid.UUID
	Mode      *GameMode
	RoomA     *Room
	RoomB     *Room
	PartyA    []uuid.UUID
	PartyB    []uuid.UUID
}

// seedTwoPartyMatch builds a 3v3 from two rooms of three, through the queue — the path that
// drops the arrival table on the floor unless formingReturnContextTx stamps the room it came
// from.
//
// The two groups take opposite sides, which is not only flavour: a party carries its local
// seat keys into matchmaking, so two groups both queuing for the same three seats cannot be
// placed on one map at all. Giving a group its own side is the shape that forms today, and
// how a match forms is explicitly not this ticket's business.
func seedTwoPartyMatch(t *testing.T, st *Store, ctx context.Context, cleaner *TestCleaner) twoPartyMatch {
	t.Helper()

	game, mode := setupRejoinMode(t, st, cleaner, "teams-"+uuid.NewString()[:8],
		`{"Red":{"count":3},"Blue":{"count":3}}`, "")
	queues, err := st.ListModeQueuesByModeID(ctx, mode.ID)
	if err != nil {
		t.Fatalf("ListModeQueuesByModeID: %v", err)
	}
	queueID := queues[0].ID

	roomA, partyA := seedQueuingParty(t, st, ctx, cleaner, game, mode, queueID, "a", "Red")
	if rec := mustReconcileForming(t, st, ctx, queueID); rec.Fired {
		t.Fatalf("three of six seats should not fire a match")
	}
	roomB, partyB := seedQueuingParty(t, st, ctx, cleaner, game, mode, queueID, "b", "Blue")
	rec := mustReconcileForming(t, st, ctx, queueID)
	if !rec.Fired || rec.SessionID == nil {
		t.Fatalf("six seated players should fire a match, got %+v", rec)
	}
	if err := st.CompleteSession(ctx, *rec.SessionID, time.Now()); err != nil {
		t.Fatalf("CompleteSession: %v", err)
	}

	return twoPartyMatch{
		SessionID: *rec.SessionID,
		Mode:      mode,
		RoomA:     roomA,
		RoomB:     roomB,
		PartyA:    partyA,
		PartyB:    partyB,
	}
}

// seedQueuingParty sits three players on one side of a room's table and presses "Find more
// players".
func seedQueuingParty(
	t *testing.T,
	st *Store,
	ctx context.Context,
	cleaner *TestCleaner,
	game *Game,
	mode *GameMode,
	queueID uuid.UUID,
	label string,
	side string,
) (*Room, []uuid.UUID) {
	t.Helper()

	king := newRejoinUser(t, st, ctx, cleaner, "party-"+label+"-king")
	room, err := st.CreateRoom(ctx, king.ID)
	if err != nil {
		t.Fatalf("CreateRoom %s: %v", label, err)
	}
	table, err := st.CreateTable(ctx, room.ID, game.ID, mode.ID, king.ID)
	if err != nil {
		t.Fatalf("CreateTable %s: %v", label, err)
	}
	sideSeats := seatKeysOnSide(t, st, ctx, mode.ID, side)
	if len(sideSeats) < 3 {
		t.Fatalf("side %q has %d seats, need 3", side, len(sideSeats))
	}

	members := []uuid.UUID{king.ID}
	if _, err := st.SitAtTable(ctx, table.ID, king.ID, sideSeats[0]); err != nil {
		t.Fatalf("king sit %s: %v", label, err)
	}
	for i := 1; i < 3; i++ {
		mate := newRejoinUser(t, st, ctx, cleaner, "party-"+label+"-mate")
		if _, err := st.JoinRoom(ctx, mate.ID, room.InviteCode); err != nil {
			t.Fatalf("JoinRoom %s: %v", label, err)
		}
		if _, err := st.SitAtTable(ctx, table.ID, mate.ID, sideSeats[i]); err != nil {
			t.Fatalf("mate sit %s: %v", label, err)
		}
		members = append(members, mate.ID)
	}

	if _, err := st.StartTableBackfill(ctx, table.ID, king.ID, queueID); err != nil {
		t.Fatalf("StartTableBackfill %s: %v", label, err)
	}
	return room, members
}

// seatKeysOnSide returns the seat keys belonging to one side of a team mode.
func seatKeysOnSide(t *testing.T, st *Store, ctx context.Context, modeID uuid.UUID, side string) []string {
	t.Helper()
	seats, err := st.ListGameModeSeats(ctx, modeID)
	if err != nil {
		t.Fatalf("ListGameModeSeats: %v", err)
	}
	var keys []string
	for _, seat := range seats {
		if seat.QueuePath != nil && *seat.QueuePath == side {
			keys = append(keys, seat.SeatKey)
		}
	}
	return keys
}

// partyOf reads the arrival party of one player, the way every partition in the system does.
func partyOf(t *testing.T, st *Store, ctx context.Context, sessionID, userID uuid.UUID) ArrivalParty {
	t.Helper()
	parties, err := st.GetArrivalParties(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetArrivalParties: %v", err)
	}
	return parties[userID]
}

// TestArrivalPartyIsTheRoomYouQueuedFrom is the fact everything else here rests on: a group
// that reaches a match through matchmaking still carries the room it came from.
//
// Without it, forming_match_assignments.table_id is the only record that the three of them
// arrived together, and that belongs to the forming map, which is finished with the moment
// the match fires. Every player read as a solo catalog joiner and the whole partition below
// collapsed into six parties of one.
func TestArrivalPartyIsTheRoomYouQueuedFrom(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	match := seedTwoPartyMatch(t, st, ctx, cleaner)

	for _, userID := range match.PartyA {
		if got := partyOf(t, st, ctx, match.SessionID, userID); got != ArrivalPartyOfRoom(match.RoomA.ID) {
			t.Errorf("party A member %s arrived as %q, want room A", userID, got)
		}
	}
	for _, userID := range match.PartyB {
		if got := partyOf(t, st, ctx, match.SessionID, userID); got != ArrivalPartyOfRoom(match.RoomB.ID) {
			t.Errorf("party B member %s arrived as %q, want room B", userID, got)
		}
	}
	if SameArrivalParty(partyOf(t, st, ctx, match.SessionID, match.PartyA[0]), partyOf(t, st, ctx, match.SessionID, match.PartyB[0])) {
		t.Error("the two groups read as one arrival party")
	}
}

// TestTwoArrivalPartiesGetTwoRegroupTables is the bug this ticket is named for: both groups
// used to converge on whichever table the first claimant happened to build, so a 3v3 ended
// with six strangers in one room that neither group agreed to.
func TestTwoArrivalPartiesGetTwoRegroupTables(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	match := seedTwoPartyMatch(t, st, ctx, cleaner)

	tableA, roomA, err := st.ClaimRegroupTable(ctx, match.SessionID, match.PartyA[0])
	if err != nil {
		t.Fatalf("claim A: %v", err)
	}
	tableB, roomB, err := st.ClaimRegroupTable(ctx, match.SessionID, match.PartyB[0])
	if err != nil {
		t.Fatalf("claim B: %v", err)
	}
	if tableA.ID == tableB.ID {
		t.Fatalf("both groups landed on table %s", tableA.ID)
	}
	if roomA.ID != match.RoomA.ID {
		t.Errorf("group A regrouped in room %s, want the room it queued from (%s)", roomA.ID, match.RoomA.ID)
	}
	if roomB.ID != match.RoomB.ID {
		t.Errorf("group B regrouped in room %s, want the room it queued from (%s)", roomB.ID, match.RoomB.ID)
	}

	// The rest of each group joins its own table, not the other one.
	for _, userID := range match.PartyA[1:] {
		table, _, err := st.ClaimRegroupTable(ctx, match.SessionID, userID)
		if err != nil {
			t.Fatalf("claim A member %s: %v", userID, err)
		}
		if table.ID != tableA.ID {
			t.Errorf("group A member %s landed on %s, want %s", userID, table.ID, tableA.ID)
		}
	}

	// The table comes back empty, because this mode has sides to rotate and a group
	// returned to choose them again (JQ-232) — so what matters is who *can* sit down.
	seated, err := st.ListTableSeats(ctx, tableA.ID)
	if err != nil {
		t.Fatalf("ListTableSeats: %v", err)
	}
	if len(seated) != 0 {
		t.Fatalf("group A's regroup table seats %d players, want an empty table to re-pick at", len(seated))
	}

	// Group A takes its own side again, and the other three seats are genuinely open:
	// backfill can offer them to a fresh opposing side rather than the group they just
	// played, who are not sitting in them.
	red := seatKeysOnSide(t, st, ctx, match.Mode.ID, "Red")
	for i, userID := range match.PartyA {
		if _, err := st.SitAtTable(ctx, tableA.ID, userID, red[i]); err != nil {
			t.Fatalf("reseat %s: %v", userID, err)
		}
	}
	seated, err = st.ListTableSeats(ctx, tableA.ID)
	if err != nil {
		t.Fatalf("ListTableSeats: %v", err)
	}
	for _, seat := range seated {
		for _, opponent := range match.PartyB {
			if seat.UserID == opponent {
				t.Errorf("opposing player %s is sitting at group A's regroup table", opponent)
			}
		}
	}
	modeSeats, err := st.ListGameModeSeats(ctx, match.Mode.ID)
	if err != nil {
		t.Fatalf("ListGameModeSeats: %v", err)
	}
	if open := len(modeSeats) - len(seated); open != 3 {
		t.Errorf("open seats = %d, want 3 for backfill", open)
	}
}

// TestRegroupInviteCodeIsPerParty is the same split one layer up. The invite code is a link
// people follow off the results screen, so answering it session-wide walks the group that
// lost into the winners' room.
func TestRegroupInviteCodeIsPerParty(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	match := seedTwoPartyMatch(t, st, ctx, cleaner)

	tableA, _, err := st.ClaimRegroupTable(ctx, match.SessionID, match.PartyA[0])
	if err != nil {
		t.Fatalf("claim A: %v", err)
	}

	// A's group-mate is pointed at the table their own group claimed.
	got, err := st.GetRegroupTableIDForUser(ctx, match.SessionID, match.PartyA[1])
	if err != nil {
		t.Fatalf("GetRegroupTableIDForUser A: %v", err)
	}
	if got == nil || *got != tableA.ID {
		t.Errorf("group A member sees %v, want their group's table %s", got, tableA.ID)
	}

	// The opposing group is pointed nowhere, because their own group has claimed nothing.
	got, err = st.GetRegroupTableIDForUser(ctx, match.SessionID, match.PartyB[0])
	if err != nil {
		t.Fatalf("GetRegroupTableIDForUser B: %v", err)
	}
	if got != nil {
		t.Errorf("opposing group sees table %s, want none of their own", *got)
	}
}

// TestArrivalPartySurvivesATableResetBetweenRounds pins where the partition is read from.
// The mutable candidates all drift: room_tables.session_id is cleared by the post-match
// reset, and game_sessions.regroup_table_id is stamped by whichever player claims first.
// return_context is written once when the session starts and never rewritten, which is the
// only reason a group still reads as a group after they have regrouped and reseated.
func TestArrivalPartySurvivesATableResetBetweenRounds(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	match := seedTwoPartyMatch(t, st, ctx, cleaner)
	before := partyOf(t, st, ctx, match.SessionID, match.PartyA[0])

	for _, userID := range match.PartyA {
		if _, _, err := st.ClaimRegroupTable(ctx, match.SessionID, userID); err != nil {
			t.Fatalf("claim %s: %v", userID, err)
		}
	}

	if after := partyOf(t, st, ctx, match.SessionID, match.PartyA[0]); after != before {
		t.Errorf("arrival party moved from %q to %q after regrouping", before, after)
	}
	if before != ArrivalPartyOfRoom(match.RoomA.ID) {
		t.Errorf("arrival party = %q, want room A", before)
	}
}

// TestSoloArrivalIsAPartyOfOne covers the other end of the rule. A player who joined the
// catalog queue alone arrived with nobody, so their party matches no one — including the
// other solos in the same match, who are parties of one themselves.
//
// Two empty parties comparing equal is the failure mode worth pinning: it would silently
// rebuild the merged room this ticket removed, with every stranger in the match as its
// members.
func TestSoloArrivalIsAPartyOfOne(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	sessionID, userA, userB := seedMatchedSession(t, st, ctx, cleaner)
	if err := st.CompleteSession(ctx, sessionID, time.Now()); err != nil {
		t.Fatalf("CompleteSession: %v", err)
	}

	partyA := partyOf(t, st, ctx, sessionID, userA)
	partyB := partyOf(t, st, ctx, sessionID, userB)
	if partyA != NoArrivalParty || partyB != NoArrivalParty {
		t.Fatalf("catalog joiners have parties %q and %q, want none", partyA, partyB)
	}
	if SameArrivalParty(partyA, partyB) {
		t.Error("two players who each arrived alone read as one party")
	}

	tableA, _, err := st.ClaimRegroupTable(ctx, sessionID, userA)
	if err != nil {
		t.Fatalf("claim A: %v", err)
	}
	tableB, _, err := st.ClaimRegroupTable(ctx, sessionID, userB)
	if err != nil {
		t.Fatalf("claim B: %v", err)
	}
	if tableA.ID == tableB.ID {
		t.Errorf("two solo players landed on the same table %s", tableA.ID)
	}
}

// TestBackfilledStrangerIsNotInTheGroupsParty is the case that needs no 3v3 at all, and the
// more common one: a group's table topped up by strangers through backfill. They queued
// alone, so they are a different arrival party, and the group's regroup card is not where
// they belong (JQ-291) — which is most of what JQ-184's expiry was written to paper over.
func TestBackfilledStrangerIsNotInTheGroupsParty(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	game, mode := setupRejoinMode(t, st, cleaner, "trio-"+uuid.NewString()[:8], `{"count":3}`, "")
	queues, err := st.ListModeQueuesByModeID(ctx, mode.ID)
	if err != nil {
		t.Fatalf("ListModeQueuesByModeID: %v", err)
	}
	queueID := queues[0].ID

	// Two friends at a table, one seat short, plus a stranger from the catalog queue.
	king := newRejoinUser(t, st, ctx, cleaner, "trio-king")
	friend := newRejoinUser(t, st, ctx, cleaner, "trio-friend")
	stranger := newRejoinUser(t, st, ctx, cleaner, "trio-stranger")

	room, err := st.CreateRoom(ctx, king.ID)
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	if _, err := st.JoinRoom(ctx, friend.ID, room.InviteCode); err != nil {
		t.Fatalf("JoinRoom friend: %v", err)
	}
	table, err := st.CreateTable(ctx, room.ID, game.ID, mode.ID, king.ID)
	if err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	seats, err := st.ListGameModeSeats(ctx, mode.ID)
	if err != nil {
		t.Fatalf("ListGameModeSeats: %v", err)
	}
	if _, err := st.SitAtTable(ctx, table.ID, king.ID, seats[0].SeatKey); err != nil {
		t.Fatalf("king sit: %v", err)
	}
	if _, err := st.SitAtTable(ctx, table.ID, friend.ID, seats[1].SeatKey); err != nil {
		t.Fatalf("friend sit: %v", err)
	}

	if _, err := st.StartTableBackfill(ctx, table.ID, king.ID, queueID); err != nil {
		t.Fatalf("StartTableBackfill: %v", err)
	}
	mustReconcileForming(t, st, ctx, queueID)
	if _, err := st.JoinModeQueue(ctx, queueID, stranger.ID, "", nil); err != nil {
		t.Fatalf("stranger join: %v", err)
	}
	rec := mustReconcileForming(t, st, ctx, queueID)
	if !rec.Fired || rec.SessionID == nil {
		t.Fatalf("expected the stranger to complete the table, got %+v", rec)
	}
	sessionID := *rec.SessionID
	if err := st.CompleteSession(ctx, sessionID, time.Now()); err != nil {
		t.Fatalf("CompleteSession: %v", err)
	}

	if !SameArrivalParty(partyOf(t, st, ctx, sessionID, king.ID), partyOf(t, st, ctx, sessionID, friend.ID)) {
		t.Error("the two friends do not read as one party")
	}
	if got := partyOf(t, st, ctx, sessionID, stranger.ID); got != NoArrivalParty {
		t.Errorf("backfilled stranger arrived as %q, want no party", got)
	}
	if SameArrivalParty(partyOf(t, st, ctx, sessionID, king.ID), partyOf(t, st, ctx, sessionID, stranger.ID)) {
		t.Error("the backfilled stranger reads as part of the group's party")
	}

	// And the stranger does not follow the friends back into their room.
	groupTable, groupRoom, err := st.ClaimRegroupTable(ctx, sessionID, king.ID)
	if err != nil {
		t.Fatalf("claim king: %v", err)
	}
	if groupRoom.ID != room.ID {
		t.Errorf("the group regrouped in room %s, want their own %s", groupRoom.ID, room.ID)
	}
	strangerTable, _, err := st.ClaimRegroupTable(ctx, sessionID, stranger.ID)
	if err != nil {
		t.Fatalf("claim stranger: %v", err)
	}
	if strangerTable.ID == groupTable.ID {
		t.Errorf("the backfilled stranger landed back at the group's table %s", groupTable.ID)
	}
}
