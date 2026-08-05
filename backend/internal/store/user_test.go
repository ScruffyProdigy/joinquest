package store

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestDefaultDisplayName(t *testing.T) {
	got := DefaultDisplayName("coleryanxxx@gmail.com")
	want := "coleryanxxx (new)"
	if got != want {
		t.Fatalf("DefaultDisplayName() = %q, want %q", got, want)
	}
}

func TestCreateUserUsesProvisionalDisplayName(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := t.Context()

	email := "display-name-" + mustUUID(t) + "@example.com"
	user, err := st.CreateUser(ctx, CreateUserParams{Email: email})
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	cleaner.TrackUser(user.ID)

	expectedPrefix := strings.Split(email, "@")[0]
	if !strings.HasPrefix(user.DisplayName, expectedPrefix+" (new)") {
		t.Fatalf("expected provisional display name, got %q", user.DisplayName)
	}
	if user.DisplayName == user.Username {
		t.Fatalf("expected display name to differ from internal username %q", user.Username)
	}
}

func TestCreateUserRespectsExplicitDisplayName(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := t.Context()

	email := "custom-name-" + mustUUID(t) + "@example.com"
	user, err := st.CreateUser(ctx, CreateUserParams{
		Email:       email,
		DisplayName: "Custom Name",
	})
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	cleaner.TrackUser(user.ID)
	if user.DisplayName != "Custom Name" {
		t.Fatalf("expected explicit display name, got %q", user.DisplayName)
	}
}

func TestCreateUserUsernameIncludesRandomSuffix(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := t.Context()

	email := "alice-" + mustUUID(t) + "@example.com"
	user, err := st.CreateUser(ctx, CreateUserParams{Email: email})
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	cleaner.TrackUser(user.ID)

	base := strings.Split(strings.ToLower(email), "@")[0]
	base = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, base)

	if !strings.HasPrefix(user.Username, base+"_") {
		t.Fatalf("expected username to start with %q_, got %q", base, user.Username)
	}
	suffix := strings.TrimPrefix(user.Username, base+"_")
	if len(suffix) != 8 {
		t.Fatalf("expected 8-character username suffix, got %q", suffix)
	}
}

func TestResolveUserIDByEmailReturnsNotFoundForUnknownAddress(t *testing.T) {
	st := openTestStore(t)
	ctx := t.Context()

	_, err := st.ResolveUserIDByEmail(ctx, "missing-"+mustUUID(t)+"@example.com")
	if err == nil {
		t.Fatal("expected ErrNotFound for unknown email")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func mustUUID(t *testing.T) string {
	t.Helper()
	return strings.ReplaceAll(uuid.NewString(), "-", "")
}
