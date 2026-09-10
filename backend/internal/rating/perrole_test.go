package rating

import (
	"context"
	"fmt"
	"math"
	"testing"
)

// wordHuntShape is a two-seat asymmetric mode in the shape of the one that
// motivated JQ-229: every team fields one Clue Giver and one Guesser, and the
// queue offers them as separate paths.
func wordHuntShape() ModeShape {
	return ModeShape{SeatClasses: map[string]string{
		"A-cg": "ClueGiver",
		"A-g":  "Guesser",
		"B-cg": "ClueGiver",
		"B-g":  "Guesser",
	}}
}

// wordHuntMatch builds one 2v2 match: each team is a Clue Giver and a
// Guesser, and teamAWins says which side took it. It goes through BuildSides
// rather than hand-writing entrant keys, so these tests exercise the same
// translation the result path uses and cannot drift from it.
func wordHuntMatch(t *testing.T, sessionID string, aCG, aG, bCG, bG string, teamAWins bool) Input {
	t.Helper()
	sides, err := BuildSides(wordHuntShape(), MatchOutcome{
		Participants: []Participant{
			{PlayerID: aCG, SeatKey: "A-cg", TeamKey: "A", IsWinner: teamAWins},
			{PlayerID: aG, SeatKey: "A-g", TeamKey: "A", IsWinner: teamAWins},
			{PlayerID: bCG, SeatKey: "B-cg", TeamKey: "B", IsWinner: !teamAWins},
			{PlayerID: bG, SeatKey: "B-g", TeamKey: "B", IsWinner: !teamAWins},
		},
	})
	if err != nil {
		t.Fatalf("BuildSides for %s: %v", sessionID, err)
	}
	return Input{SessionID: sessionID, Sides: sides}
}

// specialistHistory is the case the ticket opens with: dana wins whenever she
// calls the clues and loses whenever she guesses, while everyone else rotates
// through both seats so the seat classes stay identifiable.
//
// Her mode-level record is an even split, so a rating keyed only by
// (player, game, mode) would land her near the middle and describe neither
// half of it.
func specialistHistory(t *testing.T) []Input {
	t.Helper()
	pool := []string{"e", "f", "g", "h", "i", "j"}
	var out []Input
	for k := 0; k < 8; k++ {
		out = append(out, wordHuntMatch(t, fmt.Sprintf("cg%d", k),
			"dana", pool[k%6], pool[(k+2)%6], pool[(k+4)%6], true))
	}
	for k := 0; k < 8; k++ {
		out = append(out, wordHuntMatch(t, fmt.Sprintf("gu%d", k),
			pool[(k+1)%6], "dana", pool[(k+3)%6], pool[(k+5)%6], false))
	}
	return out
}

func replayInputs(t *testing.T, inputs []Input) (*fakeStore, Report) {
	t.Helper()
	e, err := NewWengLin("plackett-luce")
	if err != nil {
		t.Fatalf("NewWengLin: %v", err)
	}
	fs := &fakeStore{inputs: inputs}
	rep, err := NewReplayer(e, fs).ReplayMode(context.Background(), "game", "wordhunt")
	if err != nil {
		t.Fatalf("ReplayMode: %v", err)
	}
	return fs, rep
}

// A player who is strong in one seat and weak in another must end up with
// per-role ratings that say so, while their mode-level rating still reflects
// the combination of the two rather than either one alone.
func TestPerRoleRatingsSeparateAPlayersSeats(t *testing.T) {
	fs, _ := replayInputs(t, specialistHistory(t))

	clueGiver, ok := fs.players[PlayerSeatKey("dana", "ClueGiver")]
	if !ok {
		t.Fatalf("no Clue Giver rating for dana; players = %v", keysOfRatings(fs.players))
	}
	guesser, ok := fs.players[PlayerSeatKey("dana", "Guesser")]
	if !ok {
		t.Fatalf("no Guesser rating for dana; players = %v", keysOfRatings(fs.players))
	}
	mode, ok := fs.players[PlayerKey("dana")]
	if !ok {
		t.Fatalf("no mode-level rating for dana; players = %v", keysOfRatings(fs.players))
	}

	if clueGiver.Mu <= guesser.Mu {
		t.Errorf("dana rates %v as Clue Giver and %v as Guesser; the seat she always wins in should rate higher",
			clueGiver.Mu, guesser.Mu)
	}

	// "Visibly different" has to mean more than a rounding difference, or the
	// per-role split would be a number nobody could act on. A full point of mu
	// is well inside what eight decisive matches per seat should produce and
	// well outside floating-point noise.
	if gap := clueGiver.Mu - guesser.Mu; gap < 1.0 {
		t.Errorf("per-role gap for dana is %v, want a visible separation", gap)
	}

	// The mode-level rating is estimated alongside the per-role ones rather
	// than aggregated from them, but it still has to describe the whole
	// record: dana's even split of wins and losses belongs between her two
	// seats, not outside them.
	if mode.Mu >= clueGiver.Mu || mode.Mu <= guesser.Mu {
		t.Errorf("dana's mode-level rating %v is outside her per-role ratings (%v as Clue Giver, %v as Guesser); it should reflect the combination",
			mode.Mu, clueGiver.Mu, guesser.Mu)
	}
}

// A player's first appearance in a seat must start from their own mode-level
// estimate, widened — not from the global prior, and not from what they rate
// in some other seat.
func TestNewSeatSeedsFromModeLevelEstimateWidened(t *testing.T) {
	e, _ := NewWengLin("plackett-luce")
	r := NewReplayer(e, &fakeStore{})

	mode := Rating{Mu: 31.5, Sigma: 2.4}
	ratings := map[string]Rating{
		PlayerKey("dana"): mode,
		// dana is already well established as a Clue Giver. Her Guesser
		// rating must not be seeded from this.
		PlayerSeatKey("dana", "ClueGiver"): {Mu: 6.0, Sigma: 0.5},
	}

	seeded := r.seed(PlayerSeatKey("dana", "Guesser"), ratings)
	got := combineSeat(mode, seeded)

	if got.Mu != mode.Mu {
		t.Errorf("a new seat rates dana at %v, want her mode-level estimate %v", got.Mu, mode.Mu)
	}
	if got.Mu == mode.Mu+6.0 {
		t.Errorf("a new seat inherited dana's Clue Giver estimate directly")
	}
	if got.Mu == UnratedMu || got.Sigma == UnratedSigma {
		t.Errorf("a new seat fell back to the global prior (%v, %v)", UnratedMu, UnratedSigma)
	}
	if got.Sigma <= mode.Sigma {
		t.Errorf("a new seat carries uncertainty %v, want it widened beyond the mode-level %v", got.Sigma, mode.Sigma)
	}
	if want := mode.Sigma * math.Sqrt2; math.Abs(got.Sigma-want) > 1e-9 {
		t.Errorf("a new seat carries uncertainty %v, want %v at InteractionSigmaRatio %v", got.Sigma, want, InteractionSigmaRatio)
	}
}

// An unrated player entering a seat has no mode-level estimate to inherit, so
// the widening has to fall back to the global prior rather than to zero
// uncertainty — which would otherwise pin their first seat rating in place.
func TestNewSeatForUnratedPlayerWidensThePrior(t *testing.T) {
	e, _ := NewWengLin("plackett-luce")
	r := NewReplayer(e, &fakeStore{})

	seeded := r.seed(PlayerSeatKey("newcomer", "Guesser"), map[string]Rating{})
	got := combineSeat(e.Prior(), seeded)

	if got.Mu != e.Prior().Mu {
		t.Errorf("an unrated player's first seat rates them at %v, want the prior %v", got.Mu, e.Prior().Mu)
	}
	if got.Sigma <= e.Prior().Sigma {
		t.Errorf("an unrated player's first seat carries uncertainty %v, want it above the prior %v", got.Sigma, e.Prior().Sigma)
	}
}

// A symmetric mode has no seat classes to separate, so nothing about it
// changes: one rating per player, exactly as JQ-139 left it.
func TestSymmetricModeKeepsASingleRatingPerPlayer(t *testing.T) {
	fs, _ := replayInputs(t, threeMatchHistory())

	for key := range fs.players {
		if _, seatClass, _ := ParsePlayerKey(key); seatClass != "" {
			t.Errorf("symmetric mode produced a per-seat rating %q", key)
		}
	}
	if len(fs.players) != 3 {
		t.Errorf("symmetric mode produced %d player ratings, want 3", len(fs.players))
	}
}

func keysOfRatings(m map[string]Rating) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// lockedSeatHistory is a mode nobody ever switches seats in: the clue givers
// only ever give clues and the guessers only ever guess. This is the regime
// the ticket warns about — role-based queue paths actively encourage it — and
// in it a seat's own worth and the skill of the players who always hold it
// explain exactly the same variation.
func lockedSeatHistory(t *testing.T) []Input {
	t.Helper()
	givers := []string{"cg1", "cg2", "cg3"}
	guessers := []string{"gu1", "gu2", "gu3"}
	var out []Input
	for k := 0; k < 12; k++ {
		out = append(out, wordHuntMatch(t, fmt.Sprintf("s%d", k),
			givers[k%3], guessers[k%3],
			givers[(k+1)%3], guessers[(k+2)%3],
			k%2 == 0))
	}
	return out
}

// Where every player specializes, per-role ratings still have to be produced
// — they are the quantity that is actually identified — but the seat class's
// own rating must be reported as unidentifiable rather than stated as
// confidently as a number computed from history that can separate the two.
func TestFullSpecializationFlagsTheSeatModifierNotThePerRoleRatings(t *testing.T) {
	fs, rep := replayInputs(t, lockedSeatHistory(t))

	// The per-role ratings exist and are the thing worth having here.
	for _, want := range []string{
		PlayerSeatKey("cg1", "ClueGiver"),
		PlayerSeatKey("gu1", "Guesser"),
	} {
		if _, ok := fs.players[want]; !ok {
			t.Errorf("no per-role rating %q; players = %v", want, keysOfRatings(fs.players))
		}
	}

	seat, ok := rep.Categories[seatCategory]
	if !ok {
		t.Fatalf("report has no seat category; categories = %v", rep.Categories)
	}
	if seat.Identifiable() {
		t.Errorf("seat category reads as identifiable when every player is locked to one seat: %+v", seat)
	}
	if rate := seat.SpecializationRate(); rate != 1 {
		t.Errorf("specialization rate = %v, want 1 when nobody ever switches seats", rate)
	}
	if seat.PlayersWithMultipleValues != 0 {
		t.Errorf("%d players read as having held more than one seat, want 0", seat.PlayersWithMultipleValues)
	}

	for key, m := range rep.Modifiers {
		if m.Identifiable() {
			t.Errorf("modifier %q reads as identifiable under full specialization: %+v", key, m)
		}
	}
}

// The same shape of history, with players rotating through both seats, has to
// come out the other way — otherwise the check above would pass for a report
// that simply always cries confounding.
func TestRotatingSeatsReadAsIdentifiable(t *testing.T) {
	_, rep := replayInputs(t, specialistHistory(t))

	seat, ok := rep.Categories[seatCategory]
	if !ok {
		t.Fatalf("report has no seat category; categories = %v", rep.Categories)
	}
	if !seat.Identifiable() {
		t.Errorf("seat category reads as unidentifiable when players rotate seats: %+v", seat)
	}
	if rate := seat.SpecializationRate(); rate != 0 {
		t.Errorf("specialization rate = %v, want 0 when every player holds both seats", rate)
	}
}

// A seat only a handful of people have ever sat in has to read as thin even
// when it has plenty of matches behind it, which is a fact about how many
// players carried it and not about how often it appeared.
func TestSeatReportCountsDistinctPlayers(t *testing.T) {
	_, rep := replayInputs(t, lockedSeatHistory(t))

	giver, ok := rep.Modifiers[SeatKey("ClueGiver")]
	if !ok {
		t.Fatalf("report has no Clue Giver modifier; modifiers = %v", rep.Modifiers)
	}
	if giver.Matches != 12 {
		t.Errorf("Clue Giver matches = %d, want 12", giver.Matches)
	}
	if giver.DistinctPlayers != 3 {
		t.Errorf("Clue Giver distinct players = %d, want the 3 who ever held it", giver.DistinctPlayers)
	}
}

// Before JQ-229 the report inferred which seat a player held from who else
// was on their side, so on a side holding one Clue Giver and one Guesser both
// players were credited with both seats. Under full specialization that made
// a perfectly confounded mode report as identifiable — the statistic saying
// the opposite of the truth, in exactly the regime it exists to detect.
//
// The fix is that a version 2 input row names the seat each player held. This
// pins that down by replaying the same locked history twice: once as it is
// recorded now, and once with the interaction entrants stripped out, which is
// what a version 1 row looks like.
func TestSeatAttributionComesFromTheInteractionEntrantNotCoPresence(t *testing.T) {
	locked := lockedSeatHistory(t)

	withInteractions := Identifiability(locked)
	if withInteractions.Categories[seatCategory].Identifiable() {
		t.Errorf("seat category is identifiable from version 2 rows: %+v", withInteractions.Categories[seatCategory])
	}

	asVersion1 := Identifiability(stripInteractionEntrants(locked))
	if !asVersion1.Categories[seatCategory].Identifiable() {
		t.Fatalf("co-presence no longer overstates identifiability, so this regression test proves nothing; " +
			"check that stripInteractionEntrants still produces a version 1 shape")
	}
}

// stripInteractionEntrants removes every per-seat entrant, reproducing the
// shape of a version 1 input row.
func stripInteractionEntrants(inputs []Input) []Input {
	out := make([]Input, len(inputs))
	for i, in := range inputs {
		sides := make([]Side, len(in.Sides))
		for j, side := range in.Sides {
			kept := make([]Entrant, 0, len(side.Entrants))
			for _, e := range side.Entrants {
				if _, seatClass, isPlayer := ParsePlayerKey(e.Key); isPlayer && seatClass != "" {
					continue
				}
				kept = append(kept, e)
			}
			sides[j] = Side{Entrants: kept, Rank: side.Rank}
		}
		out[i] = Input{SessionID: in.SessionID, Sides: sides}
	}
	return out
}
