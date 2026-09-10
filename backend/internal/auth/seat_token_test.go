package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestSignSeatTokenClaims(t *testing.T) {
	t.Setenv("LOBBY_ISSUER_URL", "https://issuer.test")

	signer, err := LoadSignerFromEnv()
	if err != nil {
		t.Fatalf("load signer: %v", err)
	}

	userID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	token, err := signer.SignSeatToken(userID, "http://localhost:3001", "session-abc", "a", "Alice", time.Hour)
	if err != nil {
		t.Fatalf("SignSeatToken: %v", err)
	}

	parsed, err := jwt.Parse(token, func(token *jwt.Token) (any, error) {
		return signer.publicKey, nil
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("expected map claims")
	}
	if claims["iss"] != "https://issuer.test" {
		t.Fatalf("iss: %v", claims["iss"])
	}
	if claims["aud"] != "http://localhost:3001" {
		t.Fatalf("aud: %v", claims["aud"])
	}
	if claims["sub"] != userID.String() {
		t.Fatalf("sub: %v", claims["sub"])
	}
	if claims["jti"] == "" {
		t.Fatal("expected jti")
	}
	if claims["matchId"] != "session-abc" {
		t.Fatalf("matchId: %v", claims["matchId"])
	}
	if claims["seatKey"] != "a" {
		t.Fatalf("seatKey: %v", claims["seatKey"])
	}
	if claims["name"] != "Alice" {
		t.Fatalf("name: %v", claims["name"])
	}
	if claims["nbf"] == nil || claims["iat"] == nil || claims["exp"] == nil {
		t.Fatalf("expected nbf/iat/exp, got nbf=%v iat=%v exp=%v", claims["nbf"], claims["iat"], claims["exp"])
	}
}

func TestSignSeatTokenRequiresAudience(t *testing.T) {
	signer, err := LoadSignerFromEnv()
	if err != nil {
		t.Fatalf("load signer: %v", err)
	}
	_, err = signer.SignSeatToken(uuid.New(), "  ", "match", "a", "", time.Hour)
	if err == nil {
		t.Fatal("expected error for empty audience")
	}
}

// TestSeatTokenCarriesNoSkillData is the guard on JQ-141's central line: skill
// travels server-to-server only, and the seat token is the one artifact on this
// path that reaches the player's browser. A player can read and decode their own
// seat token with devtools, so anything in it is public to them.
//
// It works as an allowlist rather than by looking for the word "skill". A future
// change that leaks a rating is unlikely to name it something this test would
// have thought to search for — but it cannot avoid adding a claim, and that is
// what fails here. Adding a legitimate claim means adding it below, which is the
// moment to ask whether the player may see it.
func TestSeatTokenCarriesNoSkillData(t *testing.T) {
	t.Setenv("LOBBY_ISSUER_URL", "https://issuer.test")

	signer, err := LoadSignerFromEnv()
	if err != nil {
		t.Fatalf("load signer: %v", err)
	}

	token, err := signer.SignSeatToken(uuid.New(), "http://localhost:3001", "session-abc", "a", "Alice", time.Hour)
	if err != nil {
		t.Fatalf("SignSeatToken: %v", err)
	}

	parsed, err := jwt.Parse(token, func(token *jwt.Token) (any, error) {
		return signer.publicKey, nil
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("expected map claims")
	}

	allowed := map[string]bool{
		"iss":     true,
		"aud":     true,
		"sub":     true,
		"jti":     true,
		"matchId": true,
		"seatKey": true,
		"name":    true,
		"nbf":     true,
		"iat":     true,
		"exp":     true,
	}
	for claim := range claims {
		if !allowed[claim] {
			t.Errorf("seat token gained an unexpected claim %q. If it is legitimate, add it to the allowlist above — "+
				"but a player can read every claim in their own seat token, so skill data must never be one of them.", claim)
		}
	}

	// The JWT header travels in the clear next to the claims, so it gets the
	// same treatment.
	headerAllowed := map[string]bool{"alg": true, "typ": true, "kid": true}
	for key := range parsed.Header {
		if !headerAllowed[key] {
			t.Errorf("seat token gained an unexpected header %q; the header is as readable as the claims", key)
		}
	}
}
