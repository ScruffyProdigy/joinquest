package ratingbacktest

import (
	"context"
	"reflect"
	"testing"

	"github.com/scruffyprodigy/joinquest/internal/rating"
)

// fakeSource is an in-memory Source, so the harness is tested without Postgres.
type fakeSource struct {
	inputs    []rating.Input
	listCalls int
}

func (f *fakeSource) ListInputs(context.Context, string, string) ([]rating.Input, error) {
	f.listCalls++
	return f.inputs, nil
}

// writableSource has the full rating.Store surface, including SaveAll. It
// exists to catch a future widening of Source: today the harness holds it
// through an interface with no write method, so SaveAll is unreachable by
// construction, and this fake fails loudly the moment that stops being true.
type writableSource struct {
	fakeSource
	t *testing.T
}

func (w *writableSource) SaveAll(_ context.Context, _, _, _ string, _, _ map[string]rating.Rating) error {
	w.t.Fatal("backtest wrote ratings: a dry run must not mutate player_ratings or nonplayer_ratings")
	return nil
}

func wengLin(t *testing.T) rating.Engine {
	t.Helper()
	e, err := rating.NewWengLin("plackett-luce")
	if err != nil {
		t.Fatalf("NewWengLin: %v", err)
	}
	return e
}

func run(t *testing.T, src Source, split Split, engines ...rating.Engine) Report {
	t.Helper()
	h, err := New(src, split)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	rep, err := h.Run(context.Background(), []Target{{GameID: "g", ModeKey: "duel"}}, engines...)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(rep.Modes) != 1 {
		t.Fatalf("got %d modes, want 1", len(rep.Modes))
	}
	return rep
}

// TestHarnessDistinguishesBetterFromWorse is the acceptance criterion that
// makes every other one worth having. History is generated from a known
// latent-skill model, and three engines of known relative quality are scored
// over it: one that recovers skill, one that cannot learn at all, and one that
// learns the outcome backwards. The harness must rank them in that order, and
// must do so on both the ordering metric and the probabilistic one.
func TestHarnessDistinguishesBetterFromWorse(t *testing.T) {
	inputs, _ := syntheticHistory(12, 1200, 1.5, 20260910)
	src := &fakeSource{inputs: inputs}

	good := wengLin(t)
	rep := run(t, src, DefaultSplit, good, staticEngine{}, invertedEngine{inner: good})

	scores := map[string]Score{}
	for _, es := range rep.Modes[0].Engines {
		scores[es.EngineID] = es.Score
	}

	learned := scores[good.ID()]
	static := scores[staticEngine{}.ID()]
	inverted := scores[invertedEngine{}.ID()]

	// Logged so a human running -v sees the margins, not just a pass: a test
	// that starts squeaking by is a warning the harness has lost resolution.
	for _, es := range rep.Modes[0].Engines {
		t.Logf("%-18s accuracy=%.4f logloss=%.4f brier=%.4f over %d held-out matches",
			es.EngineID, es.Score.Accuracy, es.Score.LogLoss, es.Score.Brier, es.Score.HeldOut)
	}

	if !(learned.Accuracy > static.Accuracy) {
		t.Errorf("engine that learns did not beat one that cannot: %.4f vs %.4f", learned.Accuracy, static.Accuracy)
	}
	if !(static.Accuracy > inverted.Accuracy) {
		t.Errorf("engine that cannot learn did not beat one that learns backwards: %.4f vs %.4f", static.Accuracy, inverted.Accuracy)
	}
	// A model that knows nothing sits at chance by construction: every side
	// looks identical to it, so every pair scores half.
	if static.Accuracy != 0.5 {
		t.Errorf("static engine accuracy = %.4f, want exactly 0.5 (every pair a tie)", static.Accuracy)
	}
	// Well above chance, not merely above the strawmen — otherwise the
	// ordering above could hold while the harness measured nothing real.
	if learned.Accuracy < 0.6 {
		t.Errorf("accuracy over skill-generated history = %.4f, want > 0.6", learned.Accuracy)
	}

	if !learned.Probabilistic || !inverted.Probabilistic {
		t.Fatalf("expected both predicting engines to report probabilistic scores")
	}
	if !(learned.LogLoss < inverted.LogLoss) {
		t.Errorf("log-loss did not separate the engines: %.4f vs inverted %.4f", learned.LogLoss, inverted.LogLoss)
	}
	if !(learned.Brier < inverted.Brier) {
		t.Errorf("Brier did not separate the engines: %.4f vs inverted %.4f", learned.Brier, inverted.Brier)
	}
}

// TestNonPredictorReportsNoProbabilisticScore proves the harness reports the
// absence of a probabilistic score rather than a zero that reads like one.
func TestNonPredictorReportsNoProbabilisticScore(t *testing.T) {
	inputs, _ := syntheticHistory(8, 100, 1.5, 7)
	rep := run(t, &fakeSource{inputs: inputs}, DefaultSplit, staticEngine{})

	got := rep.Modes[0].Engines[0].Score
	if got.Probabilistic {
		t.Fatal("static engine does not implement rating.Predictor but was reported as probabilistic")
	}
	if got.ProbMatches != 0 || got.LogLoss != 0 || got.Brier != 0 {
		t.Errorf("expected no probabilistic metrics, got %+v", got)
	}
	if got.HeldOut == 0 {
		t.Error("ordering score should still have been computed")
	}
}

// TestScoreIsDeterministic mirrors rating's own determinism guarantee: the
// harness is only useful if two runs over the same history agree exactly, or
// a rerun looks like a model change.
func TestScoreIsDeterministic(t *testing.T) {
	inputs, _ := syntheticHistory(10, 400, 1.5, 99)
	engine := wengLin(t)

	first := run(t, &fakeSource{inputs: inputs}, DefaultSplit, engine)
	for i := range 50 {
		got := run(t, &fakeSource{inputs: inputs}, DefaultSplit, engine)
		if !reflect.DeepEqual(first, got) {
			t.Fatalf("run %d differs from first run:\nfirst = %+v\ngot   = %+v", i, first, got)
		}
	}
}

// TestDryRunNeverWrites is the "must not mutate the live cache" criterion. The
// source handed in here is fully capable of writing; the harness must not do
// it. See writableSource on why the test is not vacuous.
func TestDryRunNeverWrites(t *testing.T) {
	inputs, _ := syntheticHistory(6, 60, 1.5, 3)
	src := &writableSource{fakeSource: fakeSource{inputs: inputs}, t: t}

	// Assigning through rating.Store first proves the fake really does carry
	// the write method the harness is being trusted not to reach for.
	var _ rating.Store = src

	run(t, src, DefaultSplit, wengLin(t))
}

// TestEnginesShareOneRead is what makes two engines comparable: a single fetch
// of the history, replayed per engine. Two reads could disagree, and a
// difference between engines would then be indistinguishable from a difference
// between reads.
func TestEnginesShareOneRead(t *testing.T) {
	inputs, _ := syntheticHistory(8, 80, 1.5, 11)
	src := &fakeSource{inputs: inputs}

	engine := wengLin(t)
	rep := run(t, src, DefaultSplit, engine, staticEngine{}, invertedEngine{inner: engine})

	if src.listCalls != 1 {
		t.Errorf("ListInputs called %d times for one target, want 1", src.listCalls)
	}
	if len(rep.Modes[0].Engines) != 3 {
		t.Fatalf("got %d engine scores, want 3", len(rep.Modes[0].Engines))
	}
	for i, want := range []string{engine.ID(), staticEngine{}.ID(), invertedEngine{}.ID()} {
		if got := rep.Modes[0].Engines[i].EngineID; got != want {
			t.Errorf("engine %d in report is %q, want %q (supplied order)", i, got, want)
		}
	}
}

// TestReportCountsHeldOutMatches is the "a thin mode reads as thin" criterion:
// the score never travels without the number of matches behind it.
func TestReportCountsHeldOutMatches(t *testing.T) {
	inputs, _ := syntheticHistory(4, 10, 1.5, 5)
	rep := run(t, &fakeSource{inputs: inputs}, Split{TrainFraction: 0.8}, wengLin(t))

	mode := rep.Modes[0]
	if mode.Matches != 10 || mode.TrainMatches != 8 || mode.HeldOutMatches != 2 {
		t.Fatalf("split of 10 matches at 0.8 = %d train / %d held out, want 8 / 2", mode.TrainMatches, mode.HeldOutMatches)
	}
	if got := mode.Engines[0].Score.HeldOut; got != 2 {
		t.Errorf("scored %d held-out matches, want 2", got)
	}
	if rep.Split != (Split{TrainFraction: 0.8}) {
		t.Errorf("report did not carry the split it was run with: %+v", rep.Split)
	}
}

// TestSplitIsConfigurable covers both ways of naming the split point and the
// edges where one side of it empties out.
func TestSplitIsConfigurable(t *testing.T) {
	inputs, _ := syntheticHistory(4, 10, 1.5, 5)

	for _, tc := range []struct {
		name              string
		split             Split
		wantTrain, wantHO int
	}{
		{"half by fraction", Split{TrainFraction: 0.5}, 5, 5},
		{"truncates toward zero", Split{TrainFraction: 0.75}, 7, 3},
		{"exact count", Split{TrainMatches: 3}, 3, 7},
		{"count wins over fraction", Split{TrainMatches: 2, TrainFraction: 0.9}, 2, 8},
		{"count beyond history trains on all and scores none", Split{TrainMatches: 50}, 10, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rep := run(t, &fakeSource{inputs: inputs}, tc.split, wengLin(t))
			mode := rep.Modes[0]
			if mode.TrainMatches != tc.wantTrain || mode.HeldOutMatches != tc.wantHO {
				t.Fatalf("got %d train / %d held out, want %d / %d", mode.TrainMatches, mode.HeldOutMatches, tc.wantTrain, tc.wantHO)
			}
			if got := mode.Engines[0].Score.HeldOut; got != tc.wantHO {
				t.Errorf("scored %d matches, want %d", got, tc.wantHO)
			}
		})
	}
}

// TestInvalidSplitIsRejected keeps a nonsense split from silently becoming
// "train on everything, score nothing" — which reports a clean zero.
func TestInvalidSplitIsRejected(t *testing.T) {
	for _, split := range []Split{{}, {TrainFraction: 0}, {TrainFraction: 1}, {TrainFraction: 1.5}, {TrainFraction: -0.5}, {TrainMatches: -1}} {
		if _, err := New(&fakeSource{}, split); err == nil {
			t.Errorf("New accepted invalid split %+v", split)
		}
	}
}

// TestRunRequiresAnEngine guards the degenerate call that would otherwise
// produce a report full of history counts and no scores.
func TestRunRequiresAnEngine(t *testing.T) {
	h, err := New(&fakeSource{}, DefaultSplit)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := h.Run(context.Background(), []Target{{GameID: "g", ModeKey: "duel"}}); err == nil {
		t.Error("Run accepted zero engines")
	}
}

// TestReportCarriesModifierIdentifiability is the criterion that a comparison
// and the reason to distrust it arrive together. Every player here is locked
// to one seat for the whole history, so seat and skill are confounded and the
// report has to say so beside the scores rather than elsewhere.
func TestReportCarriesModifierIdentifiability(t *testing.T) {
	inputs := []rating.Input{
		{SessionID: "c1", Sides: []rating.Side{
			{Rank: 0, Entrants: []rating.Entrant{{Key: "player:alice"}, {Key: "seat:White"}}},
			{Rank: 1, Entrants: []rating.Entrant{{Key: "player:bob"}, {Key: "seat:Black"}}},
		}},
		{SessionID: "c2", Sides: []rating.Side{
			{Rank: 0, Entrants: []rating.Entrant{{Key: "player:alice"}, {Key: "seat:White"}}},
			{Rank: 1, Entrants: []rating.Entrant{{Key: "player:bob"}, {Key: "seat:Black"}}},
		}},
	}

	rep := run(t, &fakeSource{inputs: inputs}, Split{TrainMatches: 1}, wengLin(t))

	mods := rep.Modes[0].Modifiers
	white, ok := mods["seat:White"]
	if !ok {
		t.Fatalf("report has no entry for seat:White, got %+v", mods)
	}
	if white.PlayersWithMultipleValues != 0 {
		t.Errorf("seat:White PlayersWithMultipleValues = %d, want 0 (confounded with skill)", white.PlayersWithMultipleValues)
	}
	if white.Matches != 2 || white.DistinctValues != 2 {
		t.Errorf("seat:White = %+v, want 2 matches across 2 distinct seat values", white)
	}
	// Identifiability is over the whole history, not just the held-out part:
	// the training prefix is what the ratings being scored came from, so a
	// modifier confounded there taints the score either way.
	if mods["seat:Black"].Matches != 2 {
		t.Errorf("seat:Black seen in %d matches, want 2 (whole history, not just held out)", mods["seat:Black"].Matches)
	}
}

// TestMultiSideOrderingIsScoredPairwise proves a three-way result is scored on
// the whole finishing order rather than on the winner alone.
func TestMultiSideOrderingIsScoredPairwise(t *testing.T) {
	// One held-out match, three sides, all ranks distinct: three pairs.
	inputs := []rating.Input{
		{SessionID: "t1", Sides: []rating.Side{
			{Rank: 0, Entrants: []rating.Entrant{{Key: "player:a"}}},
			{Rank: 1, Entrants: []rating.Entrant{{Key: "player:b"}}},
			{Rank: 2, Entrants: []rating.Entrant{{Key: "player:c"}}},
		}},
	}
	rep := run(t, &fakeSource{inputs: inputs}, Split{TrainMatches: 0, TrainFraction: 0.5}, staticEngine{})

	got := rep.Modes[0].Engines[0].Score
	if got.Pairs != 3 {
		t.Errorf("scored %d pairs for a 3-side match, want 3", got.Pairs)
	}
	// Everyone at the prior, so every pair is a tie: exactly chance.
	if got.Accuracy != 0.5 {
		t.Errorf("accuracy = %.4f over indistinguishable sides, want 0.5", got.Accuracy)
	}
}

// TestTiedMatchesAreExcludedFromProbabilisticScores proves a shared first
// place is counted out rather than folded in under an invented convention,
// while still being scored on ordering wherever an ordering exists.
func TestTiedMatchesAreExcludedFromProbabilisticScores(t *testing.T) {
	tie := rating.Input{SessionID: "tie", Sides: []rating.Side{
		{Rank: 0, Entrants: []rating.Entrant{{Key: "player:a"}}},
		{Rank: 0, Entrants: []rating.Entrant{{Key: "player:b"}}},
	}}
	decisive := rating.Input{SessionID: "win", Sides: []rating.Side{
		{Rank: 0, Entrants: []rating.Entrant{{Key: "player:a"}}},
		{Rank: 1, Entrants: []rating.Entrant{{Key: "player:b"}}},
	}}

	rep := run(t, &fakeSource{inputs: []rating.Input{decisive, tie, decisive}}, Split{TrainMatches: 1}, wengLin(t))

	got := rep.Modes[0].Engines[0].Score
	if got.HeldOut != 2 {
		t.Fatalf("held out %d matches, want 2", got.HeldOut)
	}
	if got.ProbMatches != 1 {
		t.Errorf("probabilistic score computed over %d matches, want 1 (the tie has no single winner)", got.ProbMatches)
	}
	if got.Pairs != 1 {
		t.Errorf("ordering scored over %d pairs, want 1 (the tied pair has no order to get right)", got.Pairs)
	}
}
