package graph

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/playhub/internal/auth"
	"github.com/scruffyprodigy/playhub/internal/gameclient"
	"github.com/scruffyprodigy/playhub/internal/store"
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
