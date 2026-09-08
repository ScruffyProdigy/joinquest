package gameclient

import (
	"strings"
	"testing"

	"github.com/scruffyprodigy/joinquest/internal/prequeue"
)

const helpersModeJSON = `{"modes":[{
	"key":"duel-helpers","displayName":"Helpers","seatTemplate":{"count":2},
	"preQueue":{"groups":[{"key":"helpers","kind":"Loadout","label":"Choose your two helpers","min":2,"max":2}]}
}]}`

func TestParseGameModesCarriesPreQueue(t *testing.T) {
	modes, err := parseGameModes([]byte(helpersModeJSON))
	if err != nil {
		t.Fatalf("parseGameModes: %v", err)
	}

	decl, err := prequeue.Parse(modes[0].PreQueue)
	if err != nil {
		t.Fatalf("prequeue.Parse: %v", err)
	}
	if decl == nil || len(decl.Groups) != 1 || decl.Groups[0].Key != "helpers" {
		t.Fatalf("declaration = %+v, want the helpers group", decl)
	}
	if decl.Groups[0].Min != 2 || decl.Groups[0].Max != 2 {
		t.Fatalf("bounds = %d..%d, want 2..2", decl.Groups[0].Min, decl.Groups[0].Max)
	}
}

func TestValidateModesAcceptsAModeWithoutPreQueue(t *testing.T) {
	modes, err := parseGameModes([]byte(`{"modes":[{"key":"duel","seatTemplate":{"count":2}}]}`))
	if err != nil {
		t.Fatalf("parseGameModes: %v", err)
	}
	if err := validateModes(modes); err != nil {
		t.Fatalf("validateModes: %v", err)
	}
}

func TestValidateModesRejectsUnusablePreQueue(t *testing.T) {
	// A manifest the lobby cannot act on must fail the sync loudly rather than
	// syncing a mode whose picker can never render.
	modes, err := parseGameModes([]byte(`{"modes":[{
		"key":"duel-helpers","seatTemplate":{"count":2},
		"preQueue":{"groups":[{"key":"helpers","kind":"Pet","label":"Pick a pet"}]}
	}]}`))
	if err != nil {
		t.Fatalf("parseGameModes: %v", err)
	}

	err = validateModes(modes)
	if err == nil || !strings.Contains(err.Error(), "duel-helpers") {
		t.Fatalf("validateModes err = %v, want an error naming the mode", err)
	}
}
