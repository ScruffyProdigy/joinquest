package prequeue

import (
	"strings"
	"testing"
)

func helpersGroup() []Group {
	return []Group{{Key: "helpers", Kind: KindLoadout, Label: "Choose your two helpers", Min: 2, Max: 2}}
}

func helpersRoster() []RosterGroup {
	return []RosterGroup{{Key: "helpers", Choices: []Choice{
		{ID: "ferrus"},
		{ID: "tempered"},
		{ID: "chimera"},
		{ID: "rust", Locked: true},
	}}}
}

func TestValidateAcceptsSelectionWithinBounds(t *testing.T) {
	sel := []Selection{{GroupKey: "helpers", OptionIDs: []string{"ferrus", "tempered"}}}

	if err := Validate(helpersGroup(), helpersRoster(), sel); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidateAcceptsNothingWhenModeDeclaresNoGroups(t *testing.T) {
	if err := Validate(nil, nil, nil); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidateRejectsSelectionForModeWithoutGroups(t *testing.T) {
	sel := []Selection{{GroupKey: "helpers", OptionIDs: []string{"ferrus"}}}

	err := Validate(nil, nil, sel)
	if err == nil || !strings.Contains(err.Error(), "does not use pre-queue options") {
		t.Fatalf("Validate err = %v, want a no-options error", err)
	}
}

func TestValidateRejectsMissingRequiredGroup(t *testing.T) {
	err := Validate(helpersGroup(), helpersRoster(), nil)
	if err == nil || !strings.Contains(err.Error(), "helpers") {
		t.Fatalf("Validate err = %v, want an error naming the group", err)
	}
}

func TestValidateAcceptsOmittedOptionalGroup(t *testing.T) {
	groups := []Group{{Key: "skin", Kind: KindCharacter, Label: "Pick a skin", Min: 0, Max: 1}}
	roster := []RosterGroup{{Key: "skin", Choices: []Choice{{ID: "default"}}}}

	if err := Validate(groups, roster, nil); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidateRejectsUnknownGroupKey(t *testing.T) {
	sel := []Selection{{GroupKey: "pets", OptionIDs: []string{"ferrus"}}}

	err := Validate(helpersGroup(), helpersRoster(), sel)
	if err == nil || !strings.Contains(err.Error(), "pets") {
		t.Fatalf("Validate err = %v, want an unknown-group error", err)
	}
}

func TestValidateRejectsRepeatedGroupKey(t *testing.T) {
	// Both entries are individually valid, so only the repetition can fail this.
	sel := []Selection{
		{GroupKey: "helpers", OptionIDs: []string{"ferrus", "tempered"}},
		{GroupKey: "helpers", OptionIDs: []string{"chimera", "ferrus"}},
	}

	err := Validate(helpersGroup(), helpersRoster(), sel)
	if err == nil || !strings.Contains(err.Error(), "more than once") {
		t.Fatalf("Validate err = %v, want a repeated-group error", err)
	}
}

func TestValidateRejectsTooFewSelections(t *testing.T) {
	sel := []Selection{{GroupKey: "helpers", OptionIDs: []string{"ferrus"}}}

	err := Validate(helpersGroup(), helpersRoster(), sel)
	if err == nil || !strings.Contains(err.Error(), "2") {
		t.Fatalf("Validate err = %v, want a bounds error mentioning 2", err)
	}
}

func TestValidateRejectsTooManySelections(t *testing.T) {
	sel := []Selection{{GroupKey: "helpers", OptionIDs: []string{"ferrus", "tempered", "chimera"}}}

	if err := Validate(helpersGroup(), helpersRoster(), sel); err == nil {
		t.Fatal("Validate = nil, want a bounds error for 3 picks in a 2-max group")
	}
}

func TestValidateRejectsDuplicateOptionWithinGroup(t *testing.T) {
	sel := []Selection{{GroupKey: "helpers", OptionIDs: []string{"ferrus", "ferrus"}}}

	err := Validate(helpersGroup(), helpersRoster(), sel)
	if err == nil || !strings.Contains(err.Error(), "twice") {
		t.Fatalf("Validate err = %v, want a duplicate-choice error", err)
	}
}

func TestValidateRejectsOptionTheGameNeverOffered(t *testing.T) {
	sel := []Selection{{GroupKey: "helpers", OptionIDs: []string{"ferrus", "godmode"}}}

	err := Validate(helpersGroup(), helpersRoster(), sel)
	if err == nil || !strings.Contains(err.Error(), "godmode") {
		t.Fatalf("Validate err = %v, want an unknown-option error naming godmode", err)
	}
}

func TestValidateRejectsLockedOption(t *testing.T) {
	sel := []Selection{{GroupKey: "helpers", OptionIDs: []string{"ferrus", "rust"}}}

	err := Validate(helpersGroup(), helpersRoster(), sel)
	if err == nil || !strings.Contains(err.Error(), "locked") {
		t.Fatalf("Validate err = %v, want a locked-option error", err)
	}
}

func TestValidateRejectsGroupMissingFromRoster(t *testing.T) {
	sel := []Selection{{GroupKey: "helpers", OptionIDs: []string{"ferrus", "tempered"}}}

	err := Validate(helpersGroup(), nil, sel)
	if err == nil {
		t.Fatal("Validate = nil, want an error when the game served no roster for a declared group")
	}
}

func TestResolveOrdersGroupsAsDeclaredAndSortsChoices(t *testing.T) {
	groups := []Group{
		{Key: "kit", Kind: KindLoadout, Label: "Kit", Min: 1, Max: 1},
		{Key: "helpers", Kind: KindLoadout, Label: "Helpers", Min: 2, Max: 2},
	}
	roster := []RosterGroup{
		{Key: "helpers", Choices: []Choice{{ID: "ferrus"}, {ID: "tempered"}}},
		{Key: "kit", Choices: []Choice{{ID: "janitor"}}},
	}
	sel := []Selection{
		{GroupKey: "helpers", OptionIDs: []string{"tempered", "ferrus"}},
		{GroupKey: "kit", OptionIDs: []string{"janitor"}},
	}

	got := Resolve(groups, roster, sel)

	if len(got) != 2 || got[0].GroupKey != "kit" || got[1].GroupKey != "helpers" {
		t.Fatalf("Resolve groups = %+v, want declaration order kit,helpers", got)
	}
	if got[1].OptionIDs[0] != "ferrus" || got[1].OptionIDs[1] != "tempered" {
		t.Fatalf("Resolve choices = %v, want sorted ferrus,tempered", got[1].OptionIDs)
	}
}

func TestResolveStampsTheGamesLabelsAtPickTime(t *testing.T) {
	groups := helpersGroup()
	roster := []RosterGroup{{Key: "helpers", Choices: []Choice{
		{ID: "ferrus", Label: "Ferrus"},
		{ID: "tempered", Label: "Tempered"},
	}}}
	sel := []Selection{{GroupKey: "helpers", OptionIDs: []string{"tempered", "ferrus"}}}

	got := Resolve(groups, roster, sel)

	// The waiting screen has to name what the player picked even when the game
	// is unreachable, so the label is captured with the pick rather than
	// re-fetched later.
	if want := []string{"Ferrus", "Tempered"}; len(got[0].Labels) != 2 ||
		got[0].Labels[0] != want[0] || got[0].Labels[1] != want[1] {
		t.Fatalf("Resolve labels = %v, want %v in the same order as the ids", got[0].Labels, want)
	}
}

func TestResolveIgnoresLabelsTheClientSent(t *testing.T) {
	groups := helpersGroup()
	roster := []RosterGroup{{Key: "helpers", Choices: []Choice{
		{ID: "ferrus", Label: "Ferrus"},
		{ID: "tempered", Label: "Tempered"},
	}}}
	sel := []Selection{{
		GroupKey:  "helpers",
		OptionIDs: []string{"ferrus", "tempered"},
		Labels:    []string{"Admin Mode", "Free Win"},
	}}

	got := Resolve(groups, roster, sel)

	if got[0].Labels[0] != "Ferrus" || got[0].Labels[1] != "Tempered" {
		t.Fatalf("Resolve labels = %v, want the game's labels, not the client's", got[0].Labels)
	}
}

func TestResolveFallsBackToTheIDWhenTheGameSentNoLabel(t *testing.T) {
	groups := helpersGroup()
	roster := []RosterGroup{{Key: "helpers", Choices: []Choice{{ID: "ferrus"}, {ID: "tempered"}}}}
	sel := []Selection{{GroupKey: "helpers", OptionIDs: []string{"ferrus", "tempered"}}}

	got := Resolve(groups, roster, sel)

	if got[0].Labels[0] != "ferrus" {
		t.Fatalf("Resolve labels = %v, want the id as a last resort", got[0].Labels)
	}
}

func TestValidateDeclaredAcceptsAValidSelectionWithNoRoster(t *testing.T) {
	sel := []Selection{{GroupKey: "helpers", OptionIDs: []string{"ferrus", "tempered"}}}

	if err := ValidateDeclared(helpersGroup(), sel); err != nil {
		t.Fatalf("ValidateDeclared: %v", err)
	}
}

func TestValidateDeclaredRejectsAMissingRequiredGroup(t *testing.T) {
	err := ValidateDeclared(helpersGroup(), nil)
	if err == nil || !strings.Contains(err.Error(), "helpers") {
		t.Fatalf("ValidateDeclared err = %v, want an error naming the group", err)
	}
}

func TestValidateDeclaredRejectsCountsOutsideTheDeclaredBounds(t *testing.T) {
	sel := []Selection{{GroupKey: "helpers", OptionIDs: []string{"ferrus"}}}

	if err := ValidateDeclared(helpersGroup(), sel); err == nil {
		t.Fatal("ValidateDeclared = nil, want a bounds error for one pick in a 2-of-2 group")
	}
}

// The deliberate limit of the roster-free half. An id the game never offered is
// not a fact a declaration knows, and re-asking the game at provision time would
// fail a started match over unlocks that moved while the player waited (JQ-211).
func TestValidateDeclaredDoesNotJudgeWhetherTheGameOffersTheOption(t *testing.T) {
	sel := []Selection{{GroupKey: "helpers", OptionIDs: []string{"godmode", "ferrus"}}}

	if err := ValidateDeclared(helpersGroup(), sel); err != nil {
		t.Fatalf("ValidateDeclared: %v, want the roster question left to Validate", err)
	}
	if err := Validate(helpersGroup(), helpersRoster(), sel); err == nil {
		t.Fatal("Validate = nil, want the roster check to still reject godmode at pick time")
	}
}
