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
// migration 000051, so a value added here without the matching migration would
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
