package graph

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

// subscribeTableUpdated starts a tableUpdated subscription on an already connection_init'd
// websocket and returns its operation id. The selection carries seats because that is what
// every assertion here is about: a publish that fires is only half the guarantee, the other
// half is that the table it resolves to is the one that changed.
func subscribeTableUpdated(t *testing.T, conn *websocket.Conn, roomID string) string {
	t.Helper()

	subID := "sub-table-" + roomID
	query := fmt.Sprintf(`subscription { tableUpdated(roomId: %q) { id status seats { seatKey user { id } } } }`, roomID)
	if err := writeGraphQLWS(conn, map[string]any{
		"id":   subID,
		"type": "start",
		"payload": map[string]any{
			"query": query,
		},
	}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	return subID
}

// nextTableUpdatedPayload waits for the next "data" message on subID, forwarding
// keep-alives, and fails on "error"/"complete" so a room-membership refusal surfaces as
// itself rather than as a timeout.
//
// The timeout message names transposed publish arguments on purpose. publishTableUpdated
// takes (roomID, tableID) and the room id picks the channel, so swapping them publishes to
// a channel nobody is subscribed to: no error anywhere, just a subscriber that never hears
// anything. That is the exact silent failure this file exists to catch, and "timed out" on
// its own would send the next reader looking in the wrong place.
func nextTableUpdatedPayload(t *testing.T, conn *websocket.Conn, subID string, timeout time.Duration) map[string]any {
	t.Helper()

	deadline := time.Now().Add(timeout)
	// See nextMatchResultUpdatedPayload: without a read deadline on the connection the
	// loop condition is decorative, and a subscription that stops pushing parks in
	// ReadJSON until the whole binary times out.
	if err := conn.SetReadDeadline(deadline); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	for time.Now().Before(deadline) {
		msg, err := readGraphQLWS(conn)
		if err != nil {
			if isWSReadTimeout(err) {
				t.Fatalf("timed out waiting for a tableUpdated push (a publish that never fired, or one whose room and table ids are transposed)")
			}
			t.Fatalf("read websocket: %v", err)
		}
		typ, _ := msg["type"].(string)
		if typ == "ping" {
			_ = writeGraphQLWS(conn, map[string]any{"type": "pong"})
			continue
		}
		if typ == "ka" {
			continue
		}
		if typ == "error" || typ == "complete" {
			t.Fatalf("subscription ended: %v", msg)
		}
		if typ != "data" || msg["id"] != subID {
			continue
		}
		payload, _ := msg["payload"].(map[string]any)
		data, _ := payload["data"].(map[string]any)
		update, _ := data["tableUpdated"].(map[string]any)
		if update != nil {
			return update
		}
	}
	t.Fatalf("timed out waiting for a tableUpdated push (a publish that never fired, or one whose room and table ids are transposed)")
	return nil
}

// seatedUserIDs reads the user ids out of a pushed table's seats.
func seatedUserIDs(t *testing.T, table map[string]any) map[string]bool {
	t.Helper()

	seats, ok := table["seats"].([]any)
	if !ok {
		t.Fatalf("pushed table has no seats array: %+v", table)
	}
	out := make(map[string]bool, len(seats))
	for _, raw := range seats {
		seat, _ := raw.(map[string]any)
		user, _ := seat["user"].(map[string]any)
		id, _ := user["id"].(string)
		if id != "" {
			out[id] = true
		}
	}
	return out
}

// claimRegroupTable has player A accept the regroup offer. It is the setup step every test
// here needs: accepting is what creates the regroup table, and tableUpdated is gated on
// room membership, so somebody has to be in the room before anyone can watch it.
func claimRegroupTable(t *testing.T, env *queueIntegrationEnv, match finishedMatch) *store.RoomTable {
	t.Helper()

	claim := playAgain(t, env, match.sessionID, match.cookieA)
	if len(claim.Errors) > 0 || claim.Data.PlayAgain == nil {
		t.Fatalf("playAgain A: %+v", claim.Errors)
	}
	tableID, err := uuid.Parse(claim.Data.PlayAgain.Table.ID)
	if err != nil {
		t.Fatalf("parse regroup table id %q: %v", claim.Data.PlayAgain.Table.ID, err)
	}
	// The room id never crosses the API — Table has no roomId field — so the test reads
	// it the way the resolver does, from the table row.
	table, err := env.Store.GetRoomTableByID(context.Background(), tableID)
	if err != nil {
		t.Fatalf("GetRoomTableByID: %v", err)
	}
	if table.RoomID == table.ID {
		t.Fatalf("room id and table id are the same value (%s); this suite cannot tell a transposed publish from a correct one", table.ID)
	}
	return table
}

// watchRoomTables opens userID's tableUpdated subscription for roomID and does not return
// until it is actually live.
//
// The wait is the part that matters. tableUpdated sends no initial payload — unlike
// queueUpdated and matchResultUpdated, whose first push doubles as a readiness barrier — so
// a mutation fired straight after the "start" frame can publish before gqlgen has run the
// resolver, and the event goes to a channel with no subscribers yet. That loses the push
// this suite is here to assert on, at random, on a loaded machine.
//
// user_presence is the barrier because TableUpdated calls Presence.Track immediately after
// PubSub.Subscribe returns, and Track writes the row synchronously: a non-zero
// connection_count therefore means the channel subscription already exists. It only reads
// as *this* subscription because the user holds no other socket in these tests.
func watchRoomTables(t *testing.T, env *queueIntegrationEnv, userID, roomID uuid.UUID) (*websocket.Conn, string) {
	t.Helper()

	if count, _ := readPresence(t, env, userID); count != 0 {
		t.Fatalf("player already has %d live socket(s); presence cannot serve as the readiness barrier", count)
	}

	token, err := env.Signer.SignUserToken(userID, time.Hour)
	if err != nil {
		t.Fatalf("SignUserToken: %v", err)
	}
	conn := connectGraphQLWS(t, graphQLWSURL(env.Server.URL), "http://localhost:5173", "Bearer "+token)
	subID := subscribeTableUpdated(t, conn, roomID.String())

	deadline := time.Now().Add(5 * time.Second)
	for {
		if count, _ := readPresence(t, env, userID); count > 0 {
			return conn, subID
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for the tableUpdated subscription to go live")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestTableUpdatedPushesOnPlayAgain covers the publish playAgain has always made and
// nothing has ever checked: a player already sitting at the regroup table, watching
// /room/{code}, sees the next player arrive without refetching.
//
// Transposing publishTableUpdated's arguments in PlayAgain fails here — the event lands on
// a channel keyed by the table id, this subscriber is on the room's channel, and no push
// ever arrives.
func TestTableUpdatedPushesOnPlayAgain(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	match := seedFinishedPartyMatch(t, env, cleaner)

	table := claimRegroupTable(t, env, match)
	conn, subID := watchRoomTables(t, env, match.userA.ID, table.RoomID)

	resp := playAgain(t, env, match.sessionID, match.cookieB)
	if len(resp.Errors) > 0 || resp.Data.PlayAgain == nil {
		t.Fatalf("playAgain B: %+v", resp.Errors)
	}

	pushed := nextTableUpdatedPayload(t, conn, subID, 5*time.Second)
	if pushed["id"] != table.ID.String() {
		t.Fatalf("pushed table id = %v, want the regroup table %v", pushed["id"], table.ID)
	}
	seated := seatedUserIDs(t, pushed)
	if !seated[match.userB.ID.String()] {
		t.Fatalf("player B is missing from the pushed seats after claiming one: %+v", pushed)
	}
	if !seated[match.userA.ID.String()] {
		t.Fatalf("player A lost their seat in the pushed table: %+v", pushed)
	}
}

// TestTableUpdatedPushesOnDeclinePlayAgain is the regression this whole file was written
// for. declinePlayAgain frees the decliner's seat, and before JQ-135 it published only a
// match event — so everyone at /room/{code}, who watches tableUpdated and not
// matchResultUpdated, kept seeing the decliner seated until some unrelated table event
// happened to fire.
//
// Player B claims a seat before the subscription opens, so the only push this test can
// possibly receive is the decline's. Transposing publishTableUpdated's arguments in
// DeclinePlayAgain, or dropping the call, times the read out.
func TestTableUpdatedPushesOnDeclinePlayAgain(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	match := seedFinishedPartyMatch(t, env, cleaner)

	table := claimRegroupTable(t, env, match)

	claimB := playAgain(t, env, match.sessionID, match.cookieB)
	if len(claimB.Errors) > 0 || claimB.Data.PlayAgain == nil {
		t.Fatalf("playAgain B: %+v", claimB.Errors)
	}
	if !claimB.Data.PlayAgain.Seated {
		t.Fatal("player B was not seated, so there is no seat for the decline to free")
	}

	conn, subID := watchRoomTables(t, env, match.userA.ID, table.RoomID)

	decline := declinePlayAgain(t, env, match.sessionID, match.cookieB)
	if len(decline.Errors) > 0 || decline.Data.DeclinePlayAgain == nil {
		t.Fatalf("declinePlayAgain B: %+v", decline.Errors)
	}

	pushed := nextTableUpdatedPayload(t, conn, subID, 5*time.Second)
	if pushed["id"] != table.ID.String() {
		t.Fatalf("pushed table id = %v, want the regroup table %v", pushed["id"], table.ID)
	}
	seated := seatedUserIDs(t, pushed)
	if seated[match.userB.ID.String()] {
		t.Fatalf("the decliner is still seated in the pushed table: %+v", pushed)
	}
	if !seated[match.userA.ID.String()] {
		t.Fatalf("player A lost their seat when B declined: %+v", pushed)
	}
}
