package graph

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

const setDocumentVisibilityMutation = `
  mutation SetDocumentVisibility($documentId: ID!, $visible: Boolean!) {
    setDocumentVisibility(documentId: $documentId, visible: $visible)
  }
`

func readIsAway(t *testing.T, env *queueIntegrationEnv, userID uuid.UUID) bool {
	t.Helper()
	away, err := env.Store.UserIsAway(context.Background(), userID)
	if err != nil {
		t.Fatalf("UserIsAway: %v", err)
	}
	return away
}

// The mutation is the only way the server ever learns a tab went to the
// background, so this walks the whole path: a live socket makes the player
// connected, the report makes them away, and reporting visible again brings them
// back. Without the socket half the report means nothing, which is the point of
// asserting through UserIsAway rather than reading the table.
func TestSetDocumentVisibilityDrivesAway(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := context.Background()

	user := createTestUser(t, ctx, env, cleaner, "visibility-"+uuid.NewString()+"@example.com", "Visibility Tester")
	bearer, cookie := createTestUserSessionForUser(t, env, user.ID)

	conn := connectGraphQLWS(t, graphQLWSURL(env.Server.URL), "http://localhost:5173", bearer)
	defer conn.Close()
	subID := subscribeQueueUpdated(t, conn, demoDefaultQueueID)
	nextQueueUpdatePayload(t, conn, subID, 5*time.Second)

	if readIsAway(t, env, user.ID) {
		t.Fatal("player read as away before reporting anything; unknown must mean present")
	}

	documentID := uuid.NewString()
	body := postGraphQL(t, env.Handler, setDocumentVisibilityMutation, map[string]any{
		"documentId": documentID,
		"visible":    false,
	}, cookie)
	var hidden struct {
		Data struct {
			SetDocumentVisibility bool `json:"setDocumentVisibility"`
		} `json:"data"`
		Errors []map[string]any `json:"errors"`
	}
	if err := json.Unmarshal(body, &hidden); err != nil {
		t.Fatalf("decode hidden response: %v (%s)", err, body)
	}
	if len(hidden.Errors) > 0 {
		t.Fatalf("setDocumentVisibility(false) returned errors: %v", hidden.Errors)
	}
	if !readIsAway(t, env, user.ID) {
		t.Fatal("player with a live socket and a hidden document did not read as away")
	}

	postGraphQL(t, env.Handler, setDocumentVisibilityMutation, map[string]any{
		"documentId": documentID,
		"visible":    true,
	}, cookie)
	if readIsAway(t, env, user.ID) {
		t.Fatal("player who reported visible again still read as away")
	}
}

// Visibility is a per-user fact, so an unauthenticated caller has no document to
// report for and must be refused rather than writing a row keyed to nobody.
func TestSetDocumentVisibilityRequiresAuth(t *testing.T) {
	env := newQueueIntegrationEnv(t)

	body := postGraphQL(t, env.Handler, setDocumentVisibilityMutation, map[string]any{
		"documentId": uuid.NewString(),
		"visible":    false,
	})
	var res struct {
		Errors []map[string]any `json:"errors"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatalf("decode response: %v (%s)", err, body)
	}
	if len(res.Errors) == 0 {
		t.Fatal("unauthenticated setDocumentVisibility succeeded, want an error")
	}
}
