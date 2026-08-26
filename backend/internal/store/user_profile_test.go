package store

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestNormalizeDisplayName(t *testing.T) {
	got, err := NormalizeDisplayName("  Pat  ")
	if err != nil || got != "Pat" {
		t.Fatalf("NormalizeDisplayName() = %q, %v", got, err)
	}
	if _, err := NormalizeDisplayName("   "); err != ErrInvalidDisplayName {
		t.Fatalf("expected ErrInvalidDisplayName for blank, got %v", err)
	}
}

func TestUpdateUserProfile(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := t.Context()

	user, err := st.CreateUser(ctx, CreateUserParams{
		Email: "profile-" + uuid.NewString() + "@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(user.ID)

	updated, err := st.UpdateUserProfile(ctx, user.ID, "River", "beacon", "https://joinquest.cc")
	if err != nil {
		t.Fatalf("UpdateUserProfile: %v", err)
	}
	if updated.DisplayName != "River" {
		t.Fatalf("display name: %q", updated.DisplayName)
	}
	if updated.AvatarKey == nil || *updated.AvatarKey != "beacon" {
		t.Fatalf("avatar key: %+v", updated.AvatarKey)
	}
	if updated.AvatarURL == nil || !strings.HasSuffix(*updated.AvatarURL, "/avatars/beacon.png") {
		t.Fatalf("avatar url: %+v", updated.AvatarURL)
	}

	_, err = st.UpdateUserProfile(ctx, user.ID, "", "beacon", "https://joinquest.cc")
	if err != ErrInvalidDisplayName {
		t.Fatalf("expected ErrInvalidDisplayName, got %v", err)
	}

	_, err = st.UpdateUserProfile(ctx, user.ID, "River", "not-real", "https://joinquest.cc")
	if err != ErrInvalidAvatarKey {
		t.Fatalf("expected ErrInvalidAvatarKey, got %v", err)
	}
}

func TestUpdateUserProfileNameOnlyPreservesSpiritAnimal(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := t.Context()

	user, err := st.CreateUser(ctx, CreateUserParams{
		Email: "spirit-profile-" + uuid.NewString() + "@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(user.ID)

	spiritURL := "https://joinquest.cc/avatars/spirit/wolf.png"
	_, err = st.db.ExecContext(ctx, `
		UPDATE users
		SET avatar_url = $2, avatar_key = NULL, avatar_source = $3
		WHERE id = $1
	`, user.ID, spiritURL, SourceSpiritAnimal)
	if err != nil {
		t.Fatalf("set spirit animal avatar: %v", err)
	}

	updated, err := st.UpdateUserProfile(ctx, user.ID, "River", "", "https://joinquest.cc")
	if err != nil {
		t.Fatalf("UpdateUserProfile: %v", err)
	}
	if updated.DisplayName != "River" {
		t.Fatalf("display name: %q", updated.DisplayName)
	}
	if updated.AvatarKey != nil {
		t.Fatalf("avatar key should stay nil, got %+v", updated.AvatarKey)
	}
	if updated.AvatarSource == nil || *updated.AvatarSource != SourceSpiritAnimal {
		t.Fatalf("avatar source: %+v", updated.AvatarSource)
	}
	if updated.AvatarURL == nil || *updated.AvatarURL != spiritURL {
		t.Fatalf("avatar url: %+v", updated.AvatarURL)
	}
}
