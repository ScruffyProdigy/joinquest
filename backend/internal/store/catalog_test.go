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

// A mode's declared duration round-trips, and a later manifest that omits it
// leaves the stored value alone (JQ-161). An omission means the developer has
// not re-declared, not that they have withdrawn the duration — the same rule
// socialMode follows.
func TestApplyManifestKeepsTypicalMinutesWhenOmitted(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	minutes := 12
	slug := "catalog-" + uuid.NewString()
	manifest := sampleManifest()
	manifest.SHA256Hash = uuid.NewString()
	manifest.Modes[0].TypicalMinutes = &minutes

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

	modes, err := st.ListGameModesByGameID(ctx, result.Game.ID)
	if err != nil {
		t.Fatalf("ListGameModesByGameID failed: %v", err)
	}
	if modes[0].TypicalMinutes == nil || *modes[0].TypicalMinutes != minutes {
		t.Fatalf("expected typical minutes %d, got %v", minutes, modes[0].TypicalMinutes)
	}

	omitted := sampleManifest()
	omitted.SHA256Hash = uuid.NewString()
	if _, err := st.ApplyGameManifest(ctx, result.Game.ID, omitted); err != nil {
		t.Fatalf("ApplyGameManifest failed: %v", err)
	}

	modes, err = st.ListGameModesByGameID(ctx, result.Game.ID)
	if err != nil {
		t.Fatalf("ListGameModesByGameID failed: %v", err)
	}
	if modes[0].TypicalMinutes == nil || *modes[0].TypicalMinutes != minutes {
		t.Fatalf("expected the stored duration to survive an omitting manifest, got %v", modes[0].TypicalMinutes)
	}

	changed := 20
	updated := sampleManifest()
	updated.SHA256Hash = uuid.NewString()
	updated.Modes[0].TypicalMinutes = &changed
	if _, err := st.ApplyGameManifest(ctx, result.Game.ID, updated); err != nil {
		t.Fatalf("ApplyGameManifest failed: %v", err)
	}

	modes, err = st.ListGameModesByGameID(ctx, result.Game.ID)
	if err != nil {
		t.Fatalf("ListGameModesByGameID failed: %v", err)
	}
	if modes[0].TypicalMinutes == nil || *modes[0].TypicalMinutes != changed {
		t.Fatalf("expected a re-declared duration to win, got %v", modes[0].TypicalMinutes)
	}
}

// A mode with no declared duration stays null rather than becoming zero, so the
// card can tell "not declared" from "declared as nothing".
func TestApplyManifestLeavesTypicalMinutesNullWhenNeverDeclared(t *testing.T) {
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

	modes, err := st.ListGameModesByGameID(ctx, result.Game.ID)
	if err != nil {
		t.Fatalf("ListGameModesByGameID failed: %v", err)
	}
	if modes[0].TypicalMinutes != nil {
		t.Fatalf("expected no duration, got %d", *modes[0].TypicalMinutes)
	}
}
