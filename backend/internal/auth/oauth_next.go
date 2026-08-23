package auth

import "strings"

// oauthNextDestinations maps opaque post-sign-in keys to fixed in-app paths.
//
// Callers pass a key, never a path. The redirect target is therefore always one
// of these compile-time constants, which keeps the OAuth success page free of
// open-redirect and javascript: injection risk even though the key itself
// arrives on the query string.
var oauthNextDestinations = map[string]string{
	"dev-ai":     "/developers?path=ai",
	"dev-manual": "/developers?path=manual",
}

// NormalizeOAuthNextKey returns the key only when it names a known destination.
// Anything else — including near-misses and hostile input — becomes "".
func NormalizeOAuthNextKey(key string) string {
	if _, ok := oauthNextDestinations[key]; ok {
		return key
	}
	return ""
}

// ResolveOAuthNext turns a next key into the absolute URL to land on after
// sign-in, falling back to the lobby home page for unknown keys.
func ResolveOAuthNext(key string) string {
	base := strings.TrimRight(LobbyPublicURL(), "/")
	if path, ok := oauthNextDestinations[NormalizeOAuthNextKey(key)]; ok {
		return base + path
	}
	return base + "/"
}
