package seattemplate

import (
	"testing"
)

func TestPathSpecsWordHunt(t *testing.T) {
	specs, err := PathSpecs([]byte(`{
		"ClueGiver":{"displayName":"Clue Giver","name":["Red","Blue","Green"],"min":2,"max":3,"sizeForQueue":2},
		"Guesser":{"count":6,"min":2,"max":6,"sizeForQueue":4}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 2 {
		t.Fatalf("got %d specs, want 2: %+v", len(specs), specs)
	}
	byPath := map[string]PathSpec{}
	for _, spec := range specs {
		byPath[spec.QueuePath] = spec
	}
	cg := byPath["ClueGiver"]
	if cg.DisplayName != "Clue Giver" || cg.SeatCapacity != 3 || cg.PlayersToStart() != 2 || cg.Min != 2 || cg.Max != 3 {
		t.Fatalf("ClueGiver spec = %+v", cg)
	}
	gu := byPath["Guesser"]
	if gu.DisplayName != "Guesser" || gu.SeatCapacity != 6 || gu.PlayersToStart() != 4 || gu.Min != 2 || gu.Max != 6 {
		t.Fatalf("Guesser spec = %+v", gu)
	}
	if TotalPlayersToStart(specs) != 6 {
		t.Fatalf("total fire size = %d, want 6", TotalPlayersToStart(specs))
	}
}

func TestPathSpecsDuelDefaultsToFullTemplate(t *testing.T) {
	specs, err := PathSpecs([]byte(`{"count":2}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1 || specs[0].QueuePath != "" || specs[0].PlayersToStart() != 2 {
		t.Fatalf("got %+v", specs)
	}
}

func TestPathSpecsCompositionRoles(t *testing.T) {
	specs, err := PathSpecs([]byte(`{"Team":{"count":1,"DPS":{"count":1},"Tank":{"count":1}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 2 {
		t.Fatalf("got %+v", specs)
	}
}

func TestDerivedPlayerBoundsFromPathsWordHunt(t *testing.T) {
	specs, err := PathSpecs([]byte(`{
		"ClueGiver":{"displayName":"Clue Giver","name":["Red","Blue","Green"],"min":2,"max":3,"sizeForQueue":2},
		"Guesser":{"count":6,"min":2,"max":6,"sizeForQueue":4}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	min, max := DerivedPlayerBoundsFromPaths(specs)
	if min != 4 || max != 9 {
		t.Fatalf("got (%d, %d), want (4, 9) — matches Word Hunt's explicit top-level Min/Max", min, max)
	}
}

func TestDerivedPlayerBoundsFromPathsFixedTemplateSumsToFullCapacity(t *testing.T) {
	specs, err := PathSpecs([]byte(`{"count":2}`))
	if err != nil {
		t.Fatal(err)
	}
	min, max := DerivedPlayerBoundsFromPaths(specs)
	if min != 2 || max != 2 {
		t.Fatalf("got (%d, %d), want (2, 2) — no per-path min declared, so it should fall back to full seat capacity", min, max)
	}
}

func TestDerivedPlayerBoundsFromPathsSoloTemplate(t *testing.T) {
	specs, err := PathSpecs([]byte(`{"count":1}`))
	if err != nil {
		t.Fatal(err)
	}
	min, max := DerivedPlayerBoundsFromPaths(specs)
	if min != 1 || max != 1 {
		t.Fatalf("got (%d, %d), want (1, 1)", min, max)
	}
}

func TestDerivedPlayerBoundsFromPathsEmpty(t *testing.T) {
	min, max := DerivedPlayerBoundsFromPaths(nil)
	if min != 0 || max != 0 {
		t.Fatalf("got (%d, %d), want (0, 0) so callers fall back to leaf-count derivation", min, max)
	}
}
