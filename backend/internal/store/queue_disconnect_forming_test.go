package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/gameclient"
	"github.com/scruffyprodigy/joinquest/internal/lfg/partytree"
)

// The store tests never ran ReconcileFormingModeQueue, so no forming_match_assignments
// row ever existed in them — which is exactly why removing a waiting player without
// releasing their seat got through review. Every test in this file starts by putting a
// real assignment on the board.

func resetDemoQueue(t *testing.T, st *Store, ctx context.Context) {
	t.Helper()
	if err := st.ResetModeQueueIntegrationState(ctx, DemoDefaultQueueID); err != nil {
		t.Fatalf("reset demo queue: %v", err)
	}
}

func assignedSeatCount(t *testing.T, st *Store, ctx context.Context, userID uuid.UUID) int {
	t.Helper()
	var n int
	if err := st.db.QueryRowContext(ctx,
		`SELECT count(*) FROM forming_match_assignments WHERE user_id = $1`, userID,
	).Scan(&n); err != nil {
		t.Fatalf("count assignments: %v", err)
	}
	return n
}

// joinAndPlace puts the user in the demo queue and runs the forming worker's step, so
// they hold a real seat on the filling match — the state a live player is in ~25ms
// after joining.
func joinAndPlace(t *testing.T, st *Store, ctx context.Context, userID uuid.UUID) {
	t.Helper()
	if _, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, userID, "", nil); err != nil {
		t.Fatalf("join: %v", err)
	}
	rec := mustReconcileForming(t, st, ctx, DemoDefaultQueueID)
	if rec.Fired {
		t.Fatal("fixture fired a match with a single player, so nothing here is on a filling match")
	}
	if n := assignedSeatCount(t, st, ctx, userID); n != 1 {
		t.Fatalf("fixture never placed the player on the forming match (%d seats), so this test proves nothing", n)
	}
}

// newFourSeatQueue registers a game whose match needs four players, so a two-person
// party can be placed on the forming map without the match firing underneath the test.
func newFourSeatQueue(t *testing.T, st *Store, cleaner *TestCleaner, ctx context.Context) uuid.UUID {
	t.Helper()
	slug := "disconnect-forming-" + uuid.NewString()
	manifest := &gameclient.Manifest{
		Modes: []gameclient.ModeManifest{{
			Key:          "quad",
			DisplayName:  "Quad",
			SeatTemplate: json.RawMessage(`{"count":4}`),
		}},
		Status:     gameclient.StatusResponse{Game: "Quad", Version: "1.0.0"},
		ETag:       `"quad"`,
		RawJSON:    []byte(`{"modes":[{"key":"quad"}]}`),
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
	return queues[0].ID
}

// joinPartyAndPlace queues two players as one party and runs the forming worker's step,
// leaving both on the filling match.
func joinPartyAndPlace(t *testing.T, st *Store, ctx context.Context, queueID, leader, member uuid.UUID) {
	t.Helper()
	if _, err := st.JoinModeQueue(ctx, queueID, leader, "", &JoinPartyInput{
		Tree: partytree.Node{Members: []string{leader.String(), member.String()}},
		Members: []JoinPartyMemberInput{
			{UserID: leader, QueuePath: ""},
			{UserID: member, QueuePath: ""},
		},
	}); err != nil {
		t.Fatalf("party join: %v", err)
	}
	rec := mustReconcileForming(t, st, ctx, queueID)
	if rec.Fired {
		t.Fatal("fixture fired a match with half a lobby, so nothing here is on a filling match")
	}
	for _, userID := range []uuid.UUID{leader, member} {
		if n := assignedSeatCount(t, st, ctx, userID); n != 1 {
			t.Fatalf("fixture placed a party member on %d seats, want 1", n)
		}
	}
}

// disconnectNow drops the user's only socket and returns the stamp the window runs from.
func disconnectNow(t *testing.T, st *Store, ctx context.Context, userID uuid.UUID) time.Time {
	t.Helper()
	if _, err := st.PresenceConnected(ctx, userID); err != nil {
		t.Fatalf("connect: %v", err)
	}
	dropped, err := st.PresenceDisconnected(ctx, userID)
	if err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	if dropped.DisconnectedAt == nil {
		t.Fatal("disconnect did not stamp")
	}
	return *dropped.DisconnectedAt
}

// twoFreshPlayersStillMatch is the end-to-end statement of the wedge: whatever the
// removal did, the mode queue must still be able to form a match afterwards.
func twoFreshPlayersStillMatch(t *testing.T, st *Store, cleaner *TestCleaner, ctx context.Context) {
	t.Helper()
	for _, userID := range []uuid.UUID{
		newPresenceUser(t, st, cleaner, ctx),
		newPresenceUser(t, st, cleaner, ctx),
	} {
		if _, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, userID, "", nil); err != nil {
			t.Fatalf("join: %v", err)
		}
	}
	rec := mustReconcileForming(t, st, ctx, DemoDefaultQueueID)
	if !rec.Fired || rec.SessionID == nil {
		t.Fatalf("the mode queue is wedged: two waiting players did not form a match, got %+v", rec)
	}
}

// Cancelling a waiting row while its forming-match seat still names the player hands
// fireFormingMatchTx an assigned user with no waiting entry. Nothing expires a filling
// forming match, so that rolls back every reconcile of the mode queue for good: no
// player in this mode is ever matched again.
//
// The evicted player is in a two-person party on purpose. A solo player's seat is
// released by accident — the post-commit party reconcile finds their one-member party
// empty and cancels it, and cancelling a party releases its members' seats — so a solo
// fixture would pass with the release deleted. A party with a member still waiting is
// not stale, so nothing incidental covers for a missing release.
func TestEvictionReleasesTheFormingSeatSoTheQueueDoesNotWedge(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	queueID := newFourSeatQueue(t, st, cleaner, ctx)

	dropping := newPresenceUser(t, st, cleaner, ctx)
	staying := newPresenceUser(t, st, cleaner, ctx)
	joinPartyAndPlace(t, st, ctx, queueID, dropping, staying)

	stamp := disconnectNow(t, st, ctx, dropping)
	got, err := st.EvictDisconnectedWaitingEntry(ctx, dropping, stamp)
	if err != nil {
		t.Fatalf("EvictDisconnectedWaitingEntry: %v", err)
	}
	if !got.Acted {
		t.Fatal("expected the eviction to act")
	}
	if n := assignedSeatCount(t, st, ctx, dropping); n != 0 {
		t.Fatalf("the evicted player still holds %d forming seats", n)
	}
	if n := waitingRowCount(t, st, ctx, staying); n != 1 {
		t.Fatalf("their party member lost their place too: %d waiting rows", n)
	}

	// Three more players fill the four-seat match. It has to fire on this reconcile:
	// a stale seat would make the first pass vacate and decline instead.
	for i := 0; i < 3; i++ {
		filler := newPresenceUser(t, st, cleaner, ctx)
		if _, err := st.JoinModeQueue(ctx, queueID, filler, "", nil); err != nil {
			t.Fatalf("join filler: %v", err)
		}
	}
	rec := mustReconcileForming(t, st, ctx, queueID)
	if !rec.Fired || rec.SessionID == nil {
		t.Fatalf("the mode queue is wedged: a full pool did not form a match, got %+v", rec)
	}
}

// The sweep is the dead-pod path, which is precisely where the filling forming match
// outlives the process that built it — so it has the same obligation as the timer.
func TestSweepReleasesTheFormingSeatSoTheQueueDoesNotWedge(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	resetDemoQueue(t, st, ctx)

	userID := newPresenceUser(t, st, cleaner, ctx)
	joinAndPlace(t, st, ctx, userID)
	disconnectNow(t, st, ctx, userID)

	// Their pod died with the timer in it; the window elapsed with nobody watching.
	if _, err := st.db.ExecContext(ctx, `
		UPDATE user_presence SET disconnected_at = NOW() - INTERVAL '10 minutes' WHERE user_id = $1
	`, userID); err != nil {
		t.Fatalf("age the disconnect: %v", err)
	}

	result, err := st.SweepStaleDisconnectedQueues(ctx, DefaultQueueDisconnectGrace)
	if err != nil {
		t.Fatalf("SweepStaleDisconnectedQueues: %v", err)
	}
	if result.Cancelled != 1 {
		t.Fatalf("sweep cancelled %d rows, want 1", result.Cancelled)
	}
	if n := assignedSeatCount(t, st, ctx, userID); n != 0 {
		t.Fatalf("the swept player still holds %d forming seats", n)
	}

	twoFreshPlayersStillMatch(t, st, cleaner, ctx)
}

// Belt and braces for the same wedge, from the other end: even if some future removal
// path forgets to release the seat, firing must vacate it and decline rather than
// erroring, because an error here is permanent.
func TestFiringVacatesASeatWhoseWaitingRowIsGone(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	resetDemoQueue(t, st, ctx)

	orphan := newPresenceUser(t, st, cleaner, ctx)
	joinAndPlace(t, st, ctx, orphan)

	// A removal path that cancels the row and forgets the seat.
	if _, err := st.db.ExecContext(ctx, `
		UPDATE game_queues SET status = 'cancelled' WHERE user_id = $1 AND status = 'waiting'
	`, orphan); err != nil {
		t.Fatalf("cancel the row behind the seat's back: %v", err)
	}

	for _, userID := range []uuid.UUID{
		newPresenceUser(t, st, cleaner, ctx),
		newPresenceUser(t, st, cleaner, ctx),
	} {
		if _, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, userID, "", nil); err != nil {
			t.Fatalf("join: %v", err)
		}
	}

	declined := mustReconcileForming(t, st, ctx, DemoDefaultQueueID)
	if declined.Fired {
		t.Fatal("fired a match containing a player who is no longer queued")
	}
	if n := assignedSeatCount(t, st, ctx, orphan); n != 0 {
		t.Fatalf("the orphaned seat was not vacated (%d seats), so the next reconcile wedges too", n)
	}

	refilled := mustReconcileForming(t, st, ctx, DemoDefaultQueueID)
	if !refilled.Fired || refilled.SessionID == nil {
		t.Fatalf("the vacated seat was never refilled, got %+v", refilled)
	}
}

// Deprioritisation has to start at the disconnect, not at expiry: the player is on the
// forming map within ~25ms of joining, and nothing else takes them off it, so the match
// otherwise fires with their phone in their pocket.
func TestReleaseFormingSlotsForDisconnectedUserVacatesOnlyThatPlayer(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	resetDemoQueue(t, st, ctx)

	dropping := newPresenceUser(t, st, cleaner, ctx)
	joinAndPlace(t, st, ctx, dropping)

	// A second player on the same filling match, placed directly rather than through
	// a reconcile: two waiting players in the demo queue fire immediately, and this
	// test needs a match still filling.
	staying := newPresenceUser(t, st, cleaner, ctx)
	if _, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, staying, "", nil); err != nil {
		t.Fatalf("join staying: %v", err)
	}
	if _, err := st.db.ExecContext(ctx, `
		UPDATE forming_match_assignments
		SET user_id = $1
		WHERE id = (
		    SELECT fma.id
		    FROM forming_match_assignments fma
		    JOIN forming_matches fm ON fm.id = fma.forming_match_id
		    WHERE fm.mode_queue_id = $2 AND fm.status = 'filling' AND fma.user_id IS NULL
		    ORDER BY fma.seat_key
		    LIMIT 1
		)
	`, staying, DemoDefaultQueueID); err != nil {
		t.Fatalf("seat the staying player: %v", err)
	}
	if n := assignedSeatCount(t, st, ctx, staying); n != 1 {
		t.Fatalf("fixture seated the staying player %d times, want 1", n)
	}

	stamp := disconnectNow(t, st, ctx, dropping)

	// A stamp this disconnect did not write belongs to a window somebody else owns.
	if err := st.ReleaseFormingSlotsForDisconnectedUser(ctx, dropping, stamp.Add(-time.Minute)); err != nil {
		t.Fatalf("release on a stale stamp: %v", err)
	}
	if n := assignedSeatCount(t, st, ctx, dropping); n != 1 {
		t.Fatal("a stale stamp released a seat it does not own")
	}

	if err := st.ReleaseFormingSlotsForDisconnectedUser(ctx, dropping, stamp); err != nil {
		t.Fatalf("ReleaseFormingSlotsForDisconnectedUser: %v", err)
	}
	if n := assignedSeatCount(t, st, ctx, dropping); n != 0 {
		t.Fatalf("the disconnected player still holds %d forming seats", n)
	}
	if n := assignedSeatCount(t, st, ctx, staying); n != 1 {
		t.Fatalf("a connected player on the same match lost their seat (%d seats)", n)
	}

	// Still queued, still counted: deprioritised, not removed.
	if n := waitingRowCount(t, st, ctx, dropping); n != 1 {
		t.Fatalf("the disconnected player lost their place in the queue: %d waiting rows", n)
	}
}
