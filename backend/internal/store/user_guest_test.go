package store

import (
	"context"
	"testing"
)

func TestCreateGuestUser(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	user, err := st.CreateGuestUser(ctx)
	if err != nil {
		t.Fatalf("CreateGuestUser: %v", err)
	}
	cleaner.TrackUser(user.ID)

	if !user.IsGuest {
		t.Fatal("expected guest user")
	}
	if user.Email != "" {
		t.Fatalf("expected empty email, got %q", user.Email)
	}
	// A fresh guest is nameless until the identity prompt collects one.
	if user.DisplayName != nil {
		t.Fatalf("expected no display name, got %q", *user.DisplayName)
	}
}

func TestAddVerifiedUserEmailClearsGuest(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	guest, err := st.CreateGuestUser(ctx)
	if err != nil {
		t.Fatalf("CreateGuestUser: %v", err)
	}
	cleaner.TrackUser(guest.ID)

	if _, err := st.AddVerifiedUserEmail(ctx, guest.ID, "linked-"+guest.ID.String()+"@example.com", true); err != nil {
		t.Fatalf("AddVerifiedUserEmail: %v", err)
	}

	user, err := st.GetUserByID(ctx, guest.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if user.IsGuest {
		t.Fatal("expected is_guest false after linking email")
	}
	if user.Email == "" {
		t.Fatal("expected primary email on user row")
	}
}
