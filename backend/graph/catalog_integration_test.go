package graph

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/99designs/gqlgen/client"
	"github.com/google/uuid"
	"github.com/scruffyprodigy/playhub/internal/auth"
	"github.com/scruffyprodigy/playhub/internal/email"
)

func newMockGameAPIServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		case "/api/v1/status":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"game":"Mock Game","version":"2.0.0","appEnv":"test","standalone":true}`))
		case "/api/v1/game-modes":
			w.Header().Set("ETag", `"mock-v1"`)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"modes": [{
					"key": "duel",
					"displayName": "Duel",
					"seatTemplate": {"count": 2}
				}]
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
}

func signInAdmin(t *testing.T, adminEmail string) (*client.Client, []*http.Cookie) {
	t.Helper()
	t.Setenv("LOBBY_ADMIN_EMAILS", adminEmail)

	mailer := &email.CaptureSender{}
	c, handler, st := newAuthGraphQLTestClientWithMailer(t, mailer)
	cleaner := st.NewTestCleaner(t)
	cleaner.TrackEmail(adminEmail)

	var loginResp struct {
		RequestSignIn bool `json:"requestSignIn"`
	}
	if err := c.Post(`mutation RequestSignIn($email: String!) {
		requestSignIn(email: $email)
	}`, &loginResp, client.Var("email", adminEmail)); err != nil {
		t.Fatalf("requestSignIn failed: %v", err)
	}
	if mailer.Last.Code == "" {
		t.Fatal("expected login code in email")
	}

	completeQuery := `mutation CompleteSignInWithCode($email: String!, $code: String!) {
		completeSignInWithCode(email: $email, code: $code) {
			email
		}
	}`
	sessionCookies, body := postGraphQLWithCookies(t, handler, completeQuery, map[string]any{
		"email": adminEmail,
		"code":  mailer.Last.Code,
	})
	if len(sessionCookieOptions(sessionCookies)) == 0 {
		t.Fatalf("expected session cookie, body=%s", body)
	}

	user, err := st.GetUserByEmail(context.Background(), adminEmail)
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	cleaner.TrackUser(user.ID)

	return client.New(handler), sessionCookies
}

func TestCatalogRegisterAndRefreshManifest(t *testing.T) {
	t.Setenv("LOBBY_GAME_TOKEN_PEPPER", "catalog-test-pepper")
	gameAPI := newMockGameAPIServer(t)
	defer gameAPI.Close()

	adminEmail := "admin-catalog-" + uuid.NewString() + "@example.com"
	c, sessionCookies := signInAdmin(t, adminEmail)
	opts := sessionCookieOptions(sessionCookies)

	slug := "mock-" + uuid.NewString()
	var registerResp struct {
		RegisterGame struct {
			ServiceToken  string `json:"serviceToken"`
			WebhookSecret string `json:"webhookSecret"`
			Game          struct {
				ID   string  `json:"id"`
				Slug *string `json:"slug"`
				Modes []struct {
					ModeKey string `json:"modeKey"`
					Seats   []struct {
						SeatKey string `json:"seatKey"`
					} `json:"seats"`
					Queues []struct {
						Name           string `json:"name"`
						PlayersToStart int    `json:"playersToStart"`
					} `json:"queues"`
				} `json:"modes"`
			} `json:"game"`
		} `json:"registerGame"`
	}

	err := c.Post(`mutation RegisterGame($input: RegisterGameInput!) {
		registerGame(input: $input) {
			serviceToken
			webhookSecret
			game {
				id
				slug
				modes {
					modeKey
					seats { seatKey }
					queues { name playersToStart }
				}
			}
		}
	}`, &registerResp, append(opts, client.Var("input", map[string]any{
		"slug":       slug,
		"apiBaseUrl": gameAPI.URL,
		"iconUrl":    "/games/default.svg",
		"heroUrl":    "/games/default-hero.svg",
	}))...)
	if err != nil {
		t.Fatalf("registerGame failed: %v", err)
	}
	if registerResp.RegisterGame.WebhookSecret == "" {
		t.Fatal("expected webhook secret")
	}
	gameID, err := uuid.Parse(registerResp.RegisterGame.Game.ID)
	if err != nil {
		t.Fatalf("parse game id: %v", err)
	}
	wantToken, err := auth.FormatGameServiceToken(gameID)
	if err != nil {
		t.Fatalf("FormatGameServiceToken: %v", err)
	}
	if registerResp.RegisterGame.ServiceToken != wantToken {
		t.Fatalf("serviceToken = %q, want %q", registerResp.RegisterGame.ServiceToken, wantToken)
	}
	if registerResp.RegisterGame.Game.Slug == nil || *registerResp.RegisterGame.Game.Slug != slug {
		t.Fatalf("expected slug %q, got %+v", slug, registerResp.RegisterGame.Game.Slug)
	}
	if len(registerResp.RegisterGame.Game.Modes) != 1 {
		t.Fatalf("expected 1 mode, got %+v", registerResp.RegisterGame.Game.Modes)
	}
	if registerResp.RegisterGame.Game.Modes[0].ModeKey != "duel" {
		t.Fatalf("expected duel mode, got %+v", registerResp.RegisterGame.Game.Modes)
	}
	if len(registerResp.RegisterGame.Game.Modes[0].Seats) != 2 {
		t.Fatalf("expected 2 seats, got %+v", registerResp.RegisterGame.Game.Modes[0].Seats)
	}

	var refreshResp struct {
		RefreshGameManifest struct {
			ID          string  `json:"id"`
			GameVersion *string `json:"gameVersion"`
		} `json:"refreshGameManifest"`
	}
	err = c.Post(`mutation RefreshGameManifest($gameId: ID!) {
		refreshGameManifest(gameId: $gameId) {
			id
			gameVersion
		}
	}`, &refreshResp, append(opts, client.Var("gameId", registerResp.RegisterGame.Game.ID))...)
	if err != nil {
		t.Fatalf("refreshGameManifest failed: %v", err)
	}
	if refreshResp.RefreshGameManifest.ID != registerResp.RegisterGame.Game.ID {
		t.Fatalf("expected same game id, got %+v", refreshResp.RefreshGameManifest)
	}
	if refreshResp.RefreshGameManifest.GameVersion == nil || *refreshResp.RefreshGameManifest.GameVersion != "2.0.0" {
		t.Fatalf("expected game version 2.0.0, got %+v", refreshResp.RefreshGameManifest.GameVersion)
	}
}

func TestRegisterGameRequiresAdmin(t *testing.T) {
	gameAPI := newMockGameAPIServer(t)
	defer gameAPI.Close()

	c, _, _ := newAuthGraphQLTestClient(t)
	var resp struct {
		RegisterGame *struct{}
	}
	err := c.Post(`mutation RegisterGame($input: RegisterGameInput!) {
		registerGame(input: $input) {
			game { id }
		}
	}`, &resp, client.Var("input", map[string]any{
		"slug":       "no-auth",
		"apiBaseUrl": gameAPI.URL,
		"iconUrl":    "/games/default.svg",
		"heroUrl":    "/games/default-hero.svg",
	}))
	if err == nil {
		t.Fatal("expected registerGame to require admin authentication")
	}
}

func TestRefreshGameManifestRequiresAdmin(t *testing.T) {
	c, _, _ := newAuthGraphQLTestClient(t)
	var resp struct {
		RefreshGameManifest *struct{}
	}
	err := c.Post(`mutation RefreshGameManifest($gameId: ID!) {
		refreshGameManifest(gameId: $gameId) {
			id
		}
	}`, &resp, client.Var("gameId", "00000000-0000-4000-8000-000000000099"))
	if err == nil {
		t.Fatal("expected refreshGameManifest to require admin authentication")
	}
}

func TestRegisterGameGraphQLErrorIsJSON(t *testing.T) {
	t.Setenv("LOBBY_GAME_TOKEN_PEPPER", "catalog-test-pepper")
	gameAPI := newMockGameAPIServer(t)
	defer gameAPI.Close()

	adminEmail := "admin-catalog-" + uuid.NewString() + "@example.com"
	c, sessionCookies := signInAdmin(t, adminEmail)

	slug := "dup-" + uuid.NewString()
	var firstResp struct {
		RegisterGame struct {
			Game struct {
				ID string `json:"id"`
			} `json:"game"`
		} `json:"registerGame"`
	}
	err := c.Post(`mutation RegisterGame($input: RegisterGameInput!) {
		registerGame(input: $input) {
			game { id }
		}
	}`, &firstResp, append(sessionCookieOptions(sessionCookies), client.Var("input", map[string]any{
		"slug":       slug,
		"apiBaseUrl": gameAPI.URL,
		"iconUrl":    "/games/default.svg",
		"heroUrl":    "/games/default-hero.svg",
	}))...)
	if err != nil {
		t.Fatalf("first registerGame failed: %v", err)
	}

	var dupResp struct {
		RegisterGame struct {
			Game struct {
				ID string `json:"id"`
			} `json:"game"`
		} `json:"registerGame"`
	}
	err = c.Post(`mutation RegisterGame($input: RegisterGameInput!) {
		registerGame(input: $input) {
			game { id }
		}
	}`, &dupResp, append(sessionCookieOptions(sessionCookies), client.Var("input", map[string]any{
		"slug":       slug,
		"apiBaseUrl": gameAPI.URL,
		"iconUrl":    "/games/default.svg",
		"heroUrl":    "/games/default-hero.svg",
	}))...)
	if err == nil {
		t.Fatal("expected duplicate slug registerGame to fail")
	}
}
