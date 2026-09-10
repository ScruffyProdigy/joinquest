package auth

import (
	"crypto/ed25519"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const defaultSeatTokenTTL = 2 * time.Hour

// RejoinSeatTokenTTL is the life of a seat token minted to put a player back into a
// match they are already seated in (JQ-86). It is deliberately far shorter than the
// launch TTL: a rejoin link is handed out on demand, at the moment the player asks
// for it, so it only has to survive the navigation. Keeping the window small is what
// stops re-issue from being a way to pass your seat to someone else.
const RejoinSeatTokenTTL = 5 * time.Minute

// SignSeatToken issues a short-lived JWT for launching into a third-party game.
// Claims: iss, aud, sub, jti, matchId, seatKey, name (optional), nbf, iat, exp.
func (s *Signer) SignSeatToken(userID uuid.UUID, audience, externalMatchID, seatKey, displayName string, ttl time.Duration) (string, error) {
	return s.signSeatToken(s.kid, s.privateKey, userID, LobbyIssuer(), audience, externalMatchID, seatKey, displayName, ttl)
}

// SignSeatTokenWithIssuer signs a seat token using a custom iss claim (integration checks).
func (s *Signer) SignSeatTokenWithIssuer(userID uuid.UUID, issuer, audience, externalMatchID, seatKey, displayName string, ttl time.Duration) (string, error) {
	if strings.TrimSpace(issuer) == "" {
		return "", fmt.Errorf("auth: issuer is required")
	}
	return s.signSeatToken(s.kid, s.privateKey, userID, strings.TrimSpace(issuer), audience, externalMatchID, seatKey, displayName, ttl)
}

// SignSeatTokenWithKey signs a seat token under an alternate (kid, private
// key) pair instead of the signer's primary key. Used together with
// AddRotationKey to mint a token that only verifies against a temporary
// rotation key.
func (s *Signer) SignSeatTokenWithKey(kid string, priv ed25519.PrivateKey, userID uuid.UUID, audience, externalMatchID, seatKey, displayName string, ttl time.Duration) (string, error) {
	return s.signSeatToken(kid, priv, userID, LobbyIssuer(), audience, externalMatchID, seatKey, displayName, ttl)
}

func (s *Signer) signSeatToken(kid string, priv ed25519.PrivateKey, userID uuid.UUID, issuer, audience, externalMatchID, seatKey, displayName string, ttl time.Duration) (string, error) {
	audience = normalizeAudience(audience)
	if audience == "" {
		return "", fmt.Errorf("auth: audience is required")
	}
	if externalMatchID == "" || seatKey == "" {
		return "", fmt.Errorf("auth: match id and seat key are required")
	}
	if ttl == 0 {
		ttl = defaultSeatTokenTTL
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iss":     issuer,
		"aud":     audience,
		"sub":     userID.String(),
		"jti":     uuid.NewString(),
		"matchId": externalMatchID,
		"seatKey": seatKey,
		"nbf":     now.Unix(),
		"iat":     now.Unix(),
		"exp":     now.Add(ttl).Unix(),
	}
	if displayName != "" {
		claims["name"] = displayName
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = kid
	return token.SignedString(priv)
}

func normalizeAudience(audience string) string {
	return strings.TrimRight(strings.TrimSpace(audience), "/")
}
