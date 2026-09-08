package developer

import "testing"

func TestValidateGenre(t *testing.T) {
	if err := ValidateGenre("words-trivia"); err != nil {
		t.Fatalf("expected valid genre: %v", err)
	}
	if err := ValidateGenre(""); err != nil {
		t.Fatalf("expected empty genre to be allowed: %v", err)
	}
	if err := ValidateGenre("words"); err == nil {
		t.Fatal("expected retired tag id to fail as a genre")
	}
}

func TestValidateDifficulty(t *testing.T) {
	if err := ValidateDifficulty("casual"); err != nil {
		t.Fatalf("expected valid difficulty: %v", err)
	}
	if err := ValidateDifficulty("easy"); err == nil {
		t.Fatal("expected unknown difficulty to fail")
	}
}

func TestValidateSocialMode(t *testing.T) {
	if err := ValidateSocialMode("1v1"); err != nil {
		t.Fatalf("expected valid social mode: %v", err)
	}
	if err := ValidateSocialMode("party"); err == nil {
		t.Fatal("expected retired tag id to fail as a social mode")
	}
}

// Every axis value reaches the database through a CHECK constraint written by
// migration 000052, so a value added here without the matching migration would
// pass validation and then fail on write.
func TestAxisIDsAreStable(t *testing.T) {
	for _, tc := range []struct {
		axis string
		want []string
		got  []CatalogOption
	}{
		{"genre", []string{"action", "strategy", "deduction", "words-trivia", "drawing-creative", "puzzle"}, GenreTaxonomy},
		{"difficulty", []string{"casual", "involved", "demanding"}, DifficultyTaxonomy},
		{"social mode", []string{"free-for-all", "1v1", "teams", "hidden-roles", "co-op"}, SocialModeTaxonomy},
	} {
		if len(tc.got) != len(tc.want) {
			t.Fatalf("%s: got %d options, want %d", tc.axis, len(tc.got), len(tc.want))
		}
		for i, id := range tc.want {
			if tc.got[i].ID != id {
				t.Errorf("%s[%d]: got %q, want %q", tc.axis, i, tc.got[i].ID, id)
			}
		}
	}
}

func TestValidateTypicalMinutes(t *testing.T) {
	minutes := func(n int) *int { return &n }

	if err := ValidateTypicalMinutes(nil); err != nil {
		t.Fatalf("expected an undeclared duration to be allowed: %v", err)
	}
	if err := ValidateTypicalMinutes(minutes(12)); err != nil {
		t.Fatalf("expected a typical duration to be valid: %v", err)
	}
	if err := ValidateTypicalMinutes(minutes(MinTypicalMinutes)); err != nil {
		t.Fatalf("expected the lower bound to be valid: %v", err)
	}
	if err := ValidateTypicalMinutes(minutes(MaxTypicalMinutes)); err != nil {
		t.Fatalf("expected the upper bound to be valid: %v", err)
	}
	// Zero is a declaration, not an omission, so it fails rather than quietly
	// reading as "no duration".
	if err := ValidateTypicalMinutes(minutes(0)); err == nil {
		t.Fatal("expected zero minutes to fail")
	}
	if err := ValidateTypicalMinutes(minutes(-5)); err == nil {
		t.Fatal("expected a negative duration to fail")
	}
	// Past a day it cannot be a session length at all.
	if err := ValidateTypicalMinutes(minutes(MaxTypicalMinutes + 1)); err == nil {
		t.Fatal("expected an out-of-range duration to fail")
	}
}

// The CHECK constraint in migration 000053 is the other half of this validation;
// widening the bounds here without widening the migration would pass validation
// and then fail on write.
func TestTypicalMinutesBoundsMatchMigration(t *testing.T) {
	if MinTypicalMinutes != 1 || MaxTypicalMinutes != 1440 {
		t.Fatalf("bounds changed to %d-%d; update game_modes_typical_minutes_check in migration 000053",
			MinTypicalMinutes, MaxTypicalMinutes)
	}
}
