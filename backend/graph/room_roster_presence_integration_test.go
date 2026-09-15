package graph

import (
	"context"
	"fmt"
	"testing"
	"time"

	"net/http"

	"github.com/99designs/gqlgen/client"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

// subscribeRoomUpdated starts a roomUpdated subscription on an already connection_init'd
// websocket and returns its operation id. The selection carries the roster's disconnected
// reading because that is the whole subject: a publish that fires is only half the
// guarantee, the other half is that the room it resolves to says what changed.
func subscribeRoomUpdated(t *testing.T, conn *websocket.Conn, roomID string) string {
	t.Helper()

	subID := "sub-room-" + roomID
	query := fmt.Sprintf(`subscription { roomUpdated(roomId: %q) { id members { user { id } disconnected } } }`, roomID)
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

// nextRoomUpdatedPayload waits for the next "data" message on subID, forwarding
// keep-alives, and fails on "error"/"complete" so a room-membership refusal surfaces as
// itself rather than as a timeout.
func nextRoomUpdatedPayload(t *testing.T, conn *websocket.Conn, subID string, timeout time.Duration) map[string]any {
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
				t.Fatalf("timed out waiting for a roomUpdated push (the roster's reading changed with no event to carry it)")
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
		update, _ := data["roomUpdated"].(map[string]any)
		if update != nil {
			return update
		}
	}
	t.Fatalf("timed out waiting for a roomUpdated push (the roster's reading changed with no event to carry it)")
	return nil
}

// rosterReadings reads the pushed room's disconnected flags, keyed by user id.
func rosterReadings(t *testing.T, room map[string]any) map[string]bool {
	t.Helper()

	members, ok := room["members"].([]any)
	if !ok {
		t.Fatalf("pushed room has no members array: %+v", room)
	}
	out := make(map[string]bool, len(members))
	for _, raw := range members {
		member, _ := raw.(map[string]any)
		user, _ := member["user"].(map[string]any)
		id, _ := user["id"].(string)
		disconnected, _ := member["disconnected"].(bool)
		if id != "" {
			out[id] = disconnected
		}
	}
	return out
}

// createRoomAndJoin opens a room as the host and puts the guest in it, returning the room
// id. Both mutations publish, but nobody is watching yet — every test here subscribes
// afterwards, so the only push a subscriber can receive is the one under test.
func createRoomAndJoin(t *testing.T, env *queueIntegrationEnv, hostCookie, guestCookie *http.Cookie) string {
	t.Helper()

	var created struct {
		CreateRoom struct {
			ID         string
			InviteCode string
		} `json:"createRoom"`
	}
	if err := env.Client.Post(`mutation { createRoom { id inviteCode } }`, &created, client.AddCookie(hostCookie)); err != nil {
		t.Fatalf("createRoom: %v", err)
	}

	var joined struct {
		JoinRoom struct {
			ID string
		} `json:"joinRoom"`
	}
	if err := env.Client.Post(`mutation Join($code: String!) { joinRoom(inviteCode: $code) { id } }`, &joined,
		client.AddCookie(guestCookie), client.Var("code", created.CreateRoom.InviteCode)); err != nil {
		t.Fatalf("joinRoom: %v", err)
	}
	if joined.JoinRoom.ID != created.CreateRoom.ID {
		t.Fatalf("guest joined %s, want the host's room %s", joined.JoinRoom.ID, created.CreateRoom.ID)
	}
	return created.CreateRoom.ID
}

// waitForLastSocketClosed blocks until the player's presence row reads as a closed last
// socket, which is the moment the tracker arms its windows.
func waitForLastSocketClosed(t *testing.T, env *queueIntegrationEnv, userID uuid.UUID) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		count, stamped := readPresence(t, env, userID)
		if count == 0 && stamped {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("presence after the socket closed: connection_count=%d stamped=%t, want 0 and stamped", count, stamped)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// ageDisconnect pushes a player's disconnect stamp back past the roster's real window.
//
// The test shortens the TRIGGER's window and ages the stamp for the READING, rather than
// shortening one number for both: the reading's window is store.DefaultRoomRosterPresenceGrace,
// compiled into the members resolver, and a test that could move it would no longer be
// testing the pair of windows the product actually runs. So the trigger fires in a couple
// of seconds and finds a disconnect that is, as far as the database is concerned, a full
// 30s old.
func ageDisconnect(t *testing.T, env *queueIntegrationEnv, userID uuid.UUID, by time.Duration) {
	t.Helper()

	if _, err := env.DB.Exec(
		`UPDATE user_presence SET disconnected_at = NOW() - ($2 * INTERVAL '1 second') WHERE user_id = $1`,
		userID, by.Seconds(),
	); err != nil {
		t.Fatalf("age disconnect: %v", err)
	}
}

// quietPresenceTimers disarms the windows the test's own sockets leave behind when they
// close at cleanup.
//
// The suite's shared tracker registers no expiry at all, "so no grace timer outlives a
// test". These tests have to register one, so they clean up after it instead: a window armed
// by the last socket closing fires a couple of seconds later, by which time the
// environment's database handle is closed, and logs a failure into whichever test is running
// by then — a trail leading somewhere there is no bug.
//
// Registered before the first websocket is opened so it runs after all of them are closed,
// and the pause covers the gap between presence recording the disconnect and the tracker
// arming on it.
func quietPresenceTimers(t *testing.T, env *queueIntegrationEnv, userIDs ...uuid.UUID) {
	t.Helper()

	for _, userID := range userIDs {
		deadline := time.Now().Add(2 * time.Second)
		for {
			if count, stamped := readPresence(t, env, userID); count == 0 && stamped {
				break
			}
			if time.Now().After(deadline) {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		time.Sleep(50 * time.Millisecond)
		env.resolver.Presence.cancelTimer(userID)
	}
}

// awaitRosterReading reads pushes until the room says what this test is waiting to hear
// about one member, and fails if it never does.
//
// It tolerates pushes that say nothing new because a connect edge publishes too (see
// OnRoomRosterPresenceRestored): a watcher opening their own subscription tells the room to
// re-read, so a test that insisted on the very next push would be asserting the order of
// events rather than the reading arriving at all. What must not be tolerated is the reading
// never arriving, and the timeout is that.
func awaitRosterReading(t *testing.T, conn *websocket.Conn, subID string, userID uuid.UUID, want bool, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var last map[string]any
	for time.Now().Before(deadline) {
		pushed := nextRoomUpdatedPayload(t, conn, subID, time.Until(deadline))
		last = pushed
		if rosterReadings(t, pushed)[userID.String()] == want {
			return
		}
	}
	t.Fatalf("no push ever read member %s as disconnected=%t; last roster was %+v", userID, want, last)
}

// TestRoomUpdatedPushesWhenAMemberCrossesTheRosterWindow is the regression this ticket is
// about. Two people in a room, one drops their connection, and nothing else happens: no
// third player joining, no seat changing, no mutation of any kind. Before this, the
// roster's away reading was computed correctly and never left the server, so the other
// player watched a dropped friend stay fully present until something unrelated fired.
//
// The quiet room is the point. Deleting the presence-expiry publish and leaving everything
// else in place still passes every join/leave test in the suite, and fails here.
func TestRoomUpdatedPushesWhenAMemberCrossesTheRosterWindow(t *testing.T) {
	t.Setenv("LOBBY_PUBLIC_URL", "http://localhost:5173")

	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := context.Background()

	// The roster expiry, alone and on a short clock. Alone because the room's 5m
	// eviction and the seat's 30s release are not what this asserts, and either of them
	// publishing would let a broken trigger pass; short because the test should not wait
	// out a window whose real value is pinned by its own test.
	env.resolver.Presence = NewPresenceTracker(env.Store, env.resolver.PubSub,
		2*time.Second, env.resolver.OnRoomRosterPresenceExpired)
	env.rebuildHTTPServer(t)

	host := createTestUser(t, ctx, env, cleaner, "roster-host-"+uuid.NewString()+"@example.com", "Roster Host")
	guest := createTestUser(t, ctx, env, cleaner, "roster-guest-"+uuid.NewString()+"@example.com", "Roster Guest")
	hostBearer, hostCookie := createTestUserSessionForUser(t, env, host.ID)
	guestBearer, guestCookie := createTestUserSessionForUser(t, env, guest.ID)

	t.Cleanup(func() { quietPresenceTimers(t, env, host.ID, guest.ID) })

	roomID := createRoomAndJoin(t, env, hostCookie, guestCookie)

	guestConn := connectGraphQLWS(t, graphQLWSURL(env.Server.URL), "http://localhost:5173", guestBearer)
	guestSub := subscribeRoomUpdated(t, guestConn, roomID)
	nextRoomUpdatedPayload(t, guestConn, guestSub, 5*time.Second)

	hostConn := connectGraphQLWS(t, graphQLWSURL(env.Server.URL), "http://localhost:5173", hostBearer)
	hostSub := subscribeRoomUpdated(t, hostConn, roomID)
	initial := nextRoomUpdatedPayload(t, hostConn, hostSub, 5*time.Second)
	if readings := rosterReadings(t, initial); readings[guest.ID.String()] {
		t.Fatal("the guest reads as away while their socket is open")
	}

	// The whole event: one connection lost, and nothing else.
	if err := guestConn.Close(); err != nil {
		t.Fatalf("close the guest's websocket: %v", err)
	}
	waitForLastSocketClosed(t, env, guest.ID)
	ageDisconnect(t, env, guest.ID, store.DefaultRoomRosterPresenceGrace+time.Second)

	pushed := nextRoomUpdatedPayload(t, hostConn, hostSub, 10*time.Second)
	readings := rosterReadings(t, pushed)
	if !readings[guest.ID.String()] {
		t.Fatalf("the dropped guest still reads as present in the pushed roster: %+v", pushed)
	}
	if readings[host.ID.String()] {
		t.Fatalf("the watching host reads as away in their own pushed roster: %+v", pushed)
	}
}

// TestRoomUpdatedPushesWhenAMemberComesBack is the other edge, and it has to be the same
// mechanism: a reading that only ever arrives late is no better than one that never
// arrives, and undimming by some second route is how the two answers start disagreeing.
//
// Same quiet room — the returning player's socket is the only thing that happens.
func TestRoomUpdatedPushesWhenAMemberComesBack(t *testing.T) {
	t.Setenv("LOBBY_PUBLIC_URL", "http://localhost:5173")

	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := context.Background()

	env.resolver.Presence = NewPresenceTracker(env.Store, env.resolver.PubSub,
		2*time.Second, env.resolver.OnRoomRosterPresenceExpired).
		WithReconnect(env.resolver.OnRoomRosterPresenceRestored)
	env.rebuildHTTPServer(t)

	host := createTestUser(t, ctx, env, cleaner, "roster-back-host-"+uuid.NewString()+"@example.com", "Return Host")
	guest := createTestUser(t, ctx, env, cleaner, "roster-back-guest-"+uuid.NewString()+"@example.com", "Return Guest")
	hostBearer, hostCookie := createTestUserSessionForUser(t, env, host.ID)
	guestBearer, guestCookie := createTestUserSessionForUser(t, env, guest.ID)

	t.Cleanup(func() { quietPresenceTimers(t, env, host.ID, guest.ID) })

	roomID := createRoomAndJoin(t, env, hostCookie, guestCookie)

	guestConn := connectGraphQLWS(t, graphQLWSURL(env.Server.URL), "http://localhost:5173", guestBearer)
	guestSub := subscribeRoomUpdated(t, guestConn, roomID)
	nextRoomUpdatedPayload(t, guestConn, guestSub, 5*time.Second)

	hostConn := connectGraphQLWS(t, graphQLWSURL(env.Server.URL), "http://localhost:5173", hostBearer)
	hostSub := subscribeRoomUpdated(t, hostConn, roomID)
	nextRoomUpdatedPayload(t, hostConn, hostSub, 5*time.Second)

	if err := guestConn.Close(); err != nil {
		t.Fatalf("close the guest's websocket: %v", err)
	}
	waitForLastSocketClosed(t, env, guest.ID)
	ageDisconnect(t, env, guest.ID, store.DefaultRoomRosterPresenceGrace+time.Second)

	awaitRosterReading(t, hostConn, hostSub, guest.ID, true, 10*time.Second)

	// Back on a fresh socket, exactly as a reconnecting client arrives.
	returnedConn := connectGraphQLWS(t, graphQLWSURL(env.Server.URL), "http://localhost:5173", guestBearer)
	returnedSub := subscribeRoomUpdated(t, returnedConn, roomID)
	nextRoomUpdatedPayload(t, returnedConn, returnedSub, 5*time.Second)

	awaitRosterReading(t, hostConn, hostSub, guest.ID, false, 10*time.Second)
}
