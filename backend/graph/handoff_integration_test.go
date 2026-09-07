package graph

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/auth"
	"github.com/scruffyprodigy/joinquest/internal/gameclient"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

type provisionCall struct {
	LobbyID    string
	Lobby      gameclient.LobbyInfo
	Assignment gameclient.Assignment
}

type syncProvisioner struct {
	mu    sync.Mutex
	calls []provisionCall
}

func (p *syncProvisioner) ProvisionMatch(_ context.Context, req gameclient.ProvisionRequest) (gameclient.ProvisionResult, error) {
	p.mu.Lock()
	p.calls = append(p.calls, provisionCall{LobbyID: req.LobbyID, Lobby: req.Lobby, Assignment: req.Assignment})
	p.mu.Unlock()

	launchURLs := make(map[string]string, len(req.Assignment.Seats))
	for _, seat := range req.Assignment.Seats {
		launchURLs[seat.LobbyUserID] = fmt.Sprintf(
			"http://localhost:5174/?match=%s&seat=%s",
			req.Assignment.ExternalMatchID,
			seat.SeatKey,
		)
	}
	return gameclient.ProvisionResult{LaunchURLs: launchURLs}, nil
}

func (p *syncProvisioner) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.calls)
}

func (p *syncProvisioner) lastCall() provisionCall {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.calls) == 0 {
		return provisionCall{}
	}
	return p.calls[len(p.calls)-1]
}

func TestJoinQueueProvisionsMatchOnGameServer(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := context.Background()
	clearDemoQueue(t, env.Store)

	t.Setenv("LOBBY_ISSUER_URL", "http://localhost:8080")
	t.Setenv("LOBBY_PUBLIC_URL", "http://localhost:5173")
	t.Setenv("LOBBY_GAME_TOKEN_PEPPER", "test-pepper")

	provisioner := &syncProvisioner{}
	env.resolverWithProvisioner(t, provisioner)

	_, cookieA := createTestUserSession(t, ctx, env, cleaner)
	_, cookieB := createTestUserSession(t, ctx, env, cleaner)

	joinQuery := `mutation Join($id: ID!) { joinQueue(queueId: $id) { queued queuedCount } }`
	vars := map[string]any{"id": demoDefaultQueueID}

	postGraphQL(t, env.Handler, joinQuery, vars, cookieA)
	postGraphQL(t, env.Handler, joinQuery, vars, cookieB)
	flushFormingWorker(t, env, ctx, uuid.MustParse(demoDefaultQueueID))
	waitForProvisionCalls(t, provisioner, 1)

	if n := len(provisioner.calls); n != 1 {
		t.Fatalf("expected exactly 1 provision call, got %d", n)
	}
	call := provisioner.lastCall()
	a := call.Assignment
	if call.LobbyID != "http://localhost:8080" {
		t.Fatalf("lobbyId = %q, want http://localhost:8080", call.LobbyID)
	}
	if call.Lobby.ReturnURL != "http://localhost:5173/return" {
		t.Fatalf("lobby.returnUrl = %q", call.Lobby.ReturnURL)
	}
	if call.Lobby.GraphqlURL != "http://localhost:8080/graphql" {
		t.Fatalf("lobby.graphqlUrl = %q", call.Lobby.GraphqlURL)
	}
	gameID := uuid.MustParse(store.DemoPrimaryGameIDStr)
	wantToken, err := auth.FormatGameServiceToken(gameID)
	if err != nil {
		t.Fatalf("FormatGameServiceToken: %v", err)
	}
	if call.Lobby.ServiceToken != wantToken {
		t.Fatalf("lobby.serviceToken = %q, want %q", call.Lobby.ServiceToken, wantToken)
	}
	if a.ExternalMatchID == "" || len(a.Seats) != 2 {
		t.Fatalf("expected provisioned duel with 2 seats, got %+v", a)
	}
	if a.GameMode != "duel" {
		t.Fatalf("gameMode = %q, want duel", a.GameMode)
	}
	seatKeys := map[string]struct{}{}
	for _, seat := range a.Seats {
		seatKeys[seat.SeatKey] = struct{}{}
	}
	if len(seatKeys) != 2 {
		t.Fatalf("expected 2 unique seat keys, got %+v", a.Seats)
	}
	for _, want := range []string{"1", "2"} {
		if _, ok := seatKeys[want]; !ok {
			t.Fatalf("expected manifest seat key %q, got %+v", want, a.Seats)
		}
	}
}

func TestJoinQueueProvisionsServiceTokenWhenConfigured(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := context.Background()
	clearDemoQueue(t, env.Store)

	t.Setenv("LOBBY_ISSUER_URL", "http://localhost:8080")
	t.Setenv("LOBBY_PUBLIC_URL", "http://localhost:5173")
	t.Setenv("LOBBY_GAME_TOKEN_PEPPER", "scoped-pepper")

	provisioner := &syncProvisioner{}
	env.resolverWithProvisioner(t, provisioner)

	_, cookieA := createTestUserSession(t, ctx, env, cleaner)
	_, cookieB := createTestUserSession(t, ctx, env, cleaner)

	joinQuery := `mutation Join($id: ID!) { joinQueue(queueId: $id) { queued queuedCount } }`
	vars := map[string]any{"id": demoDefaultQueueID}

	postGraphQL(t, env.Handler, joinQuery, vars, cookieA)
	postGraphQL(t, env.Handler, joinQuery, vars, cookieB)
	flushFormingWorker(t, env, ctx, uuid.MustParse(demoDefaultQueueID))
	waitForProvisionCalls(t, provisioner, 1)

	call := provisioner.lastCall()
	wantToken, err := auth.FormatGameServiceToken(uuid.MustParse(store.DemoPrimaryGameIDStr))
	if err != nil {
		t.Fatalf("FormatGameServiceToken: %v", err)
	}
	if call.Lobby.ServiceToken != wantToken {
		t.Fatalf("lobby.serviceToken = %q, want scoped per-game token", call.Lobby.ServiceToken)
	}
}

// createIdentifiedPlayer builds a fully identified player and hands back the id
// alongside the cookie, so a test can reach into the row afterwards.
func createIdentifiedPlayer(
	t *testing.T,
	ctx context.Context,
	env *queueIntegrationEnv,
	cleaner *store.TestCleaner,
	name string,
) (uuid.UUID, *http.Cookie) {
	t.Helper()

	user, err := env.Store.CreateUser(ctx, store.CreateUserParams{
		Email:       "handoff-" + uuid.NewString() + "@example.com",
		DisplayName: name,
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(user.ID)
	if _, err := env.Store.UpdateUserProfile(ctx, user.ID, name, testAvatarKey, "http://localhost:5173"); err != nil {
		t.Fatalf("UpdateUserProfile: %v", err)
	}

	_, cookie := createTestUserSessionForUser(t, env, user.ID)
	return user.ID, cookie
}

// queueDemoDuel puts two players into the demo queue and lets the match form.
func queueDemoDuel(t *testing.T, ctx context.Context, env *queueIntegrationEnv, cookies ...*http.Cookie) {
	t.Helper()

	joinQuery := `mutation Join($id: ID!) { joinQueue(queueId: $id) { queued queuedCount } }`
	vars := map[string]any{"id": demoDefaultQueueID}
	for _, cookie := range cookies {
		postGraphQL(t, env.Handler, joinQuery, vars, cookie)
	}
	flushFormingWorker(t, env, ctx, uuid.MustParse(demoDefaultQueueID))
}

func newProvisionEnv(t *testing.T) (*queueIntegrationEnv, *store.TestCleaner, *syncProvisioner) {
	t.Helper()

	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	clearDemoQueue(t, env.Store)

	t.Setenv("LOBBY_ISSUER_URL", "http://localhost:8080")
	t.Setenv("LOBBY_PUBLIC_URL", "http://localhost:5173")
	t.Setenv("LOBBY_GAME_TOKEN_PEPPER", "test-pepper")

	provisioner := &syncProvisioner{}
	env.resolverWithProvisioner(t, provisioner)
	return env, cleaner, provisioner
}

// TestProvisionedSeatsCarryChosenDisplayName is the guarantee JQ-124 is about,
// asserted where the payload leaves Lobby: every seat a game receives names the
// player, and names them the thing they actually picked.
func TestProvisionedSeatsCarryChosenDisplayName(t *testing.T) {
	env, cleaner, provisioner := newProvisionEnv(t)
	ctx := context.Background()

	_, cookieA := createIdentifiedPlayer(t, ctx, env, cleaner, "Ada Lovelace")
	_, cookieB := createIdentifiedPlayer(t, ctx, env, cleaner, "Grace Hopper")

	queueDemoDuel(t, ctx, env, cookieA, cookieB)
	waitForProvisionCalls(t, provisioner, 1)

	seats := provisioner.lastCall().Assignment.Seats
	if len(seats) != 2 {
		t.Fatalf("expected 2 seats, got %d", len(seats))
	}

	got := make(map[string]struct{}, len(seats))
	for _, seat := range seats {
		if seat.Player == nil {
			t.Fatalf("seat %s carries no player block", seat.SeatKey)
		}
		if strings.TrimSpace(seat.Player.DisplayName) == "" {
			t.Fatalf("seat %s carries an empty displayName", seat.SeatKey)
		}
		got[seat.Player.DisplayName] = struct{}{}
	}
	for _, want := range []string{"Ada Lovelace", "Grace Hopper"} {
		if _, ok := got[want]; !ok {
			t.Fatalf("expected the chosen name %q in the payload, got %v", want, got)
		}
	}
}

// TestProvisionRejectsSeatWithoutDisplayName holds the invariant independently of
// the guards upstream of it. Only a direct write can reach this state — JQ-126
// keeps a nameless player out of the queue, and NormalizeDisplayName refuses to
// clear a name once set — which is the point: the handoff does not hand a game a
// nameless seat even when something upstream has gone wrong.
func TestProvisionRejectsSeatWithoutDisplayName(t *testing.T) {
	env, cleaner, provisioner := newProvisionEnv(t)
	ctx := context.Background()

	namelessID, cookieA := createIdentifiedPlayer(t, ctx, env, cleaner, "Briefly Named")
	_, cookieB := createIdentifiedPlayer(t, ctx, env, cleaner, "Still Named")

	joinQuery := `mutation Join($id: ID!) { joinQueue(queueId: $id) { queued queuedCount } }`
	vars := map[string]any{"id": demoDefaultQueueID}
	postGraphQL(t, env.Handler, joinQuery, vars, cookieA)
	postGraphQL(t, env.Handler, joinQuery, vars, cookieB)

	if _, err := env.DB.ExecContext(ctx, `UPDATE users SET display_name = NULL WHERE id = $1`, namelessID); err != nil {
		t.Fatalf("clear display name: %v", err)
	}
	// A failed provision is deferred and retried, so this test would otherwise
	// leave a nameless player and a stuck matched session churning in the shared
	// demo queue for whatever runs next — including the store package, which
	// tests against the same database in parallel.
	t.Cleanup(func() {
		if _, err := env.DB.ExecContext(context.Background(),
			`UPDATE users SET display_name = $2 WHERE id = $1`, namelessID, "Briefly Named"); err != nil {
			t.Errorf("restore display name: %v", err)
		}
		clearDemoQueue(t, env.Store)
	})

	flushFormingWorker(t, env, ctx, uuid.MustParse(demoDefaultQueueID))
	// The forming worker defers a failed provision and retries it rather than
	// erroring out, so give the deferral a beat before reading the count.
	time.Sleep(100 * time.Millisecond)

	if n := provisioner.callCount(); n != 0 {
		t.Fatalf("provisioned a match with a nameless seat: %+v", provisioner.lastCall().Assignment)
	}
}
