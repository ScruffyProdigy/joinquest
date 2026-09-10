package store

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/gameclient"
)

func registerModeForSkillMatching(t *testing.T, st *Store, ctx context.Context) *GameMode {
	t.Helper()

	slug := "skillsw-" + uuid.NewString()
	manifest := &gameclient.Manifest{
		Modes: []gameclient.ModeManifest{{
			Key:          "duel",
			DisplayName:  "Duel",
			SeatTemplate: json.RawMessage(`{"Player":{"count":2,"min":2,"max":2,"sizeForQueue":2}}`),
		}},
		Status:     gameclient.StatusResponse{Game: "Skill Switch", Version: "1.0.0"},
		ETag:       `"skillsw"`,
		RawJSON:    []byte(`{"modes":[{"key":"duel"}]}`),
		SHA256Hash: uuid.NewString(),
	}
	result, err := st.RegisterGame(ctx, RegisterGameParams{
		Slug:       slug,
		IconURL:    "/games/default.svg",
		HeroURL:    "/games/default-hero.svg",
		APIBaseURL: "https://api.example.com/" + slug,
	}, manifest)
	if err != nil {
		t.Fatalf("RegisterGame: %v", err)
	}
	modes, err := st.ListGameModesByGameID(ctx, result.Game.ID)
	if err != nil {
		t.Fatalf("ListGameModesByGameID: %v", err)
	}
	if len(modes) != 1 {
		t.Fatalf("registered %d modes, want 1", len(modes))
	}
	return &modes[0]
}

// Skill matching is on by default, because the lambda gate already keeps it
// harmless on a thin queue -- there is no population threshold to opt into.
// The switch exists for a different question: a mode whose skill signal turns
// out to be weak or meaningless should be able to opt out however busy it is.
func TestANewModeHasSkillMatchingEnabled(t *testing.T) {
	st := openTestStore(t)
	st.NewTestCleaner(t)
	ctx := context.Background()

	mode := registerModeForSkillMatching(t, st, ctx)

	if !mode.SkillMatchingEnabled {
		t.Errorf("a newly registered mode has SkillMatchingEnabled = false, want true")
	}
}

func TestSkillMatchingCanBeDisabledPerMode(t *testing.T) {
	st := openTestStore(t)
	st.NewTestCleaner(t)
	ctx := context.Background()

	mode := registerModeForSkillMatching(t, st, ctx)

	if _, err := st.db.ExecContext(ctx, `
		UPDATE game_modes SET skill_matching_enabled = false WHERE id = $1
	`, mode.ID); err != nil {
		t.Fatalf("disable skill matching: %v", err)
	}

	reread, err := st.GetGameModeByID(ctx, mode.ID)
	if err != nil {
		t.Fatalf("GetGameModeByID: %v", err)
	}
	if reread.SkillMatchingEnabled {
		t.Errorf("SkillMatchingEnabled = true after disabling it, want false")
	}
}
