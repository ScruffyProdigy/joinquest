package auth

import (
	"net/url"
	"os"
	"strings"

	"github.com/scruffyprodigy/joinquest/internal/returnlink"
)

// LobbyReturnURLForMatch builds the return hub link for a provisioned match.
func LobbyReturnURLForMatch(externalMatchID string) string {
	return returnlink.AppendMatchID(LobbyReturnURL(), externalMatchID)
}

const defaultLobbyIssuer = "http://localhost:8080"

// LobbyIssuer returns the canonical issuer URL for this Lobby deployment.
// Used as JWT iss on seat tokens and as lobbyId on game provision requests.
// Resolution order: LOBBY_ISSUER_URL, LOBBY_PUBLIC_URL, default (local API).
func LobbyIssuer() string {
	if v := normalizeIssuerURL(os.Getenv("LOBBY_ISSUER_URL")); v != "" {
		return v
	}
	if v := normalizeIssuerURL(os.Getenv("LOBBY_PUBLIC_URL")); v != "" {
		return v
	}
	return defaultLobbyIssuer
}

// LobbyReturnURL is the stable return hub games link to after a match.
// Games should append ?match={externalMatchId} so Lobby can route per player.
// Resolution order: LOBBY_RETURN_URL (must include path if set), else public URL + /return.
func LobbyReturnURL() string {
	if v := normalizeIssuerURL(os.Getenv("LOBBY_RETURN_URL")); v != "" {
		return v
	}
	base := LobbyPublicURL()
	if base == "" {
		base = LobbyIssuer()
	}
	return base + "/return"
}

// LobbyPublicURL is the browser-facing JoinQuest origin without a path.
func LobbyPublicURL() string {
	if v := normalizeIssuerURL(os.Getenv("LOBBY_PUBLIC_URL")); v != "" {
		return v
	}
	issuer := LobbyIssuer()
	if strings.Contains(issuer, "localhost:8080") {
		return "http://localhost:5173"
	}
	return issuer
}

// LobbyGraphQLURL is the Lobby GraphQL endpoint (player lookup, match reporting, etc.).
func LobbyGraphQLURL() string {
	return LobbyIssuer() + "/graphql"
}

func normalizeIssuerURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return strings.TrimRight(raw, "/")
	}
	u.Fragment = ""
	u.RawQuery = ""
	u.Path = strings.TrimRight(u.Path, "/")
	out := u.String()
	return strings.TrimRight(out, "/")
}
