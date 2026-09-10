package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/scruffyprodigy/joinquest/internal/rating"
	"github.com/scruffyprodigy/joinquest/internal/ratingbacktest"
)

func TestBuildEnginesPreservesOrder(t *testing.T) {
	engines, err := buildEngines("plackett-luce")
	if err != nil {
		t.Fatalf("buildEngines: %v", err)
	}
	if len(engines) != 1 {
		t.Fatalf("got %d engines, want 1", len(engines))
	}
	if got := engines[0].ID(); got != "weng-lin/plackett-luce@1" {
		t.Errorf("engine id = %q", got)
	}
}

func TestBuildEnginesRejectsBadSpecs(t *testing.T) {
	for _, spec := range []string{
		"",
		"  ",
		",",
		"elo",
		// Thurstone-Mosteller has no implementation yet (JQ-231). Naming it
		// must fail rather than silently score Plackett-Luce under its name.
		"thurstone-mosteller",
		// Two identical columns read as two models agreeing.
		"plackett-luce,plackett-luce",
	} {
		if _, err := buildEngines(spec); err == nil {
			t.Errorf("buildEngines(%q) accepted an invalid spec", spec)
		}
	}
}

func TestBuildEnginesTolerantOfSpacing(t *testing.T) {
	engines, err := buildEngines(" plackett-luce , ")
	if err != nil {
		t.Fatalf("buildEngines: %v", err)
	}
	if len(engines) != 1 {
		t.Fatalf("got %d engines, want 1", len(engines))
	}
}

// TestPrintReportShowsScoreAndCaveatTogether pins the two things a reader must
// not be able to see separately: the number of matches behind a score, and
// whether a modifier in that history is confounded with player skill. A report
// that prints a clean accuracy over four matches, or one whose seat advantage
// is inseparable from the players who always sat there, is a report that
// misleads precisely because it looks tidy.
func TestPrintReportShowsScoreAndCaveatTogether(t *testing.T) {
	var buf bytes.Buffer
	printReport(&buf, ratingbacktest.Report{
		Split: ratingbacktest.Split{TrainFraction: 0.8},
		Modes: []ratingbacktest.ModeReport{{
			Target:         ratingbacktest.Target{GameID: "g1", ModeKey: "ranked"},
			Matches:        20,
			TrainMatches:   16,
			HeldOutMatches: 4,
			Modifiers: map[string]rating.ModifierReport{
				"seat:White": {Matches: 20, DistinctValues: 2, PlayersWithMultipleValues: 0},
				"seat:Black": {Matches: 20, DistinctValues: 2, PlayersWithMultipleValues: 7},
			},
			Engines: []ratingbacktest.EngineScore{{
				EngineID: "weng-lin/plackett-luce@1",
				Score: ratingbacktest.Score{
					HeldOut: 4, Pairs: 4, Accuracy: 0.75,
					Probabilistic: true, ProbMatches: 4, LogLoss: 0.5, Brier: 0.33,
				},
			}},
		}},
	})

	out := buf.String()
	for _, want := range []string{
		"first 80% of matches",
		"20 matches (16 train, 4 held out)",
		"weng-lin/plackett-luce@1",
		"0.7500",
		"seat:White",
		"confounded with player skill",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report is missing %q:\n%s", want, out)
		}
	}
	// The identifiable modifier must not carry the warning.
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "seat:Black") && strings.Contains(line, "confounded") {
			t.Errorf("identifiable modifier was flagged as confounded: %q", line)
		}
	}
}

// TestPrintReportSaysNothingWasScored covers the mode too thin to split: it
// must say so rather than print an accuracy of zero.
func TestPrintReportSaysNothingWasScored(t *testing.T) {
	var buf bytes.Buffer
	printReport(&buf, ratingbacktest.Report{
		Split: ratingbacktest.Split{TrainMatches: 100},
		Modes: []ratingbacktest.ModeReport{{
			Target:  ratingbacktest.Target{GameID: "g1", ModeKey: "casual"},
			Matches: 3, TrainMatches: 3, HeldOutMatches: 0,
		}},
	})

	out := buf.String()
	if !strings.Contains(out, "nothing scored") {
		t.Errorf("report did not say nothing was scored:\n%s", out)
	}
	if strings.Contains(out, "0.0000") {
		t.Errorf("report printed a zero score for a mode with no held-out matches:\n%s", out)
	}
}

// TestPrintReportOmitsProbabilisticColumnsForNonPredictors keeps an engine that
// cannot forecast from reading as one that forecast perfectly.
func TestPrintReportOmitsProbabilisticColumnsForNonPredictors(t *testing.T) {
	var buf bytes.Buffer
	printReport(&buf, ratingbacktest.Report{
		Split: ratingbacktest.Split{TrainFraction: 0.5},
		Modes: []ratingbacktest.ModeReport{{
			Target:  ratingbacktest.Target{GameID: "g1", ModeKey: "ranked"},
			Matches: 10, TrainMatches: 5, HeldOutMatches: 5,
			Engines: []ratingbacktest.EngineScore{{
				EngineID: "some/engine@1",
				Score:    ratingbacktest.Score{HeldOut: 5, Pairs: 5, Accuracy: 0.6},
			}},
		}},
	})

	out := buf.String()
	if strings.Contains(out, "0.0000") {
		t.Errorf("a non-predicting engine printed a zero log-loss/Brier:\n%s", out)
	}
	if !strings.Contains(out, "-") {
		t.Errorf("expected absent probabilistic metrics to print as \"-\":\n%s", out)
	}
}
