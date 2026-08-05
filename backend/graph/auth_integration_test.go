package graph

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/google/uuid"
	"github.com/scruffyprodigy/playhub/graph/generated"
	"github.com/scruffyprodigy/playhub/internal/auth"
	"github.com/scruffyprodigy/playhub/internal/email"
	"github.com/scruffyprodigy/playhub/internal/pubsub"
	"github.com/scruffyprodigy/playhub/internal/store"
	"github.com/scruffyprodigy/playhub/internal/testdb"
	_ "github.com/lib/pq"
)

func newAuthGraphQLTestClient(t *testing.T) (*client.Client, http.Handler, *store.Store) {
	return newAuthGraphQLTestClientWithMailer(t, nil)
}

func newAuthGraphQLTestClientWithMailer(t *testing.T, mailer email.Sender) (*client.Client, http.Handler, *store.Store) {
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

	st := store.New(db)
	signer, err := auth.LoadSignerFromEnv()
	if err != nil {
		t.Fatalf("load signer: %v", err)
	}

	var authService *auth.Service
	if mailer == nil {
		authService, err = auth.NewService(st, signer)
	} else {
		authService, err = auth.NewServiceWithMailer(st, signer, mailer)
	}
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}

	resolver := NewResolver(st, authService, pubsub.NewMemory())
	gql := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))
	handlerWithAuth := auth.Middleware(signer, st, gql)
	c := client.New(handlerWithAuth)

	return c, handlerWithAuth, st
}

func postGraphQLWithCookies(t *testing.T, handler http.Handler, query string, variables map[string]any) ([]*http.Cookie, []byte) {
	t.Helper()

	payload := map[string]any{"query": query}
	if variables != nil {
		payload["variables"] = variables
	}

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal GraphQL request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code >= http.StatusBadRequest {
		t.Fatalf("GraphQL request failed with HTTP %d: %s", rec.Code, rec.Body.String())
	}

	return rec.Result().Cookies(), rec.Body.Bytes()
}

func sessionCookieOptions(cookies []*http.Cookie) []client.Option {
	cfg := auth.CookieConfigFromEnv()
	opts := make([]client.Option, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie.Name == cfg.Name && cookie.Value != "" {
			opts = append(opts, client.AddCookie(cookie))
		}
	}
	return opts
}

func TestAuthGraphQLFlow(t *testing.T) {
	t.Setenv("MAGIC_LINK_BASE_URL", "http://localhost:5174/sign-in?token={token}")
	mailer := &email.CaptureSender{}
	c, handlerWithAuth, st := newAuthGraphQLTestClientWithMailer(t, mailer)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	email := "graphql-auth-" + uuid.NewString() + "@example.com"
	cleaner.TrackEmail(email)

	var loginResp struct {
		RequestSignIn bool `json:"requestSignIn"`
	}
	if err := c.Post(`mutation RequestSignIn($email: String!) {
		requestSignIn(email: $email)
	}`, &loginResp, client.Var("email", email)); err != nil {
		t.Fatalf("requestSignIn failed: %v", err)
	}
	if !loginResp.RequestSignIn {
		t.Fatal("expected requestSignIn to return true")
	}

	token, err := auth.MagicLinkTokenFromURL(mailer.Last.Link)
	if err != nil {
		t.Fatalf("MagicLinkTokenFromURL: %v", err)
	}

	completeQuery := `mutation CompleteSignInWithLink($token: ID!) {
		completeSignInWithLink(token: $token) {
			email
			displayName
		}
	}`
	sessionCookies, completeBody := postGraphQLWithCookies(t, handlerWithAuth, completeQuery, map[string]any{"token": token})
	if len(sessionCookieOptions(sessionCookies)) == 0 {
		t.Fatal("expected completeSignInWithLink to set a session cookie")
	}

	var completeResp struct {
		Data struct {
			CompleteSignInWithLink struct {
				Email       *string `json:"email"`
				DisplayName *string `json:"displayName"`
			} `json:"completeSignInWithLink"`
		} `json:"data"`
	}
	if err := json.Unmarshal(completeBody, &completeResp); err != nil {
		t.Fatalf("decode completeSignInWithLink response: %v", err)
	}
	if completeResp.Data.CompleteSignInWithLink.Email == nil || *completeResp.Data.CompleteSignInWithLink.Email != email {
		t.Fatalf("expected completed user email %q, got %+v", email, completeResp.Data.CompleteSignInWithLink.Email)
	}
	if completeResp.Data.CompleteSignInWithLink.DisplayName == nil || *completeResp.Data.CompleteSignInWithLink.DisplayName != store.DefaultDisplayName(email) {
		t.Fatalf("expected provisional display name, got %+v", completeResp.Data.CompleteSignInWithLink.DisplayName)
	}

	user, err := st.GetUserByEmail(ctx, email)
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	cleaner.TrackUser(user.ID)

	var meResp struct {
		Me *struct {
			Email   *string `json:"email"`
			IsAdmin bool    `json:"isAdmin"`
		} `json:"me"`
	}
	if err := c.Post(`query {
		me {
			email
			isAdmin
		}
	}`, &meResp, sessionCookieOptions(sessionCookies)...); err != nil {
		t.Fatalf("me query failed: %v", err)
	}
	if meResp.Me == nil || meResp.Me.Email == nil || *meResp.Me.Email != email {
		t.Fatalf("expected authenticated me for %q, got %+v", email, meResp.Me)
	}
	if meResp.Me.IsAdmin {
		t.Fatalf("expected test user %q not to be admin", email)
	}

	var logoutResp struct {
		Logout bool `json:"logout"`
	}
	if err := c.Post(`mutation {
		logout
	}`, &logoutResp, sessionCookieOptions(sessionCookies)...); err != nil {
		t.Fatalf("logout failed: %v", err)
	}
	if !logoutResp.Logout {
		t.Fatal("expected logout to return true")
	}

	if err := c.Post(`query {
		me {
			email
		}
	}`, &meResp); err != nil {
		t.Fatalf("me query after logout failed: %v", err)
	}
	if meResp.Me != nil && meResp.Me.Email != nil {
		t.Fatalf("expected me to be unauthenticated after logout, got %+v", meResp.Me)
	}
}

func TestAuthGraphQLLoginCodeFlow(t *testing.T) {
	mailer := &email.CaptureSender{}
	c, handlerWithAuth, st := newAuthGraphQLTestClientWithMailer(t, mailer)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	emailAddr := "graphql-code-" + uuid.NewString() + "@example.com"
	cleaner.TrackEmail(emailAddr)

	var loginResp struct {
		RequestSignIn bool `json:"requestSignIn"`
	}
	if err := c.Post(`mutation RequestSignIn($email: String!) {
		requestSignIn(email: $email)
	}`, &loginResp, client.Var("email", emailAddr)); err != nil {
		t.Fatalf("requestSignIn failed: %v", err)
	}
	if mailer.Last.Code == "" {
		t.Fatal("expected login code in email payload")
	}

	completeQuery := `mutation CompleteSignInWithCode($email: String!, $code: String!) {
		completeSignInWithCode(email: $email, code: $code) {
			email
		}
	}`
	sessionCookies, completeBody := postGraphQLWithCookies(t, handlerWithAuth, completeQuery, map[string]any{
		"email": emailAddr,
		"code":  mailer.Last.Code,
	})
	if len(sessionCookieOptions(sessionCookies)) == 0 {
		t.Fatal("expected completeSignInWithCode to set a session cookie")
	}

	var completeResp struct {
		Data struct {
			CompleteSignInWithCode struct {
				Email *string `json:"email"`
			} `json:"completeSignInWithCode"`
		} `json:"data"`
	}
	if err := json.Unmarshal(completeBody, &completeResp); err != nil {
		t.Fatalf("decode completeSignInWithCode response: %v", err)
	}
	if completeResp.Data.CompleteSignInWithCode.Email == nil || *completeResp.Data.CompleteSignInWithCode.Email != emailAddr {
		t.Fatalf("expected completed user email %q, got %+v", emailAddr, completeResp.Data.CompleteSignInWithCode)
	}

	user, err := st.GetUserByEmail(ctx, emailAddr)
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	cleaner.TrackUser(user.ID)
}

func TestAuthGraphQLRejectsInvalidLoginCode(t *testing.T) {
	mailer := &email.CaptureSender{}
	c, _, st := newAuthGraphQLTestClientWithMailer(t, mailer)
	cleaner := st.NewTestCleaner(t)

	emailAddr := "graphql-bad-code-" + uuid.NewString() + "@example.com"
	cleaner.TrackEmail(emailAddr)

	var loginResp struct {
		RequestSignIn bool `json:"requestSignIn"`
	}
	if err := c.Post(`mutation RequestSignIn($email: String!) {
		requestSignIn(email: $email)
	}`, &loginResp, client.Var("email", emailAddr)); err != nil {
		t.Fatalf("requestSignIn failed: %v", err)
	}
	if mailer.Last.Code == "" {
		t.Fatal("expected login code in email payload")
	}

	var completeResp struct {
		CompleteSignInWithCode *struct {
			Email *string `json:"email"`
		} `json:"completeSignInWithCode"`
	}
	err := c.Post(`mutation CompleteSignInWithCode($email: String!, $code: String!) {
		completeSignInWithCode(email: $email, code: $code) {
			email
		}
	}`, &completeResp, client.Var("email", emailAddr), client.Var("code", "000000"))
	if err == nil {
		t.Fatal("expected completeSignInWithCode to fail for invalid code")
	}
	if !strings.Contains(err.Error(), "Invalid or expired code") {
		t.Fatalf("expected friendly invalid code error, got: %v", err)
	}
}

