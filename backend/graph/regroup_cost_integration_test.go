package graph

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/99designs/gqlgen/client"
	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/graph/model"
	"github.com/scruffyprodigy/joinquest/internal/auth"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

// regroupCostRoomQuery is the room load the player shell actually sends: TABLE_FIELDS, which
// selects regroupRoster for every table in the room (frontend/src/lib/tables.js).
const regroupCostRoomQuery = `query { myRoom { id tables {
	id status canStart canDiscard
	game { id name }
	mode { id displayName }
	king { id displayName }
	seats { seatKey user { id displayName } }
	seatSlots { seatKey displayName user { id } }
	backfillActive
	regroupRoster { user { id displayName } role regroup }
} } }`

// regroupCostQueryWithoutRoster is the same selection with regroupRoster removed. The
// difference between the two is what the field costs, which is the number JQ-177 is about;
// a total on its own moves whenever anything else in the room query does.
const regroupCostQueryWithoutRoster = `query { myRoom { id tables {
	id status canStart canDiscard
	game { id name }
	mode { id displayName }
	king { id displayName }
	seats { seatKey user { id displayName } }
	seatSlots { seatKey displayName user { id } }
	backfillActive
} } }`

// TestRegroupRosterCostsNothingForAnOrdinaryTable is the measurement JQ-177 asks for, and
// the assertion it turns into. Almost every table in existence was created directly and has
// no originating match, and that case now answers from the model rather than the database:
// selecting regroupRoster costs exactly as much as not selecting it.
func TestRegroupRosterCostsNothingForAnOrdinaryTable(t *testing.T) {
	env, counter := newCountingIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := context.Background()

	_, cookie := createTestUserSession(t, ctx, env, cleaner)
	// Three tables, because the cost being measured was per table: a room of N ordinary
	// tables used to pay N reverse lookups on every load.
	seedOrdinaryTables(t, env, cookie, 3)

	// Warm first: the first room load of a session pays for things a later one does not,
	// and the comparison is between two steady-state loads.
	postGraphQL(t, env.Handler, regroupCostRoomQuery, nil, cookie)

	withRoster := counter.measure(func() {
		requireNoGraphQLErrors(t, postGraphQL(t, env.Handler, regroupCostRoomQuery, nil, cookie))
	})
	withoutRoster := counter.measure(func() {
		requireNoGraphQLErrors(t, postGraphQL(t, env.Handler, regroupCostQueryWithoutRoster, nil, cookie))
	})

	t.Logf("room load, three ordinary tables: %d statements with regroupRoster, %d without",
		len(withRoster), len(withoutRoster))

	if len(withRoster) != len(withoutRoster) {
		t.Errorf("regroupRoster cost %d extra statements on a table with no originating match:\n%s",
			len(withRoster)-len(withoutRoster), withRoster)
	}
}

// TestRegroupRosterCostOnARegroupTable records what the field costs where it does have an
// answer to give. Nothing here is asserted as a budget — the point is that the number
// exists, and that it is paid only by the tables that are actually regrouping.
func TestRegroupRosterCostOnARegroupTable(t *testing.T) {
	env, counter := newCountingIntegrationEnv(t)
	cleaner := env.newCleaner(t)

	_, strangerCookie, tableID := seedRegroupTableWithStranger(t, env, cleaner)
	postGraphQL(t, env.Handler, regroupCostRoomQuery, nil, strangerCookie)

	withRoster := counter.measure(func() {
		requireNoGraphQLErrors(t, postGraphQL(t, env.Handler, regroupCostRoomQuery, nil, strangerCookie))
	})
	withoutRoster := counter.measure(func() {
		requireNoGraphQLErrors(t, postGraphQL(t, env.Handler, regroupCostQueryWithoutRoster, nil, strangerCookie))
	})

	t.Logf("room load, one regroup table (%s): %d statements with regroupRoster, %d without — the field costs %d",
		tableID, len(withRoster), len(withoutRoster), len(withRoster)-len(withoutRoster))

	if len(withRoster) <= len(withoutRoster) {
		t.Fatalf("a regroup table answered its roster for free (%d vs %d); the measurement is not measuring anything",
			len(withRoster), len(withoutRoster))
	}
	// The roster is one match-result read and one regroup-state read. It used to be that
	// plus a reverse lookup into game_sessions to discover which match to read at all.
	if got := withRoster.touching("FROM game_sessions"); got > 1 {
		t.Errorf("the roster reads game_sessions %d times, want at most the one that loads the result:\n%s", got, withRoster)
	}
}

// TestTableUpdatedPushCostsNothingExtraForAnOrdinaryTable measures the other live path. A
// room push resolves the same Table selection as a room load, once per table update and
// once per subscriber, so the ordinary table has to be free here too.
//
// The subscription is driven through the resolver rather than a websocket: the channel
// receive says exactly when the push has been built, where a socket read cannot separate
// the push under measurement from one still in flight (JQ-175 has the socket harness).
func TestTableUpdatedPushCostsNothingExtraForAnOrdinaryTable(t *testing.T) {
	env, counter := newCountingIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := context.Background()

	user := createTestUser(t, ctx, env, cleaner, "push-cost-"+uuid.NewString()+"@example.com", "Push Cost")
	_, cookie := createTestUserSessionForUser(t, env, user.ID)
	tableID := seedOrdinaryTables(t, env, cookie, 1)[0]
	roomID := myRoomID(t, env, cookie)

	subCtx, cancel := context.WithCancel(auth.WithUserID(ctx, user.ID.String()))
	defer cancel()
	updates, err := env.resolver.Subscription().TableUpdated(subCtx, roomID)
	if err != nil {
		t.Fatalf("subscribe tableUpdated: %v", err)
	}

	nextPush := func() *model.Table {
		t.Helper()
		if err := env.resolver.publishTableUpdated(ctx, uuid.MustParse(roomID), uuid.MustParse(tableID)); err != nil {
			t.Fatalf("publish tableUpdated: %v", err)
		}
		select {
		case table := <-updates:
			if table == nil {
				t.Fatal("tableUpdated closed instead of pushing")
			}
			return table
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for a tableUpdated push")
			return nil
		}
	}

	// Subscribe registers with pub/sub inside the resolver, but the first publish can still
	// lose the race against the goroutine that starts reading, so warm up until one lands.
	warm := nextPush()
	if warm.ID != tableID {
		t.Fatalf("pushed table %s, want %s", warm.ID, tableID)
	}

	var roster []*model.RegroupRosterEntry
	push := counter.measure(func() {
		table := nextPush()
		roster, err = env.resolver.Table().RegroupRoster(ctx, table)
	})
	if err != nil {
		t.Fatalf("resolve regroupRoster on a pushed table: %v", err)
	}
	if len(roster) != 0 {
		t.Errorf("an ordinary table pushed a roster of %d entries", len(roster))
	}

	t.Logf("tableUpdated push for an ordinary table, including regroupRoster: %d statements\n%s", len(push), push)

	// Building the pushed table is one read of room_tables. Answering regroupRoster on top
	// of it is now zero, where it used to be a game_sessions lookup per push per subscriber.
	if len(push) != 1 {
		t.Errorf("a push for a table with no originating match cost %d statements, want 1:\n%s", len(push), push)
	}
	if got := push.touching("game_sessions"); got != 0 {
		t.Errorf("the push read game_sessions %d times for a table that never came from a match:\n%s", got, push)
	}
}

// TestMatchResultPushDoesNotRefetchWhatCannotChange is the subscription half. Every event on
// a four-player match wakes this load once per subscriber, so the parts of it that are
// constant for the life of the subscription — the game — must not be re-read, and the mode
// must not be read at all unless the client asked for it.
func TestMatchResultPushDoesNotRefetchWhatCannotChange(t *testing.T) {
	env, counter := newCountingIntegrationEnv(t)
	cleaner := env.newCleaner(t)

	match := seedFinishedMatch(t, env, cleaner)
	bearer, _ := createTestUserSessionForUser(t, env, match.userA.ID)

	conn := connectGraphQLWS(t, graphQLWSURL(env.Server.URL), env.Server.URL, bearer)
	subID := subscribeMatchResultUpdated(t, conn, match.sessionID.String())
	nextMatchResultUpdatedPayload(t, conn, subID, 5*time.Second)

	push := counter.measure(func() {
		if err := env.resolver.publishMatchEvent(context.Background(), match.sessionID, "regroup"); err != nil {
			t.Fatalf("publish match event: %v", err)
		}
		nextMatchResultUpdatedPayload(t, conn, subID, 5*time.Second)
	})

	t.Logf("matchResultUpdated push: %d statements\n%s", len(push), push)

	// subscribeMatchResultUpdated selects matchId, complete, reported, status and
	// participants — not mode.
	if got := push.touching("FROM game_modes"); got != 0 {
		t.Errorf("the push read game_modes %d times for a client that never selected mode:\n%s", got, push)
	}
	if got := push.touching("FROM games"); got != 0 {
		t.Errorf("the push re-read the game %d times; game_sessions.game_id cannot change while the subscription is open:\n%s", got, push)
	}
	// One pass over the participants, inside the match-result read. There used to be a
	// second, purely to turn the same rows into player records.
	if got := push.touching("JOIN users u ON u.id = p.user_id"); got != 1 {
		t.Errorf("the push turned participants into player records %d times, want 1 — the match-result read already joins them:\n%s", got, push)
	}
}

// seedOrdinaryTables puts count tables with no originating match in the caller's room —
// what nearly every table in a room is — and returns their ids. The first one creates the
// room; the rest join it, so the measurement sees one room with several tables rather than
// several rooms.
func seedOrdinaryTables(t *testing.T, env *queueIntegrationEnv, cookie *http.Cookie, count int) []string {
	t.Helper()

	var gameResp struct {
		Game struct {
			ID    string
			Modes []struct{ ID string } `json:"modes"`
		} `json:"game"`
	}
	if err := env.Client.Post(`query Game($id: ID!) { game(id: $id) { id modes { id } } }`,
		&gameResp, client.AddCookie(cookie), client.Var("id", store.DemoPrimaryGameIDStr)); err != nil {
		t.Fatalf("game query: %v", err)
	}
	if len(gameResp.Game.Modes) == 0 {
		t.Fatal("expected the demo game to have a mode")
	}

	var tableResp struct {
		CreatePrivateTable struct{ ID string } `json:"createPrivateTable"`
	}
	if err := env.Client.Post(`mutation Create($gameId: ID!, $modeId: ID!) {
		createPrivateTable(gameId: $gameId, modeId: $modeId) { id }
	}`, &tableResp, client.AddCookie(cookie),
		client.Var("gameId", gameResp.Game.ID),
		client.Var("modeId", gameResp.Game.Modes[0].ID)); err != nil {
		t.Fatalf("createPrivateTable: %v", err)
	}
	ids := []string{tableResp.CreatePrivateTable.ID}

	roomID := myRoomID(t, env, cookie)
	for len(ids) < count {
		var extra struct {
			CreateTable struct{ ID string } `json:"createTable"`
		}
		if err := env.Client.Post(`mutation Create($roomId: ID!, $gameId: ID!, $modeId: ID!) {
			createTable(roomId: $roomId, gameId: $gameId, modeId: $modeId) { id }
		}`, &extra, client.AddCookie(cookie),
			client.Var("roomId", roomID),
			client.Var("gameId", gameResp.Game.ID),
			client.Var("modeId", gameResp.Game.Modes[0].ID)); err != nil {
			t.Fatalf("createTable: %v", err)
		}
		ids = append(ids, extra.CreateTable.ID)
	}
	return ids
}

func myRoomID(t *testing.T, env *queueIntegrationEnv, cookie *http.Cookie) string {
	t.Helper()
	var resp struct {
		MyRoom *struct{ ID string } `json:"myRoom"`
	}
	if err := env.Client.Post(`query { myRoom { id } }`, &resp, client.AddCookie(cookie)); err != nil {
		t.Fatalf("myRoom: %v", err)
	}
	if resp.MyRoom == nil {
		t.Fatal("myRoom is null; the table seed did not create a room")
	}
	return resp.MyRoom.ID
}
