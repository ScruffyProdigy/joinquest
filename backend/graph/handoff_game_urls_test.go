package graph

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/playhub/internal/gameclient"
	"github.com/scruffyprodigy/playhub/internal/store"
)

type gameURLProvisioner struct {
	launchURLs map[string]string
}

func (p *gameURLProvisioner) ProvisionMatch(_ context.Context, req gameclient.ProvisionRequest) (gameclient.ProvisionResult, error) {
	urls := make(map[string]string, len(p.launchURLs))
	for id, raw := range p.launchURLs {
		urls[id] = raw
	}
	return gameclient.ProvisionResult{LaunchURLs: urls}, nil
}

func TestFinalizeMatchedSessionUsesGameMintedURLs(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := context.Background()
	clearDemoQueue(t, env.Store)

	t.Setenv("LOBBY_ISSUER_URL", "http://localhost:8080")
	t.Setenv("LOBBY_PUBLIC_URL", "http://localhost:5173")
	t.Setenv("LOBBY_GAME_TOKEN_PEPPER", "test-pepper")

	// Named on purpose: this test drives the store directly and so skips the
	// play-entry guard, but the handoff still requires a seat to name its
	// player (JQ-124). The subject here is game-minted URLs, not identity.
	userA, err := env.Store.CreateUser(ctx, store.CreateUserParams{
		Email:       "game-url-a-" + uuid.NewString() + "@example.com",
		DisplayName: "Game URL A",
	})
	if err != nil {
		t.Fatalf("CreateUser A: %v", err)
	}
	cleaner.TrackUser(userA.ID)
	userB, err := env.Store.CreateUser(ctx, store.CreateUserParams{
		Email:       "game-url-b-" + uuid.NewString() + "@example.com",
		DisplayName: "Game URL B",
	})
	if err != nil {
		t.Fatalf("CreateUser B: %v", err)
	}
	cleaner.TrackUser(userB.ID)

	queueID := uuid.MustParse(store.DemoDefaultQueueIDStr)
	if _, err := env.Store.JoinModeQueue(ctx, queueID, userA.ID, "", nil); err != nil {
		t.Fatalf("join A: %v", err)
	}
	if _, err := env.Store.JoinModeQueue(ctx, queueID, userB.ID, "", nil); err != nil {
		t.Fatalf("join B: %v", err)
	}
	rec, err := env.Store.ReconcileFormingModeQueue(ctx, queueID)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if rec.SessionID == nil {
		t.Fatal("expected matched session")
	}
	sessionID := *rec.SessionID

	game, err := env.Store.GetGameByID(ctx, rec.GameID)
	if err != nil {
		t.Fatalf("GetGameByID: %v", err)
	}
	apiURL := "http://localhost:3001"
	game.APIBaseURL = &apiURL

	provisioner := &gameURLProvisioner{
		launchURLs: map[string]string{
			userA.ID.String(): "http://localhost:5174/?match=" + sessionID.String() + "&seat=1",
			userB.ID.String(): "http://localhost:5174/?match=" + sessionID.String() + "&seat=2",
		},
	}
	env.resolverWithProvisioner(t, provisioner)

	urls, err := env.resolver.finalizeMatchedSession(ctx, game, sessionID, rec.NotifyUserIDs)
	if err != nil {
		t.Fatalf("finalizeMatchedSession: %v", err)
	}
	for _, uid := range []uuid.UUID{userA.ID, userB.ID} {
		launch := urls[uid]
		if launch == "" {
			t.Fatalf("missing launch url for %s", uid)
		}
		if !strings.Contains(launch, "token=") {
			t.Fatalf("launch url missing token: %q", launch)
		}
		if !strings.Contains(launch, "match="+sessionID.String()) {
			t.Fatalf("launch url missing match: %q", launch)
		}
	}

	stored, err := env.Store.GetSessionParticipantLaunchURLBase(ctx, sessionID, userB.ID)
	if err != nil {
		t.Fatalf("GetSessionParticipantLaunchURLBase: %v", err)
	}
	if !strings.Contains(stored, "seat=2") {
		t.Fatalf("stored base = %q", stored)
	}
}
