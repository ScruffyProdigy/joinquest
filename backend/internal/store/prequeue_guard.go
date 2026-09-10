package store

import (
	"fmt"

	"github.com/scruffyprodigy/joinquest/internal/prequeue"
)

// modeOptionGroups reads a mode's declared pre-queue groups.
//
// A declaration that will not parse is an error here, not "no groups". The lobby
// has two ways to be wrong about a mode nobody can describe, and they are not
// equally bad: refusing the join is loud and lands on the developer who shipped
// the manifest, while quietly dropping the picker provisions a match with the
// wrong loadout and tells nobody (JQ-211).
//
// This is the same posture the roster fetch already takes — a game that cannot
// serve a player's choices stops accepting joins for that mode rather than
// guessing — and the same one ModeOffersPreMatchChoice takes when it reads an
// unparseable block as "there is a choice here". Manifest sync rejects a bad
// declaration on the way in, so a row that fails here predates that check.
func modeOptionGroups(mode *GameMode) ([]prequeue.Group, error) {
	if mode == nil {
		return nil, nil
	}
	decl, err := prequeue.Parse(mode.PreQueue)
	if err != nil {
		return nil, fmt.Errorf("store: mode %q has an unreadable pre-queue declaration: %w", mode.ModeKey, err)
	}
	if decl == nil {
		return nil, nil
	}
	return decl.Groups, nil
}

// ensureSelectionsSatisfyMode is the guarantee prequeue.Validate exists to give,
// enforced where every path has to pass: no seat reaches a provision carrying a
// selection its mode would not accept (JQ-211).
//
// It checks the declaration only, never the game's roster. The roster is one
// player's answer at one moment and belongs at pick time; a match must not fail
// to start because somebody's unlocks moved while they waited.
func ensureSelectionsSatisfyMode(mode *GameMode, selections []prequeue.Selection) error {
	groups, err := modeOptionGroups(mode)
	if err != nil {
		return err
	}
	if err := prequeue.ValidateDeclared(groups, selections); err != nil {
		return err
	}
	return nil
}

// selectionsSatisfyMode is the same question asked where the answer is a
// decision rather than a failure: a regroup replaying last round's picks needs
// to know whether they still stand, not to blow up the claim.
func selectionsSatisfyMode(mode *GameMode, selections []prequeue.Selection) bool {
	return ensureSelectionsSatisfyMode(mode, selections) == nil
}
