package graph

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/playhub/internal/gameclient"
	"github.com/scruffyprodigy/playhub/internal/store"
)

type failingProvisioner struct{}

func (failingProvisioner) ProvisionMatch(context.Context, gameclient.ProvisionRequest) (gameclient.ProvisionResult, error) {
	return gameclient.ProvisionResult{}, errors.New("game server unavailable")
}

func TestSignLaunchURLEmptyUntilProvisioned(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := context.Background()
	clearDemoQueue(t, env.Store)

	t.Setenv("LOBBY_ISSUER_URL", "http://localhost:8080")
	t.Setenv("LOBBY_PUBLIC_URL", "http://localhost:5173")
	t.Setenv("LOBBY_GAME_TOKEN_PEPPER", "test-pepper")

	resolver := env.resolver
	env.resolverWithProvisioner(t, failingProvisioner{})

	userA, err := env.Store.CreateUser(ctx, store.CreateUserParams{
		Email: "launch-a-" + uuid.NewString() + "@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser A: %v", err)
	}
	cleaner.TrackUser(userA.ID)
	userB, err := env.Store.CreateUser(ctx, store.CreateUserParams{
		Email: "launch-b-" + uuid.NewString() + "@example.com",
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

	game, err := env.Store.GetGameByID(ctx, rec.GameID)
	if err != nil {
		t.Fatalf("GetGameByID: %v", err)
	}
	apiURL := "http://localhost:3001"
	game.APIBaseURL = &apiURL

	launch, err := resolver.signLaunchURL(ctx, game, *rec.SessionID, userB.ID)
	if err != nil {
		t.Fatalf("signLaunchURL: %v", err)
	}
	if launch != "" {
		t.Fatalf("expected empty launch url before provision, got %q", launch)
	}
}

func TestSignLaunchURLFromStoredBase(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := context.Background()
	clearDemoQueue(t, env.Store)

	t.Setenv("LOBBY_ISSUER_URL", "http://localhost:8080")
	t.Setenv("LOBBY_PUBLIC_URL", "http://localhost:5173")
	t.Setenv("LOBBY_GAME_TOKEN_PEPPER", "test-pepper")

	userA, err := env.Store.CreateUser(ctx, store.CreateUserParams{
		Email: "launch-a-" + uuid.NewString() + "@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser A: %v", err)
	}
	cleaner.TrackUser(userA.ID)
	userB, err := env.Store.CreateUser(ctx, store.CreateUserParams{
		Email: "launch-b-" + uuid.NewString() + "@example.com",
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

	game, err := env.Store.GetGameByID(ctx, rec.GameID)
	if err != nil {
		t.Fatalf("GetGameByID: %v", err)
	}
	apiURL := "http://localhost:3001"
	game.APIBaseURL = &apiURL
	sessionID := *rec.SessionID

	provisioner := &gameURLProvisioner{
		launchURLs: map[string]string{
			userA.ID.String(): "http://localhost:5174/?match=" + sessionID.String() + "&seat=1",
			userB.ID.String(): "http://localhost:5174/?match=" + sessionID.String() + "&seat=2",
		},
	}
	env.resolverWithProvisioner(t, provisioner)

	if _, err := env.resolver.finalizeMatchedSession(ctx, game, sessionID, rec.NotifyUserIDs); err != nil {
		t.Fatalf("finalizeMatchedSession: %v", err)
	}

	launch, err := env.resolver.signLaunchURL(ctx, game, sessionID, userB.ID)
	if err != nil {
		t.Fatalf("signLaunchURL: %v", err)
	}
	if launch == "" {
		t.Fatal("expected launch url from stored base")
	}
	if !strings.Contains(launch, "match=") || !strings.Contains(launch, "token=") {
		t.Fatalf("launch url = %q", launch)
	}
}
