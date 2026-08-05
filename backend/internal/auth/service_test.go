package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/scruffyprodigy/playhub/internal/email"
	"github.com/scruffyprodigy/playhub/internal/store"
	"github.com/scruffyprodigy/playhub/internal/testdb"
)

func openAuthTestService(t *testing.T) *Service {
	t.Helper()
	return openAuthTestServiceWithMailer(t, nil)
}

func openAuthTestServiceWithMailer(t *testing.T, mailer email.Sender) *Service {
	t.Helper()
	t.Setenv("MAGIC_LINK_BASE_URL", "http://localhost:5174/sign-in?token={token}")

	databaseURL := testdb.RequireURL(t)

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		t.Fatalf("ping database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	signer, err := LoadSignerFromEnv()
	if err != nil {
		t.Fatalf("load signer: %v", err)
	}

	if mailer == nil {
		service, err := NewService(store.New(db), signer)
		if err != nil {
			t.Fatalf("new auth service: %v", err)
		}
		return service
	}

	service, err := NewServiceWithMailer(store.New(db), signer, mailer)
	if err != nil {
		t.Fatalf("new auth service with mailer: %v", err)
	}
	return service
}

func TestMagicLinkLoginFlow(t *testing.T) {
	t.Setenv("MAGIC_LINK_BASE_URL", "http://localhost:5174/sign-in?token={token}")
	mailer := &email.CaptureSender{}
	service := openAuthTestServiceWithMailer(t, mailer)
	cleaner := service.store.NewTestCleaner(t)
	ctx := context.Background()

	email := "auth-" + uuid.NewString() + "@example.com"
	cleaner.TrackEmail(email)

	if err := service.RequestMagicLink(ctx, email); err != nil {
		t.Fatalf("RequestMagicLink failed: %v", err)
	}

	link, err := service.store.GetLatestMagicLinkByEmail(ctx, email)
	if err != nil {
		t.Fatalf("GetLatestMagicLinkByEmail failed: %v", err)
	}
	if link.CodeHash == "" {
		t.Fatal("expected login code hash on magic link")
	}
	if link.TokenHash == "" {
		t.Fatal("expected token hash on magic link")
	}

	token, err := MagicLinkTokenFromURL(mailer.Last.Link)
	if err != nil {
		t.Fatalf("MagicLinkTokenFromURL: %v", err)
	}
	user, sessionToken, err := service.CompleteMagicLogin(ctx, token)
	if err != nil {
		t.Fatalf("CompleteMagicLogin failed: %v", err)
	}
	if user.Email != email {
		t.Fatalf("expected email %q, got %q", email, user.Email)
	}
	cleaner.TrackUser(user.ID)

	if sessionToken == "" {
		t.Fatal("expected session token")
	}

	userID, err := service.signer.VerifyUserToken(sessionToken)
	if err != nil {
		t.Fatalf("verify session token: %v", err)
	}
	if userID != user.ID {
		t.Fatalf("expected user id %s in token, got %s", user.ID, userID)
	}

	authCtx := WithUserID(ctx, user.ID.String())
	found, err := service.GetAuthenticatedUser(authCtx)
	if err != nil {
		t.Fatalf("GetAuthenticatedUser failed: %v", err)
	}
	if found == nil || found.ID != user.ID {
		t.Fatalf("expected authenticated user %s, got %+v", user.ID, found)
	}

	if _, _, err := service.CompleteMagicLogin(ctx, token); err == nil {
		t.Fatal("expected reused magic link to fail")
	}
}

func TestLoginCodeFlow(t *testing.T) {
	mailer := &email.CaptureSender{}
	service := openAuthTestServiceWithMailer(t, mailer)
	cleaner := service.store.NewTestCleaner(t)
	ctx := context.Background()

	emailAddr := "auth-code-" + uuid.NewString() + "@example.com"
	cleaner.TrackEmail(emailAddr)

	if err := service.RequestMagicLink(ctx, emailAddr); err != nil {
		t.Fatalf("RequestMagicLink failed: %v", err)
	}
	if mailer.Last.Code == "" {
		t.Fatal("expected login code in email payload")
	}
	if !loginCodePattern.MatchString(mailer.Last.Code) {
		t.Fatalf("expected 6-digit code, got %q", mailer.Last.Code)
	}

	user, sessionToken, err := service.CompleteLoginCode(ctx, emailAddr, mailer.Last.Code)
	if err != nil {
		t.Fatalf("CompleteLoginCode failed: %v", err)
	}
	cleaner.TrackUser(user.ID)

	if sessionToken == "" {
		t.Fatal("expected session token")
	}

	if _, _, err := service.CompleteLoginCode(ctx, emailAddr, mailer.Last.Code); err == nil {
		t.Fatal("expected reused login code to fail")
	}
}

func TestCompleteLoginCodeRejectsInvalidCode(t *testing.T) {
	mailer := &email.CaptureSender{}
	service := openAuthTestServiceWithMailer(t, mailer)
	cleaner := service.store.NewTestCleaner(t)
	ctx := context.Background()

	emailAddr := "auth-bad-code-" + uuid.NewString() + "@example.com"
	cleaner.TrackEmail(emailAddr)

	if err := service.RequestMagicLink(ctx, emailAddr); err != nil {
		t.Fatalf("RequestMagicLink failed: %v", err)
	}

	if _, _, err := service.CompleteLoginCode(ctx, emailAddr, "000000"); err != ErrInvalidLoginCode {
		t.Fatalf("expected ErrInvalidLoginCode, got %v", err)
	}
}

func TestRequestMagicLinkRejectsInvalidEmail(t *testing.T) {
	service := openAuthTestService(t)
	err := service.RequestMagicLink(context.Background(), "not-an-email")
	if err != ErrInvalidEmail {
		t.Fatalf("expected ErrInvalidEmail, got %v", err)
	}
}

type failingMailer struct{}

func (failingMailer) SendMagicLink(context.Context, email.MagicLinkEmail) error {
	return fmt.Errorf("smtp rate limit exceeded")
}

func TestRequestMagicLinkFailsWhenEmailDeliveryFails(t *testing.T) {
	t.Setenv("MAGIC_LINK_BASE_URL", "http://localhost:5173/auth/complete?token={token}")
	service := openAuthTestServiceWithMailer(t, failingMailer{})
	cleaner := service.store.NewTestCleaner(t)
	ctx := context.Background()

	emailAddr := "mailfail-" + uuid.NewString() + "@example.com"
	cleaner.TrackEmail(emailAddr)

	err := service.RequestMagicLink(ctx, emailAddr)
	if err == nil {
		t.Fatal("expected error when mailer fails")
	}
	if !errors.Is(err, ErrSignInEmailNotSent) {
		t.Fatalf("expected ErrSignInEmailNotSent, got %v", err)
	}

	count, err := service.store.CountRecentMagicLinksByEmail(ctx, emailAddr, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("CountRecentMagicLinksByEmail: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected magic link rolled back, count=%d", count)
	}
}

func TestLogoutClearsSessionCookie(t *testing.T) {
	service := openAuthTestService(t)

	recorder := httptest.NewRecorder()
	ctx := WithResponseWriter(context.Background(), recorder)
	service.Logout(ctx)

	cookies := recorder.Result().Cookies()
	cfg := service.CookieConfig()

	var sessionCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == cfg.Name {
			sessionCookie = cookie
			break
		}
	}

	if sessionCookie == nil {
		t.Fatalf("expected %q cookie to be cleared", cfg.Name)
	}
	if sessionCookie.Value != "" {
		t.Fatalf("expected empty session cookie value, got %q", sessionCookie.Value)
	}
	if sessionCookie.MaxAge >= 0 {
		t.Fatalf("expected session cookie MaxAge < 0, got %d", sessionCookie.MaxAge)
	}
}
