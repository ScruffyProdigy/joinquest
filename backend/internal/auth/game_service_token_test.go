package auth

import (
	"testing"

	"github.com/google/uuid"
)

func TestFormatAndParseGameServiceToken(t *testing.T) {
	t.Setenv("LOBBY_GAME_TOKEN_PEPPER", "pepper-secret")
	gameID := uuid.New()

	token, err := FormatGameServiceToken(gameID)
	if err != nil {
		t.Fatalf("FormatGameServiceToken: %v", err)
	}
	parsed, err := ParseGameServiceToken(token)
	if err != nil {
		t.Fatalf("ParseGameServiceToken: %v", err)
	}
	if parsed != gameID {
		t.Fatalf("parsed game id %s, want %s", parsed, gameID)
	}
	if !MatchesGameServiceToken(token) {
		t.Fatal("expected MatchesGameServiceToken to accept per-game token")
	}
}

func TestParseGameServiceTokenRejectsWrongGame(t *testing.T) {
	t.Setenv("LOBBY_GAME_TOKEN_PEPPER", "pepper-secret")
	token, err := FormatGameServiceToken(uuid.New())
	if err != nil {
		t.Fatalf("FormatGameServiceToken: %v", err)
	}
	token = token + "tampered"
	if _, err := ParseGameServiceToken(token); err == nil {
		t.Fatal("expected tampered token to fail")
	}
}
