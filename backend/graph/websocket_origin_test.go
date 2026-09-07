package graph

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/scruffyprodigy/joinquest/internal/auth"
)

func TestAuthWebSocketOriginAllowedForBrowserOrigins(t *testing.T) {
	for _, origin := range []string{
		"http://localhost:5173",
		"http://127.0.0.1:5173",
		"http://[::1]:5173",
	} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Origin", origin)
		if !auth.WebSocketOriginAllowed(req) {
			t.Fatalf("expected origin %q to be allowed", origin)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://evil.example")
	if auth.WebSocketOriginAllowed(req) {
		t.Fatal("expected foreign origin to be rejected")
	}
}
