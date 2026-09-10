package graph

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// readPresence returns the user's socket count and disconnect stamp, or (0, false) if
// they have no presence row at all — which is what a subscription that never called
// Track leaves behind.
func readPresence(t *testing.T, env *queueIntegrationEnv, userID uuid.UUID) (int, bool) {
	t.Helper()
	var count int
	var stamp sql.NullTime
	err := env.DB.QueryRow(
		`SELECT connection_count, disconnected_at FROM user_presence WHERE user_id = $1`, userID,
	).Scan(&count, &stamp)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false
	}
	if err != nil {
		t.Fatalf("read presence: %v", err)
	}
	return count, stamp.Valid
}

// The six subscription resolvers each call Presence.Track and defer its release, and
// nothing else in the suite reaches those lines — delete one and everything still
// passes. This runs a real websocket through QueueUpdated and watches user_presence
// follow it up and back down, so the wiring is covered end to end rather than at the
// tracker's own seam.
func TestSubscriptionOpenAndCloseMovesUserPresence(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := context.Background()

	user := createTestUser(t, ctx, env, cleaner, "presence-ws-"+uuid.NewString()+"@example.com", "Presence Tester")
	bearer, _ := createTestUserSessionForUser(t, env, user.ID)

	conn := connectGraphQLWS(t, graphQLWSURL(env.Server.URL), "http://localhost:5173", bearer)
	subID := subscribeQueueUpdated(t, conn, demoDefaultQueueID)
	// The initial payload is the resolver's own proof of life: it is sent from the
	// same goroutine that ran Track.
	nextQueueUpdatePayload(t, conn, subID, 5*time.Second)

	count, disconnected := readPresence(t, env, user.ID)
	if count != 1 {
		t.Fatalf("connection_count while the socket is open: got %d, want 1", count)
	}
	if disconnected {
		t.Fatal("a connected player is carrying a disconnect stamp")
	}

	// Closing the socket is the departure signal this whole ticket is built on.
	if err := conn.Close(); err != nil {
		t.Fatalf("close websocket: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		count, disconnected = readPresence(t, env, user.ID)
		if count == 0 && disconnected {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("presence after the socket closed: connection_count=%d stamped=%t, want 0 and stamped", count, disconnected)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
