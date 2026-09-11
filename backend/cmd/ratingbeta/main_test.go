package main

import (
	"strings"
	"testing"

	"github.com/scruffyprodigy/joinquest/internal/rating"
	"github.com/scruffyprodigy/joinquest/internal/ratingbacktest"
)

func TestParseMultipliers(t *testing.T) {
	if got, err := parseMultipliers(""); err != nil || got != nil {
		t.Errorf("empty spec = %v, %v; want the standard grid left in place", got, err)
	}

	got, err := parseMultipliers(" 0.5, 1 ,2 ")
	if err != nil {
		t.Fatalf("parseMultipliers: %v", err)
	}
	want := []float64{0.5, 1, 2}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}

	for _, spec := range []string{"nope", "0", "-1", ","} {
		if _, err := parseMultipliers(spec); err == nil {
			t.Errorf("parseMultipliers(%q) = nil error, want a rejection", spec)
		}
	}
}

// TestPrintEstimatesShowsSampleSizeEvenWhenUnmeasured is the acceptance
// criterion at the surface a human actually reads. A mode that was not fitted
// still has to say how much history it had, or "unmeasured" reads as "unknown"
// and nobody can tell a mode that is nearly ready from one with four matches.
func TestPrintEstimatesShowsSampleSizeEvenWhenUnmeasured(t *testing.T) {
	var out strings.Builder
	printEstimates(&out, []ratingbacktest.BetaEstimate{{
		Target:         ratingbacktest.Target{GameID: "g", ModeKey: "party"},
		Beta:           rating.DefaultBeta,
		Default:        rating.DefaultBeta,
		Matches:        60,
		HeldOut:        12,
		ProbMatches:    12,
		MinProbMatches: ratingbacktest.MinProbMatches,
		Unmeasured:     "only 12 scored held-out match(es), below the floor of 200",
		Candidates: []ratingbacktest.BetaCandidate{
			{Beta: rating.DefaultBeta, Score: ratingbacktest.Score{LogLoss: 0.69}},
		},
	}}, rating.DefaultBeta)

	text := out.String()
	for _, want := range []string{"60 matches", "12 held out, 12 scored", "unmeasured", "keeping the default"} {
		if !strings.Contains(text, want) {
			t.Errorf("report is missing %q:\n%s", want, text)
		}
	}
}

// TestPrintEstimatesFlagsAGridEdgeWinner keeps a bound from being read as a
// fit.
func TestPrintEstimatesFlagsAGridEdgeWinner(t *testing.T) {
	var out strings.Builder
	printEstimates(&out, []ratingbacktest.BetaEstimate{{
		Target:         ratingbacktest.Target{GameID: "g", ModeKey: "party"},
		Beta:           rating.DefaultBeta * 8,
		Default:        rating.DefaultBeta,
		Measured:       true,
		AtGridEdge:     true,
		Matches:        5000,
		HeldOut:        1000,
		ProbMatches:    1000,
		MinProbMatches: ratingbacktest.MinProbMatches,
		LogLoss:        0.61,
		DefaultLogLoss: 0.68,
		Candidates: []ratingbacktest.BetaCandidate{
			{Beta: rating.DefaultBeta, Score: ratingbacktest.Score{LogLoss: 0.68}},
			{Beta: rating.DefaultBeta * 8, Score: ratingbacktest.Score{LogLoss: 0.61}},
		},
	}}, rating.DefaultBeta)

	text := out.String()
	if !strings.Contains(text, "edge of the grid") {
		t.Errorf("a winner at the grid edge was not flagged:\n%s", text)
	}
	if !strings.Contains(text, "fitted on 1000 match(es)") {
		t.Errorf("the sample size is missing from the measured line:\n%s", text)
	}
	if !strings.Contains(text, "vs 0.6800 at the default") {
		t.Errorf("the default's score is missing, so the reader cannot see what fitting bought:\n%s", text)
	}
}

// TestPrintAppliedSaysWhatHappensNext: the write on its own changes nothing a
// player can see until the sweep replays the mode, and an operator who does
// not know that will assume it failed.
func TestPrintAppliedSaysWhatHappensNext(t *testing.T) {
	var out strings.Builder
	printApplied(&out, true, []string{"g/duel"}, nil)
	if text := out.String(); !strings.Contains(text, "replayed in full") {
		t.Errorf("no mention of the replay that has to follow:\n%s", text)
	}

	var dry strings.Builder
	printApplied(&dry, false, nil, []ratingbacktest.BetaEstimate{{Measured: true}, {Measured: false}})
	if text := dry.String(); !strings.Contains(text, "1 of 2") || !strings.Contains(text, "-apply") {
		t.Errorf("dry run did not say what would be written:\n%s", text)
	}
}
