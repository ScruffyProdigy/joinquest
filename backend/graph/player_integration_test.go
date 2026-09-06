package graph

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/playhub/internal/auth"
	"github.com/scruffyprodigy/playhub/internal/store"
)

func postGraphQLWithBearer(t *testing.T, handler http.Handler, bearer, query string, variables map[string]any) []byte {
	t.Helper()

	payload := map[string]any{"query": query}
	if variables != nil {
		payload["variables"] = variables
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code >= http.StatusBadRequest {
		t.Fatalf("HTTP %d: %s", rec.Code, rec.Body.String())
	}
	return rec.Body.Bytes()
}

func TestPlayerLookupRequiresServiceTokenWhenConfigured(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := t.Context()

	t.Setenv("LOBBY_GAME_SERVICE_TOKEN", "test-game-service-token")

	user, err := env.Store.CreateUser(ctx, store.CreateUserParams{
		Email:       "player-lookup-" + uuid.NewString() + "@example.com",
		DisplayName: "Lookup Test Player",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(user.ID)

	query := `query Player($id: ID!) { player(id: $id) { id displayName avatarUrl avatarSource } }`
	vars := map[string]any{"id": user.ID.String()}

	deniedBody := postGraphQLWithBearer(t, env.Handler, "", query, vars)
	var denied struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(deniedBody, &denied); err != nil {
		t.Fatalf("decode denied: %v", err)
	}
	if len(denied.Errors) == 0 {
		t.Fatalf("expected auth error without service token, got %s", deniedBody)
	}

	okBody := postGraphQLWithBearer(t, env.Handler, "test-game-service-token", query, vars)
	var okResp struct {
		Data struct {
			Player *struct {
				ID          string  `json:"id"`
				DisplayName *string `json:"displayName"`
			} `json:"player"`
		} `json:"data"`
	}
	if err := json.Unmarshal(okBody, &okResp); err != nil {
		t.Fatalf("decode ok: %v", err)
	}
	if okResp.Data.Player == nil {
		t.Fatalf("expected player, got %s", okBody)
	}
	if okResp.Data.Player.ID != user.ID.String() {
		t.Fatalf("id = %q", okResp.Data.Player.ID)
	}
	if okResp.Data.Player.DisplayName == nil || *okResp.Data.Player.DisplayName != "Lookup Test Player" {
		t.Fatalf("displayName = %+v", okResp.Data.Player.DisplayName)
	}
}

func TestPlayerLookupRequiresServiceTokenWithPepperOnly(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := t.Context()

	t.Setenv("LOBBY_GAME_TOKEN_PEPPER", "pepper-only-auth")
	t.Setenv("LOBBY_GAME_SERVICE_TOKEN", "")

	user, err := env.Store.CreateUser(ctx, store.CreateUserParams{
		Email:       "player-pepper-" + uuid.NewString() + "@example.com",
		DisplayName: "Pepper Auth Player",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(user.ID)

	query := `query Player($id: ID!) { player(id: $id) { id displayName avatarUrl avatarSource } }`
	vars := map[string]any{"id": user.ID.String()}

	deniedBody := postGraphQLWithBearer(t, env.Handler, "", query, vars)
	var denied struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(deniedBody, &denied); err != nil {
		t.Fatalf("decode denied: %v", err)
	}
	if len(denied.Errors) == 0 {
		t.Fatalf("expected auth error without service token when pepper is configured, got %s", deniedBody)
	}

	gameID := uuid.MustParse(store.DemoPrimaryGameIDStr)
	token, err := auth.FormatGameServiceToken(gameID)
	if err != nil {
		t.Fatalf("FormatGameServiceToken: %v", err)
	}
	okBody := postGraphQLWithBearer(t, env.Handler, token, query, vars)
	var okResp struct {
		Data struct {
			Player *struct {
				ID string `json:"id"`
			} `json:"player"`
		} `json:"data"`
	}
	if err := json.Unmarshal(okBody, &okResp); err != nil {
		t.Fatalf("decode ok: %v", err)
	}
	if okResp.Data.Player == nil || okResp.Data.Player.ID != user.ID.String() {
		t.Fatalf("expected player with per-game token, got %s", okBody)
	}
}

func TestPlayerLookupWithoutServiceTokenInDev(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := t.Context()

	t.Setenv("LOBBY_GAME_SERVICE_TOKEN", "")
	t.Setenv("LOBBY_GAME_TOKEN_PEPPER", "")

	user, err := env.Store.CreateUser(ctx, store.CreateUserParams{
		Email: "player-open-" + uuid.NewString() + "@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(user.ID)

	query := `query Player($id: ID!) { player(id: $id) { id displayName avatarUrl avatarSource } }`
	body := postGraphQLWithBearer(t, env.Handler, "", query, map[string]any{"id": user.ID.String()})

	var resp struct {
		Data struct {
			Player *struct {
				ID string `json:"id"`
			} `json:"player"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.Player == nil || resp.Data.Player.ID != user.ID.String() {
		t.Fatalf("expected player in dev mode, got %s", body)
	}
}
