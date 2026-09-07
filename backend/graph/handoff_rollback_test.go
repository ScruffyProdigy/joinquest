package graph

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// queueStatusForUser reads a player's row in the demo queue, so a rollback can
// be shown to have put them back rather than merely torn the match down.
func queueStatusForUser(t *testing.T, env *queueIntegrationEnv, userID uuid.UUID) string {
	t.Helper()

	var status string
	err := env.DB.QueryRowContext(context.Background(),
		`SELECT status FROM game_queues WHERE user_id = $1 ORDER BY joined_at DESC LIMIT 1`, userID).Scan(&status)
	if err != nil {
		t.Fatalf("queue status for %s: %v", userID, err)
	}
	return status
}

// TestIdentityRollbackRequeuesTheOtherPlayers is the recovery half of the
// guarantee. One seat without a name must not cost everyone else their match:
// the nameless player is dropped from the queue, everyone else goes back to
// waiting with their original joined_at, and the next real player to arrive
// forms a match with them.
func TestIdentityRollbackRequeuesTheOtherPlayers(t *testing.T) {
	env, cleaner, provisioner := newProvisionEnv(t)
	ctx := context.Background()

	keptID, cookieKept := createIdentifiedPlayer(t, ctx, env, cleaner, "Kept Player")
	droppedID, cookieDropped := createIdentifiedPlayer(t, ctx, env, cleaner, "Dropped Player")

	joinQuery := `mutation Join($id: ID!) { joinQueue(queueId: $id) { queued queuedCount } }`
	vars := map[string]any{"id": demoDefaultQueueID}
	postGraphQL(t, env.Handler, joinQuery, vars, cookieKept)
	postGraphQL(t, env.Handler, joinQuery, vars, cookieDropped)

	// Only a direct write can reach this state; the guards upstream make sure of
	// that. The point is what the handoff does when it happens anyway.
	if _, err := env.DB.ExecContext(ctx, `UPDATE users SET display_name = NULL WHERE id = $1`, droppedID); err != nil {
		t.Fatalf("clear display name: %v", err)
	}
	t.Cleanup(func() {
		if _, err := env.DB.ExecContext(context.Background(),
			`UPDATE users SET display_name = $2 WHERE id = $1`, droppedID, "Dropped Player"); err != nil {
			t.Errorf("restore display name: %v", err)
		}
		clearDemoQueue(t, env.Store)
	})

	flushFormingWorker(t, env, ctx, uuid.MustParse(demoDefaultQueueID))

	if n := provisioner.callCount(); n != 0 {
		t.Fatalf("provisioned a match with a nameless seat: %+v", provisioner.lastCall().Assignment)
	}
	if got := queueStatusForUser(t, env, keptID); got != "waiting" {
		t.Fatalf("the named player should be back in the queue, got status %q", got)
	}
	if got := queueStatusForUser(t, env, droppedID); got != "cancelled" {
		t.Fatalf("the nameless player should have left the queue, got status %q", got)
	}

	// The real test of "back at the front, ready to start": one more player
	// turns up and the waiting player matches with them.
	_, cookieNext := createIdentifiedPlayer(t, ctx, env, cleaner, "Next Player")
	postGraphQL(t, env.Handler, joinQuery, vars, cookieNext)
	flushFormingWorker(t, env, ctx, uuid.MustParse(demoDefaultQueueID))
	waitForProvisionCalls(t, provisioner, 1)

	seats := provisioner.lastCall().Assignment.Seats
	if len(seats) != 2 {
		t.Fatalf("expected a 2-seat match, got %d", len(seats))
	}
	names := make(map[string]struct{}, len(seats))
	for _, seat := range seats {
		if seat.Player == nil || strings.TrimSpace(seat.Player.DisplayName) == "" {
			t.Fatalf("seat %s still has no name", seat.SeatKey)
		}
		names[seat.Player.DisplayName] = struct{}{}
		if seat.LobbyUserID == droppedID.String() {
			t.Fatal("the nameless player was matched again after being dropped")
		}
	}
	for _, want := range []string{"Kept Player", "Next Player"} {
		if _, ok := names[want]; !ok {
			t.Fatalf("expected %q in the re-formed match, got %v", want, names)
		}
	}
}
