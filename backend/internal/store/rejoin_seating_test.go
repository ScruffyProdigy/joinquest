package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/gameclient"
	"github.com/scruffyprodigy/joinquest/internal/prequeue"
)

func TestModeOffersPreMatchChoice(t *testing.T) {
	cases := []struct {
		name     string
		template string
		preQueue string
		want     bool
	}{
		{
			name:     "flat template, no options",
			template: `{"count":2}`,
			want:     false,
		},
		{
			// The seat keys differ ("1".."5") but the class does not, so there is no
			// choice here — which is the whole point of keying on the class.
			name:     "many identical seats are one seat type",
			template: `{"count":5}`,
			want:     false,
		},
		{
			name:     "two roles",
			template: `{"ClueGiver":{"count":1},"Guesser":{"count":3}}`,
			want:     true,
		},
		{
			name:     "one role, several seats, still one class",
			template: `{"Guesser":{"count":4}}`,
			want:     false,
		},
		{
			name:     "flat template with pre-queue options",
			template: `{"count":2}`,
			preQueue: `{"groups":[{"key":"helpers","kind":"Loadout","label":"Pick two","min":2,"max":2}]}`,
			want:     true,
		},
		{
			// Nothing can describe this mode's seats, so nothing should be guessing a
			// seat for a returning player either.
			name:     "unreadable template",
			template: `{"seats":[]}`,
			want:     true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mode := &GameMode{SeatTemplate: json.RawMessage(tc.template)}
			if tc.preQueue != "" {
				mode.PreQueue = json.RawMessage(tc.preQueue)
			}
			if got := ModeOffersPreMatchChoice(mode); got != tc.want {
				t.Fatalf("ModeOffersPreMatchChoice = %v, want %v", got, tc.want)
			}
		})
	}
}

// setupRolesMode registers the smallest mode that has something to choose: two seats in
// two different classes, so returning players have a role to swap.
func setupRolesMode(t *testing.T, st *Store, cleaner *TestCleaner) (*Game, *GameMode) {
	t.Helper()
	return setupRejoinMode(t, st, cleaner, "roles", `{"ClueGiver":{"count":1},"Guesser":{"count":1}}`, "")
}

func setupRejoinMode(t *testing.T, st *Store, cleaner *TestCleaner, key, template, preQueue string) (*Game, *GameMode) {
	t.Helper()
	ctx := context.Background()
	slug := "rejoin-" + key + "-" + uuid.NewString()
	modeManifest := gameclient.ModeManifest{
		Key:          key,
		DisplayName:  key,
		SeatTemplate: json.RawMessage(template),
	}
	if preQueue != "" {
		modeManifest.PreQueue = json.RawMessage(preQueue)
	}
	manifest := &gameclient.Manifest{
		Modes:      []gameclient.ModeManifest{modeManifest},
		Status:     gameclient.StatusResponse{Game: key, Version: "1.0.0"},
		ETag:       `"` + key + `"`,
		RawJSON:    []byte(`{"modes":[{"key":"` + key + `"}]}`),
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
	return result.Game, &modes[0]
}

func newRejoinUser(t *testing.T, st *Store, ctx context.Context, cleaner *TestCleaner, tag string) *User {
	t.Helper()
	user, err := st.CreateUser(ctx, CreateUserParams{Email: "rejoin-" + tag + "-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser %s: %v", tag, err)
	}
	cleaner.TrackUser(user.ID)
	return user
}

// playRoomTableMatch seats two players at a room table, starts it and completes the
// session — the room-table origin, which is what "played with a group" means here.
func playRoomTableMatch(t *testing.T, st *Store, ctx context.Context, cleaner *TestCleaner, game *Game, mode *GameMode, options []prequeue.Selection) (sessionID uuid.UUID, table *RoomTable, host, guest *User) {
	t.Helper()
	host = newRejoinUser(t, st, ctx, cleaner, "host")
	guest = newRejoinUser(t, st, ctx, cleaner, "guest")

	room, err := st.CreateRoom(ctx, host.ID)
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	if err := st.addRoomMemberDirect(ctx, room.ID, guest.ID); err != nil {
		t.Fatalf("add guest: %v", err)
	}
	table, err = st.CreateTable(ctx, room.ID, game.ID, mode.ID, host.ID)
	if err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	modeSeats, err := st.ListGameModeSeats(ctx, mode.ID)
	if err != nil {
		t.Fatalf("ListGameModeSeats: %v", err)
	}
	if len(modeSeats) < 2 {
		t.Fatalf("mode has %d seats, want at least 2", len(modeSeats))
	}
	if _, err := st.SitAtTableWithOptions(ctx, table.ID, host.ID, modeSeats[0].SeatKey, options); err != nil {
		t.Fatalf("host sit: %v", err)
	}
	if _, err := st.SitAtTableWithOptions(ctx, table.ID, guest.ID, modeSeats[1].SeatKey, options); err != nil {
		t.Fatalf("guest sit: %v", err)
	}
	started, err := st.StartTable(ctx, table.ID, host.ID)
	if err != nil {
		t.Fatalf("StartTable: %v", err)
	}
	if err := st.CompleteSession(ctx, started.SessionID, time.Now()); err != nil {
		t.Fatalf("CompleteSession: %v", err)
	}
	return started.SessionID, table, host, guest
}

func seatCountAtTable(t *testing.T, st *Store, ctx context.Context, tableID uuid.UUID) int {
	t.Helper()
	seats, err := st.ListTableSeats(ctx, tableID)
	if err != nil {
		t.Fatalf("ListTableSeats: %v", err)
	}
	return len(seats)
}

// A mode with roles to rotate must come back empty. Re-seating everyone on their old seat
// is what silently proposes "take your old seat again", which is the opposite of what a
// group that rotates the Clue Giver came back to do (JQ-232).
func TestCompleteSessionDoesNotReseatWhenThereAreRolesToRotate(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	game, mode := setupRolesMode(t, st, cleaner)
	sessionID, table, host, guest := playRoomTableMatch(t, st, ctx, cleaner, game, mode, nil)

	if got := seatCountAtTable(t, st, ctx, table.ID); got != 0 {
		t.Fatalf("seats after completion = %d, want 0", got)
	}

	// The card reads the roster, and both players must be Awaiting on it — which they
	// only are because nothing seated them behind their backs.
	roster, err := st.GetRegroupRoster(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetRegroupRoster: %v", err)
	}
	if roster[host.ID] != RegroupPending || roster[guest.ID] != RegroupPending {
		t.Fatalf("roster = %v/%v, want PENDING for both", roster[host.ID], roster[guest.ID])
	}
}

// Pre-queue options count exactly as several seat types do: pre-selecting last round's
// character pre-empts the choice just as firmly as pre-seating pre-empts rotation.
func TestCompleteSessionDoesNotReseatWhenTheModeHasPreQueueOptions(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	game, mode := setupRejoinMode(t, st, cleaner, "options", `{"count":2}`,
		`{"groups":[{"key":"helpers","kind":"Loadout","label":"Choose your two helpers","min":2,"max":2}]}`)
	_, table, _, _ := playRoomTableMatch(t, st, ctx, cleaner, game, mode, helperPicks())

	if got := seatCountAtTable(t, st, ctx, table.ID); got != 0 {
		t.Fatalf("seats after completion = %d, want 0 — no option may be pre-selected either", got)
	}
}

// The other half of the rule, and the reason it is not simply "never re-seat": where
// there is nothing to choose, making people re-click an identical seat is friction with
// no upside.
func TestCompleteSessionReseatsWhenThereIsNothingToChoose(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	game, mode := setupDuelMode(t, st, cleaner)
	_, table, _, _ := playRoomTableMatch(t, st, ctx, cleaner, game, mode, nil)

	if got := seatCountAtTable(t, st, ctx, table.ID); got != 2 {
		t.Fatalf("seats after completion = %d, want 2", got)
	}
}

// Clicking "Another round" is not itself a seat choice. A group claimant is recorded as
// back and left standing, so taking a different seat is exactly as easy as taking the
// previous one.
func TestClaimRegroupTableSeatsNoGroupPlayerWhenThereIsAChoice(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	game, mode := setupRolesMode(t, st, cleaner)
	sessionID, table, host, _ := playRoomTableMatch(t, st, ctx, cleaner, game, mode, nil)

	claimed, _, err := st.ClaimRegroupTable(ctx, sessionID, host.ID)
	if err != nil {
		t.Fatalf("ClaimRegroupTable: %v", err)
	}
	if claimed.ID != table.ID {
		t.Fatalf("claimed table = %s, want the group's own table %s", claimed.ID, table.ID)
	}
	if got := seatCountAtTable(t, st, ctx, table.ID); got != 0 {
		t.Fatalf("seats after claim = %d, want 0", got)
	}

	// Seatless, but demonstrably back: an IN who holds no seat is "returned, still
	// picking", which is what the "Picking a seat" card renders.
	roster, err := st.GetRegroupRoster(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetRegroupRoster: %v", err)
	}
	if roster[host.ID] != RegroupIn {
		t.Fatalf("claimant = %v, want IN", roster[host.ID])
	}
}

// A solo player almost always wants exactly what they just had, so the fast path carries
// both halves of it forward: the seat and the options.
func TestClaimRegroupTableReplaysASoloPlayersSelections(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	_, _, queueID := setupHelpersMode(t, st, cleaner)
	first := newRejoinUser(t, st, ctx, cleaner, "solo-a")
	second := newRejoinUser(t, st, ctx, cleaner, "solo-b")

	firstPicks := helperPicks()
	secondPicks := []prequeue.Selection{{GroupKey: "helpers", OptionIDs: []string{"chimera", "grudge"}}}
	if _, err := st.JoinModeQueueWithOptions(ctx, queueID, first.ID, "", firstPicks, nil); err != nil {
		t.Fatalf("join first: %v", err)
	}
	if _, err := st.JoinModeQueueWithOptions(ctx, queueID, second.ID, "", secondPicks, nil); err != nil {
		t.Fatalf("join second: %v", err)
	}
	mustReconcileForming(t, st, ctx, queueID)

	session, err := st.GetMatchedSessionForUserAndModeQueue(ctx, queueID, first.ID)
	if err != nil {
		t.Fatalf("GetMatchedSessionForUserAndModeQueue: %v", err)
	}
	played, err := st.ListSessionSeatAssignments(ctx, session.ID)
	if err != nil {
		t.Fatalf("ListSessionSeatAssignments: %v", err)
	}
	playedSeat := map[uuid.UUID]string{}
	for _, p := range played {
		playedSeat[p.UserID] = p.SeatKey
	}
	if err := st.CompleteSession(ctx, session.ID, time.Now()); err != nil {
		t.Fatalf("CompleteSession: %v", err)
	}

	table, _, err := st.ClaimRegroupTable(ctx, session.ID, first.ID)
	if err != nil {
		t.Fatalf("ClaimRegroupTable: %v", err)
	}
	seats, err := st.ListTableSeats(ctx, table.ID)
	if err != nil {
		t.Fatalf("ListTableSeats: %v", err)
	}
	if len(seats) != 1 {
		t.Fatalf("seats after solo claim = %d, want 1", len(seats))
	}
	if seats[0].SeatKey != playedSeat[first.ID] {
		t.Fatalf("replayed seat = %q, want the seat they held, %q", seats[0].SeatKey, playedSeat[first.ID])
	}
	if len(seats[0].QueueOptions) != 1 || len(seats[0].QueueOptions[0].OptionIDs) != 2 ||
		seats[0].QueueOptions[0].OptionIDs[0] != "ferrus" {
		t.Fatalf("replayed options = %+v, want ferrus+tempered", seats[0].QueueOptions)
	}

	// And it is their own picks that come back, not the other player's.
	if _, _, err := st.ClaimRegroupTable(ctx, session.ID, second.ID); err != nil {
		t.Fatalf("ClaimRegroupTable second: %v", err)
	}
	seats, err = st.ListTableSeats(ctx, table.ID)
	if err != nil {
		t.Fatalf("ListTableSeats: %v", err)
	}
	for _, seat := range seats {
		if seat.UserID != second.ID {
			continue
		}
		if len(seat.QueueOptions) != 1 || seat.QueueOptions[0].OptionIDs[0] != "chimera" {
			t.Fatalf("second player's replayed options = %+v, want chimera+grudge", seat.QueueOptions)
		}
	}
}

// The guard rail on the whole feature. The solo and the group rule are different on
// purpose — same mode, same click, different outcome — and this asserts both at once so
// neither is later "fixed" into the other.
func TestRejoinSeatingDiffersBetweenSoloAndGroupPlay(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	_, mode, queueID := setupHelpersMode(t, st, cleaner)
	game, err := st.GetGameByID(ctx, mode.GameID)
	if err != nil {
		t.Fatalf("GetGameByID: %v", err)
	}

	// Group: two players who reached the match from a table they were sitting at.
	groupSession, groupTable, groupHost, _ := playRoomTableMatch(t, st, ctx, cleaner, game, mode, helperPicks())
	if _, _, err := st.ClaimRegroupTable(ctx, groupSession, groupHost.ID); err != nil {
		t.Fatalf("ClaimRegroupTable group: %v", err)
	}
	groupSeats := seatCountAtTable(t, st, ctx, groupTable.ID)

	// Solo: two players who reached the same mode through the catalog queue.
	soloA := newRejoinUser(t, st, ctx, cleaner, "solo-a")
	soloB := newRejoinUser(t, st, ctx, cleaner, "solo-b")
	if _, err := st.JoinModeQueueWithOptions(ctx, queueID, soloA.ID, "", helperPicks(), nil); err != nil {
		t.Fatalf("join soloA: %v", err)
	}
	if _, err := st.JoinModeQueueWithOptions(ctx, queueID, soloB.ID, "", helperPicks(), nil); err != nil {
		t.Fatalf("join soloB: %v", err)
	}
	mustReconcileForming(t, st, ctx, queueID)
	soloSession, err := st.GetMatchedSessionForUserAndModeQueue(ctx, queueID, soloA.ID)
	if err != nil {
		t.Fatalf("GetMatchedSessionForUserAndModeQueue: %v", err)
	}
	if err := st.CompleteSession(ctx, soloSession.ID, time.Now()); err != nil {
		t.Fatalf("CompleteSession solo: %v", err)
	}
	soloTable, _, err := st.ClaimRegroupTable(ctx, soloSession.ID, soloA.ID)
	if err != nil {
		t.Fatalf("ClaimRegroupTable solo: %v", err)
	}
	soloSeats := seatCountAtTable(t, st, ctx, soloTable.ID)

	if groupSeats != 0 {
		t.Errorf("group claim seated %d player(s); a group re-chooses", groupSeats)
	}
	if soloSeats != 1 {
		t.Errorf("solo claim seated %d player(s); a solo player's selections are replayed", soloSeats)
	}
}
