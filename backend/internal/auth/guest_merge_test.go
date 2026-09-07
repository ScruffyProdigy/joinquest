package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/email"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

func TestPreviewLinkEmailDetectsMerge(t *testing.T) {
	service := openAuthTestService(t)
	ctx := context.Background()

	guest, _, err := service.CreateGuestSession(ctx)
	if err != nil {
		t.Fatalf("CreateGuestSession: %v", err)
	}

	existing, err := service.store.CreateUser(ctx, store.CreateUserParams{
		Email:       "merge-preview-" + uuid.NewString() + "@example.com",
		DisplayName: "Existing Player",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	preview, err := service.PreviewLinkEmail(ctx, guest.ID, existing.Email)
	if err != nil {
		t.Fatalf("PreviewLinkEmail: %v", err)
	}
	if !preview.WillMergeAccounts {
		t.Fatal("expected merge preview")
	}
	if preview.MergeSourceDisplayName != "Existing Player" {
		t.Fatalf("expected source display name, got %q", preview.MergeSourceDisplayName)
	}
}

func TestCompleteLinkEmailRequiresMergeConfirmation(t *testing.T) {
	t.Setenv("MAGIC_LINK_BASE_URL", "http://localhost:5173/auth/link?token={token}")
	mailer := &email.CaptureSender{}
	service := openAuthTestServiceWithMailer(t, mailer)
	ctx := context.Background()

	guest, _, err := service.CreateGuestSession(ctx)
	if err != nil {
		t.Fatalf("CreateGuestSession: %v", err)
	}

	existingEmail := "merge-link-" + uuid.NewString() + "@example.com"
	existing, err := service.store.CreateUser(ctx, store.CreateUserParams{
		Email: existingEmail,
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	_ = existing

	if err := service.RequestLinkEmail(ctx, guest.ID, existingEmail); err != nil {
		t.Fatalf("RequestLinkEmail: %v", err)
	}
	if mailer.Last.Code == "" {
		t.Fatal("expected verification code")
	}

	_, _, err = service.CompleteLinkEmailWithCode(ctx, existingEmail, mailer.Last.Code, false)
	if !errors.Is(err, ErrMergeConfirmationRequired) {
		t.Fatalf("expected merge confirmation error, got %v", err)
	}

	user, _, err := service.CompleteLinkEmailWithCode(ctx, existingEmail, mailer.Last.Code, true)
	if err != nil {
		t.Fatalf("CompleteLinkEmailWithCode confirm: %v", err)
	}
	if user.ID != guest.ID {
		t.Fatalf("expected guest account to remain active, got %s", user.ID)
	}
	if user.IsGuest {
		t.Fatal("expected guest flag cleared after linking email")
	}
}
