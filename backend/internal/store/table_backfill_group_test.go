package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/gameclient"
)

// setupModeWithTemplate registers a one-mode game with the seat template given, so a test
// can name the shape it cares about instead of carrying a fixture per shape.
func setupModeWithTemplate(
	t *testing.T,
	st *Store,
	cleaner *TestCleaner,
	tag, template string,
) (*Game, *GameMode, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	slug := "table-" + tag + "-" + uuid.NewString()
	result, err := st.RegisterGame(ctx, RegisterGameParams{
		Slug:       slug,
		IconURL:    "/games/default.svg",
		HeroURL:    "/games/default-hero.svg",
		APIBaseURL: "https://api.example.com/" + slug,
	}, &gameclient.Manifest{
		Modes: []gameclient.ModeManifest{{
			Key:          tag,
			DisplayName:  tag,
			SeatTemplate: json.RawMessage(template),
		}},
		Status:     gameclient.StatusResponse{Game: tag, Version: "1.0.0"},
		ETag:       `"` + tag + `"`,
		RawJSON:    []byte(`{"modes":[{"key":"` + tag + `"}]}`),
		SHA256Hash: uuid.NewString(),
	})
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

// setupTeamMode registers a 3v3 naming neither side. Its one queue path is the empty
// string and its six seats are one bucket, which is the mode declaring its two sides
// equivalent — so a group of three takes a side rather than picking one, and matchmaking
// decides what that side ends up being called.
func setupTeamMode(t *testing.T, st *Store, cleaner *TestCleaner) (*Game, *GameMode, uuid.UUID) {
	t.Helper()
	return setupModeWithTemplate(t, st, cleaner, "teams", `{"Team":{"count":2,"Seat":{"count":3}}}`)
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

// One mechanism, whatever the mode looks like.
//
// Three friends wanting opponents in a 3v3 and three friends wanting a game in an
// eight-player free-for-all go through the identical process: they sit at a table, the
// king asks the lobby for the rest of the match, strangers are queued alongside them, and
// matchmaking seats everybody. Nothing branches on the shape of the mode — the seat
// template is an input to the same machinery, not a fork in it — so the shapes belong in
// one table rather than in four tests each implying a different story.
//
// What varies per shape is only what "the rest" means and where it lands, which is what
// each case's arrangement check states.
func TestBackfillFillsTheTableWhateverShapeTheModeIs(t *testing.T) {
	cases := []struct {
		name string
		// template is the mode's seat template; groupSeats are the seats the friends
		// claim (the king first); strangerPaths is one queue path per stranger needed
		// to complete the match.
		template      string
		groupSeats    []string
		strangerPaths []string
		matchSize     int
		// arrangement asserts where everyone ended up, given each player's side.
		arrangement func(t *testing.T, sideOfGroup []string, sideOfStrangers []string)
	}{
		{
			name:          "eight-player free-for-all, three friends and five strangers",
			template:      `{"count":8}`,
			groupSeats:    []string{"1", "2", "3"},
			strangerPaths: []string{"", "", "", "", ""},
			matchSize:     8,
			arrangement: func(t *testing.T, group, strangers []string) {
				// No sides to land on. Everyone being in the match is the whole claim,
				// and the caller has already checked that.
			},
		},
		{
			name:          "three versus three, sides unnamed",
			template:      `{"Team":{"count":2,"Seat":{"count":3}}}`,
			groupSeats:    []string{"Team-1-Seat-1", "Team-1-Seat-2", "Team-1-Seat-3"},
			strangerPaths: []string{"", "", ""},
			matchSize:     6,
			arrangement:   groupTogetherAndStrangersOpposite,
		},
		{
			name:          "three versus three, sides named",
			template:      `{"Red":{"count":3},"Blue":{"count":3}}`,
			groupSeats:    []string{"Red-1", "Red-2", "Red-3"},
			strangerPaths: []string{"Blue", "Blue", "Blue"},
			matchSize:     6,
			arrangement:   groupTogetherAndStrangersOpposite,
		},
		{
			name:          "three versus three, the group itself split across named sides",
			template:      `{"Red":{"count":3},"Blue":{"count":3}}`,
			groupSeats:    []string{"Red-1", "Red-2", "Blue-1"},
			strangerPaths: []string{"Red", "Blue", "Blue"},
			matchSize:     6,
			arrangement: func(t *testing.T, group, strangers []string) {
				t.Helper()
				if group[0] != group[1] {
					t.Fatalf("the pair was split apart: %s vs %s", group[0], group[1])
				}
				if group[2] == group[0] {
					t.Fatalf("the lone player was folded into the pair's side: all on %s", group[0])
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := openTestStore(t)
			cleaner := st.NewTestCleaner(t)
			ctx := context.Background()

			game, mode, queueID := setupModeWithTemplate(t, st, cleaner, "shape", tc.template)

			friends := make([]*User, len(tc.groupSeats))
			seating := map[*User]string{}
			for i := range friends {
				friends[i] = mustUser(t, st, cleaner, "shape-friend")
				seating[friends[i]] = tc.groupSeats[i]
			}
			table := mustGroupTable(t, st, game, mode, friends[0], seating)

			if _, err := st.StartTableBackfill(ctx, table.ID, friends[0].ID, queueID); err != nil {
				t.Fatalf("StartTableBackfill: %v", err)
			}
			if rec := mustReconcileForming(t, st, ctx, queueID); rec.Fired {
				t.Fatal("expected the group to wait: the table is not full yet")
			}

			strangers := make([]*User, len(tc.strangerPaths))
			for i, path := range tc.strangerPaths {
				strangers[i] = mustUser(t, st, cleaner, "shape-stranger")
				if _, err := st.JoinModeQueue(ctx, queueID, strangers[i].ID, path, nil); err != nil {
					t.Fatalf("stranger %d joining on %q: %v", i, path, err)
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
			if len(assignments) != tc.matchSize {
				t.Fatalf("expected %d players in the match, got %d", tc.matchSize, len(assignments))
			}

			seatByUser := map[uuid.UUID]string{}
			for _, a := range assignments {
				seatByUser[a.UserID] = a.SeatKey
			}
			// "You'll be taken in automatically" — asserted for every player, friend and
			// stranger alike, rather than assumed.
			sides := func(users []*User) []string {
				out := make([]string, len(users))
				for i, user := range users {
					seat, ok := seatByUser[user.ID]
					if !ok {
						t.Fatalf("player %s did not reach the match", user.ID)
					}
					out[i] = sideOf(seat)
				}
				return out
			}
			tc.arrangement(t, sides(friends), sides(strangers))
		})
	}
}

// groupTogetherAndStrangersOpposite is the expectation shared by every two-sided shape:
// the friends play together and the arrivals are their opponents.
func groupTogetherAndStrangersOpposite(t *testing.T, group, strangers []string) {
	t.Helper()
	for _, side := range group[1:] {
		if side != group[0] {
			t.Fatalf("the group was split across sides: %s and %s", group[0], side)
		}
	}
	for _, side := range strangers {
		if side == group[0] {
			t.Fatalf("a stranger was seated on the group's own side (%s)", side)
		}
	}
}

// Filling the remaining seats with strangers is the king's, like starting early, because
// it is the same decision: it ends the wait for anyone still on their way, selling their
// seat rather than playing without them (JQ-137).
func TestStartTableBackfillIsTheKingsAlone(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	game, mode, queueID := setupTeamMode(t, st, cleaner)
	king := mustUser(t, st, cleaner, "gate-king")
	friend := mustUser(t, st, cleaner, "gate-friend")
	watcher := mustUser(t, st, cleaner, "gate-watcher")

	table := mustGroupTable(t, st, game, mode, king, map[*User]string{
		king:   "Team-1-Seat-1",
		friend: "Team-1-Seat-2",
	}, watcher)

	// Seated, and still not theirs to press.
	if _, err := st.StartTableBackfill(ctx, table.ID, friend.ID, queueID); err == nil {
		t.Fatal("expected a seated player who is not the king to be refused")
	}
	// Nor a room member watching from outside the table.
	if _, err := st.StartTableBackfill(ctx, table.ID, watcher.ID, queueID); err == nil {
		t.Fatal("expected a room member with no seat to be refused")
	}
	if _, err := st.StartTableBackfill(ctx, table.ID, king.ID, queueID); err != nil {
		t.Fatalf("the king could not ask: %v", err)
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

	// Bob cannot withdraw what Alice committed the group to.
	if _, err := st.CancelTableBackfill(ctx, table.ID, bob.ID); err == nil {
		t.Fatal("expected a seated player who is not the king to be refused the cancel")
	}

	result, err := st.CancelTableBackfill(ctx, table.ID, alice.ID)
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
	if _, err := st.StartTableBackfill(ctx, table.ID, alice.ID, queueID); err != nil {
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

// Starting before the table is full is the king's alone, and stays so (JQ-137).
//
// Nothing else on the group screen is theirs any more — anyone seated may ask for the
// rest of the match, and anyone may stop it — which is exactly why this one needs pinning
// rather than leaving to hold by accident. Deciding not to wait for the friend still on
// their way is a decision taken on everybody's behalf, so it keeps an owner.
//
// The nil-actor arm of startTable exists only for a full table, and is asserted here
// alongside the king gate because the two are one rule: a start nobody authorised is
// permitted only when there is nothing left to decide.
func TestOnlyTheKingCanStartEarly(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	// Word Hunt starts at four — two clue givers and two guessers — in a nine-seat mode,
	// so a table can be playable and a long way from full at the same time.
	game, mode, _ := setupWordHuntMode(t, st, cleaner)
	modeSeats, err := st.ListGameModeSeats(ctx, mode.ID)
	if err != nil {
		t.Fatalf("ListGameModeSeats: %v", err)
	}
	seatsInPath := func(path string, n int) []string {
		var out []string
		for _, seat := range modeSeats {
			if seatQueuePathValue(seat) == path && len(out) < n {
				out = append(out, seat.SeatKey)
			}
		}
		if len(out) < n {
			t.Fatalf("mode has fewer than %d seats on path %q", n, path)
		}
		return out
	}
	clueGivers := seatsInPath("ClueGiver", 2)
	guessers := seatsInPath("Guesser", 2)

	king := mustUser(t, st, cleaner, "early-king")
	others := []*User{
		mustUser(t, st, cleaner, "early-b"),
		mustUser(t, st, cleaner, "early-c"),
		mustUser(t, st, cleaner, "early-d"),
	}
	table := mustGroupTable(t, st, game, mode, king, map[*User]string{
		king:      clueGivers[0],
		others[0]: clueGivers[1],
		others[1]: guessers[0],
		others[2]: guessers[1],
	})

	// Playable, and five seats short of full.
	canStart, err := st.TableCanStart(ctx, table.ID)
	if err != nil {
		t.Fatalf("TableCanStart: %v", err)
	}
	if !canStart {
		t.Fatal("expected four seated players to be enough to start this mode")
	}

	for _, other := range others {
		if _, err := st.StartTable(ctx, table.ID, other.ID); err == nil {
			t.Fatalf("player %s started the game early without being the king", other.ID)
		}
	}

	// Nor may the automatic path stand in for them: it starts a full table, never an
	// early one.
	auto, err := st.AutoStartTable(ctx, table.ID)
	if err != nil {
		t.Fatalf("AutoStartTable: %v", err)
	}
	if auto != nil {
		t.Fatal("a table five seats short of full started itself")
	}

	result, err := st.StartTable(ctx, table.ID, king.ID)
	if err != nil {
		t.Fatalf("the king could not start early: %v", err)
	}
	if len(result.NotifyUserIDs) != 4 {
		t.Fatalf("expected all 4 seated players taken into the match, got %d",
			len(result.NotifyUserIDs))
	}
}
