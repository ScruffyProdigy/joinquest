package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/gameclient"
)

// setupTeamMode registers a 3v3: two sides of three, naming neither. Its one queue path is
// the empty string and its six seats are one bucket, which is the mode declaring its two
// sides equivalent — so a group of three takes a side rather than picking one, and
// matchmaking decides what that side ends up being called.
//
// This is the shape of the ticket's own examples: a group that needs opponents rather
// than more of itself, and that should never have to get a side number right to sit
// together.
func setupTeamMode(t *testing.T, st *Store, cleaner *TestCleaner) (*Game, *GameMode, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	slug := "table-team-" + uuid.NewString()
	manifest := &gameclient.Manifest{
		Modes: []gameclient.ModeManifest{{
			Key:          "teams",
			DisplayName:  "3v3",
			SeatTemplate: json.RawMessage(`{"Team":{"count":2,"Seat":{"count":3}}}`),
		}},
		Status:     gameclient.StatusResponse{Game: "Teams", Version: "1.0.0"},
		ETag:       `"teams"`,
		RawJSON:    []byte(`{"modes":[{"key":"teams"}]}`),
		SHA256Hash: uuid.NewString(),
	}
	result, err := st.RegisterGame(ctx, RegisterGameParams{
		Slug:       slug,
		IconURL:    "/games/default.svg",
		HeroURL:    "/games/default-hero.svg",
		APIBaseURL: "https://api.example.com/" + slug,
	}, manifest)
	if err != nil {
		t.Fatalf("RegisterGame: %v", err)
	}
	cleaner.TrackGame(result.Game.ID)

	modes, err := st.ListGameModesByGameID(ctx, result.Game.ID)
	if err != nil {
		t.Fatalf("ListGameModesByGameID: %v", err)
	}
	queues, err := st.ListModeQueuesByModeID(ctx, modes[0].ID)
	if err != nil {
		t.Fatalf("ListModeQueuesByModeID: %v", err)
	}
	return result.Game, &modes[0], queues[0].ID
}

// setupTwoSidedMode registers a 3v3 whose sides are named, so each seat carries its own
// queue path and a player can choose which side to sit on.
//
// Naming them is how a mode says the two sides are not interchangeable. setupTeamMode
// above says the opposite by leaving them anonymous, and its seats pool across both as a
// result — a group there takes a side rather than choosing one, which is what the tests
// above assert. Only a mode like this one can mean "two of us here, one of us over
// there", so only this one can be asked whether that survives matchmaking.
func setupTwoSidedMode(t *testing.T, st *Store, cleaner *TestCleaner) (*Game, *GameMode, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	slug := "table-sides-" + uuid.NewString()
	manifest := &gameclient.Manifest{
		Modes: []gameclient.ModeManifest{{
			Key:          "sides",
			DisplayName:  "Red vs Blue",
			SeatTemplate: json.RawMessage(`{"Red":{"count":3},"Blue":{"count":3}}`),
		}},
		Status:     gameclient.StatusResponse{Game: "Sides", Version: "1.0.0"},
		ETag:       `"sides"`,
		RawJSON:    []byte(`{"modes":[{"key":"sides"}]}`),
		SHA256Hash: uuid.NewString(),
	}
	result, err := st.RegisterGame(ctx, RegisterGameParams{
		Slug:       slug,
		IconURL:    "/games/default.svg",
		HeroURL:    "/games/default-hero.svg",
		APIBaseURL: "https://api.example.com/" + slug,
	}, manifest)
	if err != nil {
		t.Fatalf("RegisterGame: %v", err)
	}
	cleaner.TrackGame(result.Game.ID)

	modes, err := st.ListGameModesByGameID(ctx, result.Game.ID)
	if err != nil {
		t.Fatalf("ListGameModesByGameID: %v", err)
	}
	queues, err := st.ListModeQueuesByModeID(ctx, modes[0].ID)
	if err != nil {
		t.Fatalf("ListModeQueuesByModeID: %v", err)
	}
	return result.Game, &modes[0], queues[0].ID
}

func mustUser(t *testing.T, st *Store, cleaner *TestCleaner, tag string) *User {
	t.Helper()
	user, err := st.CreateUser(context.Background(), CreateUserParams{
		Email: tag + "-" + uuid.NewString() + "@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser %s: %v", tag, err)
	}
	cleaner.TrackUser(user.ID)
	return user
}

// mustGroupTable creates a room with a table and seats everyone in it, returning the
// table. `owner` is seated first and so becomes the king. `alsoInRoom` join the room
// without taking a seat, ready to claim one later.
func mustGroupTable(
	t *testing.T,
	st *Store,
	game *Game,
	mode *GameMode,
	owner *User,
	seating map[*User]string,
	alsoInRoom ...*User,
) *RoomTable {
	t.Helper()
	ctx := context.Background()
	room, err := st.CreateRoom(ctx, owner.ID)
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	table, err := st.CreateTable(ctx, room.ID, game.ID, mode.ID, owner.ID)
	if err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	if _, err := st.SitAtTable(ctx, table.ID, owner.ID, seating[owner]); err != nil {
		t.Fatalf("sit owner: %v", err)
	}
	join := func(user *User) {
		if err := st.addRoomMemberDirect(ctx, room.ID, user.ID); err != nil {
			t.Fatalf("addRoomMember: %v", err)
		}
	}
	for user, seatKey := range seating {
		if user == owner {
			continue
		}
		join(user)
		if _, err := st.SitAtTable(ctx, table.ID, user.ID, seatKey); err != nil {
			t.Fatalf("sit %s: %v", seatKey, err)
		}
	}
	for _, user := range alsoInRoom {
		join(user)
	}
	return table
}

// sideOf reports which side of the match a seat key belongs to: "Team-1" out of
// "Team-1-Seat-2", and "Red" out of "Red-2".
func sideOf(seatKey string) string {
	parts := strings.Split(seatKey, "-")
	if len(parts) >= 3 {
		return parts[0] + "-" + parts[1]
	}
	return parts[0]
}

// End-to-end for the ticket's headline case: two friends on the same side ask for the
// rest of the match, strangers fill the other seats, and everybody reaches one game.
func TestGroupBackfillSameSideFillsAndAllPlayersReachTheGame(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	game, mode, queueID := setupTeamMode(t, st, cleaner)
	alice := mustUser(t, st, cleaner, "same-side-a")
	bob := mustUser(t, st, cleaner, "same-side-b")

	table := mustGroupTable(t, st, game, mode, alice, map[*User]string{
		alice: "Team-1-Seat-1",
		bob:   "Team-1-Seat-2",
	})

	// Bob, not the king, makes the request — the point of the ticket.
	if _, err := st.StartTableBackfill(ctx, table.ID, bob.ID, queueID); err != nil {
		t.Fatalf("StartTableBackfill by non-king: %v", err)
	}
	if rec := mustReconcileForming(t, st, ctx, queueID); rec.Fired {
		t.Fatal("expected the group to wait: four seats are still empty")
	}

	strangers := make([]*User, 4)
	for i := range strangers {
		strangers[i] = mustUser(t, st, cleaner, "same-side-stranger")
		if _, err := st.JoinModeQueue(ctx, queueID, strangers[i].ID, "", nil); err != nil {
			t.Fatalf("stranger %d join: %v", i, err)
		}
	}

	rec := mustReconcileForming(t, st, ctx, queueID)
	if !rec.Fired || rec.SessionID == nil {
		t.Fatalf("expected the filled match to fire, got %+v", rec)
	}

	assignments, err := st.ListSessionSeatAssignments(ctx, *rec.SessionID)
	if err != nil {
		t.Fatalf("ListSessionSeatAssignments: %v", err)
	}
	if len(assignments) != 6 {
		t.Fatalf("expected 6 players in the match, got %d", len(assignments))
	}

	seatByUser := map[uuid.UUID]string{}
	for _, a := range assignments {
		seatByUser[a.UserID] = a.SeatKey
	}
	// Every player reaches the game, the two friends included — "you'll be taken in
	// automatically", asserted rather than assumed.
	for _, user := range append([]*User{alice, bob}, strangers...) {
		if _, ok := seatByUser[user.ID]; !ok {
			t.Fatalf("player %s did not reach the match", user.ID)
		}
	}
	if sideOf(seatByUser[alice.ID]) != sideOf(seatByUser[bob.ID]) {
		t.Fatalf("friends queued on one side were split: %s vs %s",
			seatByUser[alice.ID], seatByUser[bob.ID])
	}
}

// The split case, through the whole path a group actually walks. Two friends on one side
// and one on the other must come out of matchmaking still split that way. This asserts
// behaviour that already works, so that a later change to the control cannot quietly
// break it.
//
// It uses a mode whose sides are named, because a deliberate split is only a thing a
// player can mean when the sides differ. Where they do not, the group simply takes a side
// together and the tests above cover it.
func TestGroupBackfillSplitGroupKeepsItsSides(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	game, mode, queueID := setupTwoSidedMode(t, st, cleaner)
	alice := mustUser(t, st, cleaner, "split-a")
	bob := mustUser(t, st, cleaner, "split-b")
	cara := mustUser(t, st, cleaner, "split-c")

	table := mustGroupTable(t, st, game, mode, alice, map[*User]string{
		alice: "Red-1",
		bob:   "Red-2",
		cara:  "Blue-1",
	})
	// The split is the players' own, and the seat rows have to say so before
	// matchmaking can be asked to preserve anything.
	seated, err := st.ListTableSeats(ctx, table.ID)
	if err != nil {
		t.Fatalf("ListTableSeats: %v", err)
	}
	for _, seat := range seated {
		want := map[uuid.UUID]string{alice.ID: "Red-1", bob.ID: "Red-2", cara.ID: "Blue-1"}[seat.UserID]
		if seat.SeatKey != want {
			t.Fatalf("player %s sat in %s, wanted %s", seat.UserID, seat.SeatKey, want)
		}
	}

	if _, err := st.StartTableBackfill(ctx, table.ID, cara.ID, queueID); err != nil {
		t.Fatalf("StartTableBackfill: %v", err)
	}
	// One more for Red, two more for Blue — the gaps the split itself implies.
	for _, side := range []string{"Red", "Blue", "Blue"} {
		stranger := mustUser(t, st, cleaner, "split-stranger")
		if _, err := st.JoinModeQueue(ctx, queueID, stranger.ID, side, nil); err != nil {
			t.Fatalf("stranger join as %s: %v", side, err)
		}
	}

	rec := mustReconcileForming(t, st, ctx, queueID)
	if !rec.Fired || rec.SessionID == nil {
		t.Fatalf("expected the filled match to fire, got %+v", rec)
	}

	assignments, err := st.ListSessionSeatAssignments(ctx, *rec.SessionID)
	if err != nil {
		t.Fatalf("ListSessionSeatAssignments: %v", err)
	}
	if len(assignments) != 6 {
		t.Fatalf("expected 6 players in the match, got %d", len(assignments))
	}
	seatByUser := map[uuid.UUID]string{}
	for _, a := range assignments {
		seatByUser[a.UserID] = a.SeatKey
	}
	if sideOf(seatByUser[alice.ID]) != sideOf(seatByUser[bob.ID]) {
		t.Fatalf("the pair was split apart: %s vs %s", seatByUser[alice.ID], seatByUser[bob.ID])
	}
	if sideOf(seatByUser[cara.ID]) == sideOf(seatByUser[alice.ID]) {
		t.Fatalf("the lone player was folded into the pair's side: all on %s",
			sideOf(seatByUser[alice.ID]))
	}
}

// The king gate is gone, but the table is still the boundary: a room member watching
// from outside it has no seat to carry into the queue.
func TestStartTableBackfillRejectsUnseatedRoomMember(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	game, mode, queueID := setupTeamMode(t, st, cleaner)
	alice := mustUser(t, st, cleaner, "unseated-king")
	watcher := mustUser(t, st, cleaner, "unseated-watcher")

	table := mustGroupTable(t, st, game, mode, alice, map[*User]string{alice: "Team-1-Seat-1"})
	room, err := st.GetRoomTableByID(ctx, table.ID)
	if err != nil {
		t.Fatalf("GetRoomTableByID: %v", err)
	}
	if err := st.addRoomMemberDirect(ctx, room.RoomID, watcher.ID); err != nil {
		t.Fatalf("addRoomMember: %v", err)
	}

	if _, err := st.StartTableBackfill(ctx, table.ID, watcher.ID, queueID); err == nil {
		t.Fatal("expected a room member with no seat to be refused")
	}
}

// Cancelling takes the whole table back out, not just whoever pressed it.
func TestCancelTableBackfillTakesTheWholeGroupOutOfTheQueue(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	game, mode, queueID := setupTeamMode(t, st, cleaner)
	alice := mustUser(t, st, cleaner, "cancel-a")
	bob := mustUser(t, st, cleaner, "cancel-b")

	table := mustGroupTable(t, st, game, mode, alice, map[*User]string{
		alice: "Team-1-Seat-1",
		bob:   "Team-1-Seat-2",
	})

	if _, err := st.StartTableBackfill(ctx, table.ID, alice.ID, queueID); err != nil {
		t.Fatalf("StartTableBackfill: %v", err)
	}
	mustReconcileForming(t, st, ctx, queueID)

	// Bob cancels what Alice started: a group request has no owner.
	result, err := st.CancelTableBackfill(ctx, table.ID, bob.ID)
	if err != nil {
		t.Fatalf("CancelTableBackfill: %v", err)
	}
	if !result.Cancelled {
		t.Fatal("expected the cancel to report that it withdrew a live request")
	}

	for _, user := range []*User{alice, bob} {
		queueIDForUser, waiting, err := st.WaitingModeQueueIDForUser(ctx, user.ID)
		if err != nil {
			t.Fatalf("WaitingModeQueueIDForUser: %v", err)
		}
		if waiting {
			t.Fatalf("player %s is still waiting on queue %s after the cancel", user.ID, queueIDForUser)
		}
	}
	active, err := st.TableBackfillActive(ctx, table.ID)
	if err != nil {
		t.Fatalf("TableBackfillActive: %v", err)
	}
	if active {
		t.Fatal("expected backfill to read as inactive after the cancel")
	}

	// The seats are the table's again, so asking a second time is allowed.
	if _, err := st.StartTableBackfill(ctx, table.ID, bob.ID, queueID); err != nil {
		t.Fatalf("re-request after cancel: %v", err)
	}
}

// Cancelling when nothing is waiting is not an error — first writer wins, and the second
// cancel finds the state it asked for.
func TestCancelTableBackfillWithNothingWaiting(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	game, mode, _ := setupTeamMode(t, st, cleaner)
	alice := mustUser(t, st, cleaner, "cancel-noop")
	table := mustGroupTable(t, st, game, mode, alice, map[*User]string{alice: "Team-1-Seat-1"})

	result, err := st.CancelTableBackfill(ctx, table.ID, alice.ID)
	if err != nil {
		t.Fatalf("CancelTableBackfill: %v", err)
	}
	if result.Cancelled {
		t.Fatal("expected false when there was no live request to withdraw")
	}
}

// A full table starts itself; a table merely past the mode's minimum does not, because
// waiting for the friend still on their way is the whole reason a group screen exists.
func TestAutoStartTableOnlyWhenEverySeatIsTaken(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	game, mode, _ := setupTeamMode(t, st, cleaner)
	users := make([]*User, 6)
	seating := map[*User]string{}
	seatKeys := []string{
		"Team-1-Seat-1", "Team-1-Seat-2", "Team-1-Seat-3",
		"Team-2-Seat-1", "Team-2-Seat-2", "Team-2-Seat-3",
	}
	for i := range users {
		users[i] = mustUser(t, st, cleaner, "autostart")
	}
	for i := 0; i < 5; i++ {
		seating[users[i]] = seatKeys[i]
	}
	table := mustGroupTable(t, st, game, mode, users[0], seating, users[5])

	result, err := st.AutoStartTable(ctx, table.ID)
	if err != nil {
		t.Fatalf("AutoStartTable with one seat open: %v", err)
	}
	if result != nil {
		t.Fatal("expected no start while a seat is still open")
	}

	if _, err := st.SitAtTable(ctx, table.ID, users[5].ID, seatKeys[5]); err != nil {
		t.Fatalf("sit last: %v", err)
	}
	result, err = st.AutoStartTable(ctx, table.ID)
	if err != nil {
		t.Fatalf("AutoStartTable when full: %v", err)
	}
	if result == nil {
		t.Fatal("expected a full table to start itself")
	}
	if len(result.NotifyUserIDs) != 6 {
		t.Fatalf("expected all 6 players told the game started, got %d", len(result.NotifyUserIDs))
	}
}

// A table with a live request is the queue's to fill: firing a private session under it
// would strand the strangers being formed around it.
func TestAutoStartTableSkippedWhileBackfillIsLive(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	game, mode, queueID := setupTeamMode(t, st, cleaner)
	seatKeys := []string{
		"Team-1-Seat-1", "Team-1-Seat-2", "Team-1-Seat-3",
		"Team-2-Seat-1", "Team-2-Seat-2", "Team-2-Seat-3",
	}
	users := make([]*User, 6)
	seating := map[*User]string{}
	for i := range users {
		users[i] = mustUser(t, st, cleaner, "autostart-backfill")
		if i < 5 {
			seating[users[i]] = seatKeys[i]
		}
	}
	table := mustGroupTable(t, st, game, mode, users[0], seating, users[5])

	if _, err := st.StartTableBackfill(ctx, table.ID, users[0].ID, queueID); err != nil {
		t.Fatalf("StartTableBackfill: %v", err)
	}
	mustReconcileForming(t, st, ctx, queueID)

	if _, err := st.SitAtTable(ctx, table.ID, users[5].ID, seatKeys[5]); err != nil {
		t.Fatalf("sit last: %v", err)
	}
	result, err := st.AutoStartTable(ctx, table.ID)
	if err != nil {
		t.Fatalf("AutoStartTable: %v", err)
	}
	if result != nil {
		t.Fatal("expected no auto-start while the table is waiting on matchmaking")
	}
}

// Three friends group up, land together, and matchmaking finds them three opponents —
// the ticket's headline case, and the reason a symmetric template pools its seats.
//
// Nobody chose a side. `{"Team":{"count":2,"Seat":{"count":3}}}` names neither, which is
// the game saying the two are equivalent, so the claim resolution gathers the group onto
// whichever side has room and the solver decides what that side is called. A player who
// had to get "Team 1" or "Team 2" right before their friends could sit with them would be
// answering a question the mode never asked.
func TestSymmetricSidesSeatAGroupTogetherWhateverTheyClick(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	game, mode, _ := setupTeamMode(t, st, cleaner)
	alice := mustUser(t, st, cleaner, "symmetric-a")
	bob := mustUser(t, st, cleaner, "symmetric-b")
	cara := mustUser(t, st, cleaner, "symmetric-c")

	// Deliberately scattered across both sides. The mode declares them interchangeable,
	// so what the group gets is one side, not the three seats they happened to tap.
	table := mustGroupTable(t, st, game, mode, alice, map[*User]string{
		alice: "Team-1-Seat-1",
		bob:   "Team-2-Seat-2",
		cara:  "Team-2-Seat-1",
	})

	seated, err := st.ListTableSeats(ctx, table.ID)
	if err != nil {
		t.Fatalf("ListTableSeats: %v", err)
	}
	if len(seated) != 3 {
		t.Fatalf("expected 3 seated, got %d", len(seated))
	}
	side := sideOf(seated[0].SeatKey)
	for _, seat := range seated {
		if sideOf(seat.SeatKey) != side {
			t.Fatalf("the group was scattered across sides: %s and %s",
				side, sideOf(seat.SeatKey))
		}
	}
}

// And two such groups meet as opponents. Each table queues as one branch node, and
// `placeBranchSiblings` permutes them into distinct side instances — which is what lets a
// mode leave its sides unnamed and still never put two groups of three on top of each
// other. This is the case the ticket's non-goals protect, asserted so a later change to
// the control cannot quietly break it.
func TestTwoGroupsOfThreeMeetOnOppositeSides(t *testing.T) {
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

	home, homeTable := newGroup("meet-home")
	away, awayTable := newGroup("meet-away")

	// Either group may ask, and neither is the king of the other's table.
	if _, err := st.StartTableBackfill(ctx, homeTable.ID, home[2].ID, queueID); err != nil {
		t.Fatalf("home group asks: %v", err)
	}
	if rec := mustReconcileForming(t, st, ctx, queueID); rec.Fired {
		t.Fatal("expected the first group to wait for opponents")
	}
	if _, err := st.StartTableBackfill(ctx, awayTable.ID, away[1].ID, queueID); err != nil {
		t.Fatalf("away group asks: %v", err)
	}

	rec := mustReconcileForming(t, st, ctx, queueID)
	if !rec.Fired || rec.SessionID == nil {
		t.Fatalf("expected the two groups to complete a match, got %+v", rec)
	}
	assignments, err := st.ListSessionSeatAssignments(ctx, *rec.SessionID)
	if err != nil {
		t.Fatalf("ListSessionSeatAssignments: %v", err)
	}
	if len(assignments) != 6 {
		t.Fatalf("expected 6 players in the match, got %d", len(assignments))
	}
	sideByUser := map[uuid.UUID]string{}
	for _, a := range assignments {
		sideByUser[a.UserID] = sideOf(a.SeatKey)
	}
	homeSide := sideByUser[home[0].ID]
	for _, user := range home {
		if sideByUser[user.ID] != homeSide {
			t.Fatalf("the home group was split across sides: %s and %s",
				homeSide, sideByUser[user.ID])
		}
	}
	for _, user := range away {
		if sideByUser[user.ID] == homeSide {
			t.Fatalf("the two groups were poured onto the same side (%s)", homeSide)
		}
	}
}
