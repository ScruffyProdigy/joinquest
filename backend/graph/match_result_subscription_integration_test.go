package graph

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// subscribeMatchResultUpdated starts a matchResultUpdated subscription over an already
// connection_init'd websocket and returns its operation id.
func subscribeMatchResultUpdated(t *testing.T, conn *websocket.Conn, matchID string) string {
	t.Helper()

	subID := "sub-" + matchID
	query := fmt.Sprintf(`subscription { matchResultUpdated(matchId: %q) { matchId complete status participants { user { id } regroup } } }`, matchID)
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
	for time.Now().Before(deadline) {
		msg, err := readGraphQLWS(conn)
		if err != nil {
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
	for time.Now().Before(deadline) {
		msg, err := readGraphQLWS(conn)
		if err != nil {
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
