package store

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
)

func TestUpdateMyGameMetadataAccentColor(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	owner, err := st.CreateUser(ctx, CreateUserParams{
		Email:       "accent-" + uuid.NewString() + "@example.com",
		DisplayName: "Accent Owner",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(owner.ID)

	slug := "accent-" + uuid.NewString()[:8]
	registered, err := st.RegisterMyGame(ctx, RegisterMyGameParams{
		OwnerUserID:      owner.ID,
		Slug:             slug,
		Name:             "Accent Game",
		ShortDescription: "A short blurb",
		APIBaseURL:       "https://api.example.com/" + slug,
		ContactEmail:     "dev@example.com",
	}, nil, fmt.Errorf("unreachable"))
	if err != nil {
		t.Fatalf("RegisterMyGame: %v", err)
	}
	cleaner.TrackGame(registered.Game.ID)

	if registered.Game.AccentColor != nil {
		t.Fatalf("expected a new game to have no accent color, got %q", *registered.Game.AccentColor)
	}

	accent := "#7c3aed"
	updated, err := st.UpdateMyGameMetadata(ctx, registered.Game.ID, owner.ID, UpdateMyGameMetadataParams{
		AccentColor: &accent,
	})
	if err != nil {
		t.Fatalf("UpdateMyGameMetadata set: %v", err)
	}
	if updated.AccentColor == nil || *updated.AccentColor != accent {
		t.Fatalf("expected accent %q, got %v", accent, updated.AccentColor)
	}

	reread, err := st.GetOwnedGame(ctx, registered.Game.ID, owner.ID)
	if err != nil {
		t.Fatalf("GetOwnedGame: %v", err)
	}
	if reread.AccentColor == nil || *reread.AccentColor != accent {
		t.Fatalf("expected re-read accent %q, got %v", accent, reread.AccentColor)
	}

	cleared, err := st.UpdateMyGameMetadata(ctx, registered.Game.ID, owner.ID, UpdateMyGameMetadataParams{
		ClearAccentColor: true,
	})
	if err != nil {
		t.Fatalf("UpdateMyGameMetadata clear: %v", err)
	}
	if cleared.AccentColor != nil {
		t.Fatalf("expected cleared accent, got %q", *cleared.AccentColor)
	}
}
