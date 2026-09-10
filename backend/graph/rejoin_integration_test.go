package graph

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/auth"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

func authedContext(userID uuid.UUID) context.Context {
	return auth.WithUserID(context.Background(), userID.String())
}

// rejoinEnv is a provisioned, live two-player session ready to be rejoined.
type rejoinEnv struct {
	env       *queueIntegrationEnv
	resolver  *Resolver
	game      *store.Game
	sessionID uuid.UUID
	userA     *store.User
	userB     *store.User
}

func newRejoinEnv(t *testing.T) *rejoinEnv {
	t.Helper()
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := context.Background()
	clearDemoQueue(t, env.Store)

	t.Setenv("LOBBY_ISSUER_URL", "http://localhost:8080")
	t.Setenv("LOBBY_PUBLIC_URL", "http://localhost:5173")
	t.Setenv("LOBBY_GAME_TOKEN_PEPPER", "test-pepper")

	userA, err := env.Store.CreateUser(ctx, store.CreateUserParams{
		Email:       "rejoin-a-" + uuid.NewString() + "@example.com",
		DisplayName: "Rejoin A",
	})
	if err != nil {
		t.Fatalf("CreateUser A: %v", err)
	}
	cleaner.TrackUser(userA.ID)
	userB, err := env.Store.CreateUser(ctx, store.CreateUserParams{
		Email:       "rejoin-b-" + uuid.NewString() + "@example.com",
		DisplayName: "Rejoin B",
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
	if _, err := env.resolver.finalizeMatchedSession(ctx, game, sessionID, rec.NotifyUserIDs); err != nil {
		t.Fatalf("finalizeMatchedSession: %v", err)
	}

	return &rejoinEnv{
		env:       env,
		resolver:  env.resolver,
		game:      game,
		sessionID: sessionID,
		userA:     userA,
		userB:     userB,
	}
}

func seatTokenClaims(t *testing.T, launchURL string) jwt.MapClaims {
	t.Helper()
	parsed, err := url.Parse(launchURL)
	if err != nil {
		t.Fatalf("parse launch url %q: %v", launchURL, err)
	}
	raw := parsed.Query().Get("token")
	if raw == "" {
		t.Fatalf("launch url %q carries no token", launchURL)
	}
	claims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(raw, claims); err != nil {
		t.Fatalf("parse seat token: %v", err)
	}
	return claims
}

func claimSeconds(t *testing.T, claims jwt.MapClaims, key string) int64 {
	t.Helper()
	value, ok := claims[key].(float64)
	if !ok {
		t.Fatalf("claim %q missing or not numeric: %#v", key, claims[key])
	}
	return int64(value)
}

// A player who lost the tab gets back into the same seat in the same match, with a
// token that is new rather than the one that went out at launch (JQ-86).
func TestRejoinActiveMatchKeepsSeatAndMintsFreshToken(t *testing.T) {
	re := newRejoinEnv(t)
	ctx := authedContext(re.userB.ID)

	launch, err := re.resolver.signLaunchURL(ctx, re.game, re.sessionID, re.userB.ID)
	if err != nil {
		t.Fatalf("signLaunchURL: %v", err)
	}
	launchClaims := seatTokenClaims(t, launch)

	rejoinURL, err := re.resolver.Mutation().RejoinActiveMatch(ctx)
	if err != nil {
		t.Fatalf("RejoinActiveMatch: %v", err)
	}
	rejoinClaims := seatTokenClaims(t, rejoinURL)

	for _, claim := range []string{"sub", "matchId", "seatKey"} {
		if rejoinClaims[claim] != launchClaims[claim] {
			t.Fatalf("%s = %v, want %v (rejoin must not move the player)", claim, rejoinClaims[claim], launchClaims[claim])
		}
	}
	if rejoinClaims["sub"] != re.userB.ID.String() {
		t.Fatalf("sub = %v, want %s", rejoinClaims["sub"], re.userB.ID)
	}
	if rejoinClaims["matchId"] != re.sessionID.String() {
		t.Fatalf("matchId = %v, want %s", rejoinClaims["matchId"], re.sessionID)
	}
	if rejoinClaims["jti"] == launchClaims["jti"] {
		t.Fatal("rejoin reused the launch token's jti")
	}
	if !strings.Contains(rejoinURL, "match=") {
		t.Fatalf("rejoin url = %q, want the game-minted base preserved", rejoinURL)
	}
}

// AC 4: a rejoin token must be too short-lived to be a way of handing your seat to
// somebody else, and shorter than the token that went out at launch.
func TestRejoinActiveMatchTokenIsShortLived(t *testing.T) {
	re := newRejoinEnv(t)
	ctx := authedContext(re.userA.ID)

	rejoinURL, err := re.resolver.Mutation().RejoinActiveMatch(ctx)
	if err != nil {
		t.Fatalf("RejoinActiveMatch: %v", err)
	}
	claims := seatTokenClaims(t, rejoinURL)
	life := time.Duration(claimSeconds(t, claims, "exp")-claimSeconds(t, claims, "iat")) * time.Second
	if life != auth.RejoinSeatTokenTTL {
		t.Fatalf("rejoin token life = %s, want %s", life, auth.RejoinSeatTokenTTL)
	}
	if life > 10*time.Minute {
		t.Fatalf("rejoin token life = %s, too long to be theft-safe", life)
	}

	launch, err := re.resolver.signLaunchURL(ctx, re.game, re.sessionID, re.userA.ID)
	if err != nil {
		t.Fatalf("signLaunchURL: %v", err)
	}
	launchClaims := seatTokenClaims(t, launch)
	launchLife := time.Duration(claimSeconds(t, launchClaims, "exp")-claimSeconds(t, launchClaims, "iat")) * time.Second
	if life >= launchLife {
		t.Fatalf("rejoin life %s is not shorter than launch life %s", life, launchLife)
	}
}

// AC 3, first half: once the player has reported finished there is nothing to
// rejoin, and re-issue must refuse rather than hand back a live seat.
func TestRejoinActiveMatchRefusedAfterPlayerFinished(t *testing.T) {
	re := newRejoinEnv(t)
	ctx := authedContext(re.userB.ID)

	if err := re.env.Store.MarkParticipantFinished(context.Background(), re.sessionID, re.userB.ID, time.Now()); err != nil {
		t.Fatalf("MarkParticipantFinished: %v", err)
	}

	if _, err := re.resolver.Mutation().RejoinActiveMatch(ctx); err == nil {
		t.Fatal("expected rejoin to be refused for a player who already finished")
	}
}

// AC 3, second half: the match itself being over closes the door for everyone,
// including a player who never reported finished.
func TestRejoinActiveMatchRefusedAfterSessionEnds(t *testing.T) {
	re := newRejoinEnv(t)
	ctx := authedContext(re.userA.ID)

	if err := re.env.Store.CompleteSession(context.Background(), re.sessionID, time.Now()); err != nil {
		t.Fatalf("CompleteSession: %v", err)
	}

	if _, err := re.resolver.Mutation().RejoinActiveMatch(ctx); err == nil {
		t.Fatal("expected rejoin to be refused once the session is over")
	}
}

func TestRejoinActiveMatchRequiresAuth(t *testing.T) {
	re := newRejoinEnv(t)
	if _, err := re.resolver.Mutation().RejoinActiveMatch(context.Background()); err == nil {
		t.Fatal("expected rejoin to require an authenticated player")
	}
}
