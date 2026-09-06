package auth

import "testing"

func TestLobbyGraphQLURL(t *testing.T) {
	t.Setenv("LOBBY_ISSUER_URL", "https://joinquest.cc")
	if got := LobbyGraphQLURL(); got != "https://joinquest.cc/graphql" {
		t.Fatalf("LobbyGraphQLURL() = %q", got)
	}
}

func TestLobbyReturnURLPrefersPublicURL(t *testing.T) {
	t.Setenv("LOBBY_ISSUER_URL", "http://localhost:8080")
	t.Setenv("LOBBY_PUBLIC_URL", "http://localhost:5173")
	if got := LobbyReturnURL(); got != "http://localhost:5173/return" {
		t.Fatalf("LobbyReturnURL() = %q", got)
	}
}

func TestLobbyReturnURLForMatch(t *testing.T) {
	t.Setenv("LOBBY_PUBLIC_URL", "https://joinquest.cc")
	got := LobbyReturnURLForMatch("550e8400-e29b-41d4-a716-446655440000")
	if got != "https://joinquest.cc/return?match=550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("LobbyReturnURLForMatch() = %q", got)
	}
}

func TestLobbyIssuerDefault(t *testing.T) {
	t.Setenv("LOBBY_ISSUER_URL", "")
	t.Setenv("LOBBY_PUBLIC_URL", "")
	if got := LobbyIssuer(); got != defaultLobbyIssuer {
		t.Fatalf("LobbyIssuer() = %q, want %q", got, defaultLobbyIssuer)
	}
}

func TestLobbyIssuerFromPublicURL(t *testing.T) {
	t.Setenv("LOBBY_ISSUER_URL", "")
	t.Setenv("LOBBY_PUBLIC_URL", "https://joinquest.cc/")
	if got := LobbyIssuer(); got != "https://joinquest.cc" {
		t.Fatalf("LobbyIssuer() = %q", got)
	}
}

func TestLobbyIssuerExplicitOverridesPublic(t *testing.T) {
	t.Setenv("LOBBY_PUBLIC_URL", "https://joinquest.cc")
	t.Setenv("LOBBY_ISSUER_URL", "https://api.joinquest.cc/")
	if got := LobbyIssuer(); got != "https://api.joinquest.cc" {
		t.Fatalf("LobbyIssuer() = %q", got)
	}
}

func TestNormalizeIssuerURLStripsQueryAndFragment(t *testing.T) {
	got := normalizeIssuerURL("https://joinquest.cc/auth?x=1#frag")
	if got != "https://joinquest.cc/auth" {
		t.Fatalf("normalizeIssuerURL() = %q", got)
	}
}
