package graph

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// isWSReadTimeout reports whether a websocket read failed because the read deadline these
// helpers set on the connection expired, rather than because the peer went away.
func isWSReadTimeout(err error) bool {
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// subscribeMatchResultUpdated starts a matchResultUpdated subscription over an already
// connection_init'd websocket and returns its operation id.
func subscribeMatchResultUpdated(t *testing.T, conn *websocket.Conn, matchID string) string {
	t.Helper()

	subID := "sub-" + matchID
	query := fmt.Sprintf(`subscription { matchResultUpdated(matchId: %q) { matchId complete reported status participants { user { id } regroup winner } } }`, matchID)
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

// nextMatchResultUpdatedPayload waits for the next "data" message on subID, forwarding
// keep-alives, and fails the test on "error"/"complete" so an authorization refusal or a
// resolver panic surfaces immediately rather than as a timeout.
func nextMatchResultUpdatedPayload(t *testing.T, conn *websocket.Conn, subID string, timeout time.Duration) map[string]any {
	t.Helper()

	deadline := time.Now().Add(timeout)
	// Without a read deadline on the connection the loop condition below is decorative:
	// a subscription that silently stops pushing parks in ReadJSON until the whole test
	// binary times out, instead of failing here with a useful message.
	if err := conn.SetReadDeadline(deadline); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	for time.Now().Before(deadline) {
		msg, err := readGraphQLWS(conn)
		if err != nil {
			if isWSReadTimeout(err) {
				t.Fatalf("timed out waiting for matchResultUpdated event")
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
		update, _ := data["matchResultUpdated"].(map[string]any)
		if update != nil {
			return update
		}
	}
	t.Fatalf("timed out waiting for matchResultUpdated event")
	return nil
}

// waitForMatchSubscriptionError waits for the message gqlgen's graphql-ws transport sends
// when a subscription's resolver function returns an error before ever producing a
// channel: a "data" message whose payload carries a GraphQL "errors" array and a null
// "matchResultUpdated", not a resolved roster. It fails the test if an actual result comes
// back instead.
func waitForMatchSubscriptionError(t *testing.T, conn *websocket.Conn, subID string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	// See nextMatchResultUpdatedPayload: the deadline has to be on the connection, not
	// only on the loop, or a silent subscription hangs the test binary instead of
	// failing here.
	if err := conn.SetReadDeadline(deadline); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	for time.Now().Before(deadline) {
		msg, err := readGraphQLWS(conn)
		if err != nil {
			if isWSReadTimeout(err) {
				t.Fatalf("timed out waiting for matchResultUpdated to be refused")
			}
			t.Fatalf("read websocket: %v", err)
		}
		typ, _ := msg["type"].(string)
		switch typ {
		case "ping":
			_ = writeGraphQLWS(conn, map[string]any{"type": "pong"})
			continue
		case "ka":
			continue
		}
		if msg["id"] != subID {
			continue
		}
		switch typ {
		case "error":
			return
		case "data":
			payload, _ := msg["payload"].(map[string]any)
			if errs, ok := payload["errors"].([]any); ok && len(errs) > 0 {
				return
			}
			t.Fatalf("expected the subscription to be refused, got data: %v", msg)
		case "complete":
			t.Fatalf("subscription completed without an error message: %v", msg)
		}
	}
	t.Fatalf("timed out waiting for matchResultUpdated to be refused")
}

// TestMatchResultUpdatedRequiresParticipant is the live-subscription counterpart of
// TestMatchResultVisibleOnlyToParticipants: a match id travels in a player-editable return
// URL, so a signed-in stranger who knows or guesses one must not be able to open a live
// feed of another group's standings and roster.
func TestMatchResultUpdatedRequiresParticipant(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	match := seedFinishedMatch(t, env, cleaner)

	outsiderBearer, _ := createTestUserSession(t, context.Background(), env, cleaner)
	conn := connectGraphQLWS(t, graphQLWSURL(env.Server.URL), "http://localhost:5173", outsiderBearer)
	subID := subscribeMatchResultUpdated(t, conn, match.sessionID.String())

	waitForMatchSubscriptionError(t, conn, subID, 5*time.Second)
}

// TestMatchResultUpdatedPushesInitialResult confirms a participant's subscription opens
// and pushes the current standings as its first event, exercising the same
// requireMatchParticipant + loadMatchResultModel path the query resolver uses.
func TestMatchResultUpdatedPushesInitialResult(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	match := seedFinishedMatch(t, env, cleaner)

	tokenA, err := env.Signer.SignUserToken(match.userA.ID, time.Hour)
	if err != nil {
		t.Fatalf("SignUserToken: %v", err)
	}
	conn := connectGraphQLWS(t, graphQLWSURL(env.Server.URL), "http://localhost:5173", "Bearer "+tokenA)
	subID := subscribeMatchResultUpdated(t, conn, match.sessionID.String())

	initial := nextMatchResultUpdatedPayload(t, conn, subID, 5*time.Second)
	if initial["matchId"] != match.sessionID.String() {
		t.Fatalf("matchId = %v, want %v", initial["matchId"], match.sessionID)
	}
	if initial["complete"] != true {
		t.Fatalf("expected complete=true on the initial push, got %+v", initial)
	}
	if initial["status"] != "COMPLETED" {
		t.Fatalf("expected status=COMPLETED on the initial push, got %+v", initial)
	}
}

// TestMatchResultUpdatedPushesOnRegroup exercises the actual wiring this task adds: a
// player still watching the results screen sees another player's regroup answer land
// live, without polling. It is the end-to-end complement to
// TestMatchEventRoundTrip (marshal/unmarshal in isolation) and pins that PlayAgain's
// publishMatchEvent call actually reaches a subscriber.
func TestMatchResultUpdatedPushesOnRegroup(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	match := seedFinishedMatch(t, env, cleaner)

	tokenA, err := env.Signer.SignUserToken(match.userA.ID, time.Hour)
	if err != nil {
		t.Fatalf("SignUserToken: %v", err)
	}
	conn := connectGraphQLWS(t, graphQLWSURL(env.Server.URL), "http://localhost:5173", "Bearer "+tokenA)
	subID := subscribeMatchResultUpdated(t, conn, match.sessionID.String())

	// Drain the initial push before triggering the change under test.
	nextMatchResultUpdatedPayload(t, conn, subID, 5*time.Second)

	playResp := playAgain(t, env, match.sessionID, match.cookieB)
	if len(playResp.Errors) > 0 {
		t.Fatalf("playAgain B: %+v", playResp.Errors)
	}

	updated := nextMatchResultUpdatedPayload(t, conn, subID, 5*time.Second)
	participants, _ := updated["participants"].([]any)
	found := false
	for _, raw := range participants {
		p, _ := raw.(map[string]any)
		user, _ := p["user"].(map[string]any)
		if user["id"] == match.userB.ID.String() {
			found = true
			if p["regroup"] != "IN" {
				t.Fatalf("player B regroup = %v, want IN after playAgain", p["regroup"])
			}
		}
	}
	if !found {
		t.Fatalf("player B missing from pushed participants: %+v", updated)
	}
}

// TestMatchResultUpdatedPushesResultAfterSessionCompleted pins the ordering the real
// lifecycle actually produces: the game reports every player finished — which completes
// the session inside reportPlayerFinished — and only then reports the match result. The
// result is recorded either way, so a subscriber still parked on the results screen must
// receive it and flip to the final standings. Before the fix, ReportMatchResult returned
// early on CompleteSession's ErrNotFound (the session was already completed) and published
// nothing, leaving every subscriber on winner:false / status:null until they navigated
// away and refetched.
func TestMatchResultUpdatedPushesResultAfterSessionCompleted(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	match := seedActiveMatch(t, env, cleaner)

	// Both players finish: the second report drives remaining == 0 and completes the
	// session, so the later reportMatchResult hits CompleteSession's ErrNotFound branch.
	reportPlayerFinished(t, env, match.sessionID, match.userA.ID, 1)
	reportPlayerFinished(t, env, match.sessionID, match.userB.ID, 2)

	tokenA, err := env.Signer.SignUserToken(match.userA.ID, time.Hour)
	if err != nil {
		t.Fatalf("SignUserToken: %v", err)
	}
	conn := connectGraphQLWS(t, graphQLWSURL(env.Server.URL), "http://localhost:5173", "Bearer "+tokenA)
	subID := subscribeMatchResultUpdated(t, conn, match.sessionID.String())

	initial := nextMatchResultUpdatedPayload(t, conn, subID, 5*time.Second)
	if initial["reported"] != false {
		t.Fatalf("expected reported=false before the result lands, got %+v", initial)
	}

	reportMatchResult(t, env, match.sessionID, "COMPLETED", match.userA.ID.String())

	updated := nextMatchResultUpdatedPayload(t, conn, subID, 5*time.Second)
	if updated["reported"] != true {
		t.Fatalf("expected reported=true on the pushed result, got %+v", updated)
	}
	if updated["status"] != "COMPLETED" {
		t.Fatalf("status = %v, want COMPLETED: %+v", updated["status"], updated)
	}
	participants, _ := updated["participants"].([]any)
	sawWinner := false
	for _, raw := range participants {
		p, _ := raw.(map[string]any)
		user, _ := p["user"].(map[string]any)
		if user["id"] == match.userA.ID.String() && p["winner"] == true {
			sawWinner = true
		}
	}
	if !sawWinner {
		t.Fatalf("expected player A flagged as the winner in the pushed result: %+v", updated)
	}
}
