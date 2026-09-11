package graph

import (
	"encoding/json"
	"testing"
)

// TestMatchResultMarksTheViewersArrivalParty is the API half of the partition (JQ-291):
// `arrivalParty` says, per row, whether that player came into the match with the viewer, so
// the results screen can show a group its own members instead of everyone the match held.
func TestMatchResultMarksTheViewersArrivalParty(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	match := seedFinishedPartyMatch(t, env, cleaner)

	resp := queryMatchResult(t, env, match.sessionID, match.cookieA)
	if len(resp.Errors) > 0 {
		t.Fatalf("matchResult errors: %+v", resp.Errors)
	}
	if resp.Data.MatchResult == nil {
		t.Fatal("matchResult is null")
	}
	for _, p := range resp.Data.MatchResult.Participants {
		if !p.ArrivalParty {
			t.Errorf("player %s reads as arrivalParty=false, but both queued from one room table", p.User.ID)
		}
	}
}

// TestMatchResultGivesASoloViewerNoArrivalParty is the other end of the same rule, and the
// one that keeps two empty parties from collapsing into one: a player who joined the catalog
// queue alone arrived with nobody, so every row reads false — including their own, and
// including the strangers they were matched with.
func TestMatchResultGivesASoloViewerNoArrivalParty(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	match := seedFinishedMatch(t, env, cleaner)

	resp := queryMatchResult(t, env, match.sessionID, match.cookieA)
	if len(resp.Errors) > 0 {
		t.Fatalf("matchResult errors: %+v", resp.Errors)
	}
	if resp.Data.MatchResult == nil {
		t.Fatal("matchResult is null")
	}
	for _, p := range resp.Data.MatchResult.Participants {
		if p.ArrivalParty {
			t.Errorf("player %s reads as arrivalParty=true for a viewer who queued alone", p.User.ID)
		}
	}
}

// TestRegroupRosterIsEmptyForASoloClaimant covers the ghosted-seat field for the same case.
// The roster names an arrival party's members, so a table built for someone who arrived
// alone names nobody — rather than ghosting the strangers that player was matched with onto
// a card offering to bring them back.
func TestRegroupRosterIsEmptyForASoloClaimant(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	match := seedFinishedMatch(t, env, cleaner)

	body := postGraphQL(t, env.Handler, playAgainWithRosterMutation,
		map[string]any{"matchId": match.sessionID.String()}, match.cookieA)
	var resp playAgainWithRosterResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode playAgain: %v body=%s", err, body)
	}
	if len(resp.Errors) > 0 {
		t.Fatalf("playAgain errors: %+v", resp.Errors)
	}
	if resp.Data.PlayAgain == nil {
		t.Fatal("playAgain returned a null result")
	}
	if roster := resp.Data.PlayAgain.Table.RegroupRoster; len(roster) != 0 {
		t.Errorf("roster = %d entries, want none for a player who arrived alone: %+v", len(roster), roster)
	}
}
