package store

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/gameclient"
)

func sampleManifest() *gameclient.Manifest {
	return &gameclient.Manifest{
		Modes: []gameclient.ModeManifest{{
			Key:          "classic",
			DisplayName:  "Classic",
			SeatTemplate: json.RawMessage(`{"count":2}`),
		}},
		Status: gameclient.StatusResponse{
			Game:    "Catalog Test Game",
			Version: "1.0.0",
		},
		ETag:       `"test-etag"`,
		RawJSON:    []byte(`{"modes":[{"key":"classic"}]}`),
		SHA256Hash: "abc123",
	}
}

func TestRegisterGameRequiresIcon(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	slug := "catalog-" + uuid.NewString()
	_, err := st.RegisterGame(ctx, RegisterGameParams{
		Slug:       slug,
		APIBaseURL: "https://api.example.com/" + slug,
	}, sampleManifest())
	if err == nil {
		t.Fatal("expected RegisterGame to require iconUrl")
	}
}

func TestRegisterGameRequiresHero(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	slug := "catalog-" + uuid.NewString()
	_, err := st.RegisterGame(ctx, RegisterGameParams{
		Slug:       slug,
		IconURL:    "/games/default.svg",
		APIBaseURL: "https://api.example.com/" + slug,
	}, sampleManifest())
	if err == nil {
		t.Fatal("expected RegisterGame to require heroUrl")
	}
}

func TestRegisterGameAndRefreshManifest(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	slug := "catalog-" + uuid.NewString()
	manifest := sampleManifest()
	manifest.SHA256Hash = uuid.NewString()

	result, err := st.RegisterGame(ctx, RegisterGameParams{
		Slug:       slug,
		IconURL:    "/games/default.svg",
		HeroURL:    "/games/default-hero.svg",
		APIBaseURL: "https://api.example.com/" + slug,
	}, manifest)
	if err != nil {
		t.Fatalf("RegisterGame failed: %v", err)
	}
	cleaner.TrackGame(result.Game.ID)

	if result.WebhookSecret == "" {
		t.Fatal("expected webhook secret")
	}
	if result.Game.Slug == nil || *result.Game.Slug != slug {
		t.Fatalf("expected slug %q, got %+v", slug, result.Game.Slug)
	}
	if result.Game.ManifestHash == nil || *result.Game.ManifestHash != manifest.SHA256Hash {
		t.Fatalf("expected manifest hash %q, got %+v", manifest.SHA256Hash, result.Game.ManifestHash)
	}

	modes, err := st.ListGameModesByGameID(ctx, result.Game.ID)
	if err != nil {
		t.Fatalf("ListGameModesByGameID failed: %v", err)
	}
	if len(modes) != 1 {
		t.Fatalf("expected 1 mode, got %d", len(modes))
	}
	if modes[0].ModeKey != "classic" {
		t.Fatalf("expected mode key classic, got %q", modes[0].ModeKey)
	}

	seats, err := st.ListGameModeSeats(ctx, modes[0].ID)
	if err != nil {
		t.Fatalf("ListGameModeSeats failed: %v", err)
	}
	if len(seats) != 2 {
		t.Fatalf("expected 2 seats, got %d", len(seats))
	}
	if seats[0].SeatKey != "1" || seats[1].SeatKey != "2" {
		t.Fatalf("expected seat keys 1/2, got %+v", seats)
	}

	queues, err := st.ListModeQueuesByModeID(ctx, modes[0].ID)
	if err != nil {
		t.Fatalf("ListModeQueuesByModeID failed: %v", err)
	}
	if len(queues) != 1 {
		t.Fatalf("expected default queue, got %d", len(queues))
	}
	if queues[0].PlayersToStart != modes[0].MinPlayers {
		t.Fatalf("expected players_to_start=%d, got %d", modes[0].MinPlayers, queues[0].PlayersToStart)
	}

	unchanged, err := st.ApplyGameManifest(ctx, result.Game.ID, manifest)
	if err != nil {
		t.Fatalf("ApplyGameManifest with same hash failed: %v", err)
	}
	if unchanged.Changed {
		t.Fatal("expected unchanged manifest to skip reconcile")
	}

	updatedManifest := sampleManifest()
	updatedManifest.SHA256Hash = uuid.NewString()
	updatedManifest.Modes[0].DisplayName = "Classic Updated"
	updatedManifest.Status.Version = "1.0.1"

	updated, err := st.ApplyGameManifest(ctx, result.Game.ID, updatedManifest)
	if err != nil {
		t.Fatalf("ApplyGameManifest update failed: %v", err)
	}
	if !updated.Changed {
		t.Fatal("expected manifest update to reconcile")
	}
	if updated.Game.GameVersion == nil || *updated.Game.GameVersion != "1.0.1" {
		t.Fatalf("expected game version 1.0.1, got %+v", updated.Game.GameVersion)
	}

	modes, err = st.ListGameModesByGameID(ctx, result.Game.ID)
	if err != nil {
		t.Fatalf("ListGameModesByGameID after update failed: %v", err)
	}
	if len(modes) != 1 || modes[0].DisplayName != "Classic Updated" {
		t.Fatalf("expected updated mode display name, got %+v", modes)
	}
}
