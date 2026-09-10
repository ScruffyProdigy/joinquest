package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/scruffyprodigy/joinquest/internal/coldstart"
)

// A dry run has to say so. The command exists to be pointed at production, and
// a reader who cannot tell from the output whether it just wrote something has
// to go and check.
func TestPrintFitsSaysWhetherItWrote(t *testing.T) {
	var dry, wrote bytes.Buffer
	printFits(&dry, nil, false)
	printFits(&wrote, nil, true)

	if !strings.Contains(dry.String(), "dry run") {
		t.Errorf("dry run output does not say so:\n%s", dry.String())
	}
	if strings.Contains(wrote.String(), "dry run") {
		t.Errorf("a run that wrote is described as a dry run:\n%s", wrote.String())
	}
}

// The usable column is the one a reader acts on, so a pair that fails the
// evidence gate must read as unusable next to its own numbers rather than
// leaving them to be interpreted.
func TestPrintFitsMarksUnusablePairs(t *testing.T) {
	thin := coldstart.PairStats{
		SourceMode: "arena", TargetMode: "duel",
		PairedPlayers:  coldstart.MinPairedPlayers - 1,
		Correlation:    0.97,
		SourceSD:       5,
		TargetSD:       5,
		ResidualSD:     2,
		FlatResidualSD: 8,
	}
	strong := thin
	strong.PairedPlayers = coldstart.MinPairedPlayers * 3
	strong.SourceMode = "blitz"

	var out bytes.Buffer
	printFits(&out, []gameFit{{GameID: uuid.New(), Pairs: []coldstart.PairStats{thin, strong}}}, false)

	lines := strings.Split(out.String(), "\n")
	var thinLine, strongLine string
	for _, line := range lines {
		if strings.Contains(line, "arena") {
			thinLine = line
		}
		if strings.Contains(line, "blitz") {
			strongLine = line
		}
	}

	if !strings.HasSuffix(strings.TrimSpace(thinLine), "no") {
		t.Errorf("a pair below MinPairedPlayers is not marked unusable: %q", thinLine)
	}
	if !strings.HasSuffix(strings.TrimSpace(strongLine), "yes") {
		t.Errorf("a well-measured pair is not marked usable: %q", strongLine)
	}
}

// A game whose modes share no players is a different situation from one that
// was measured and came out weak, and the output has to distinguish them.
func TestPrintFitsExplainsAGameWithNoPairs(t *testing.T) {
	var out bytes.Buffer
	printFits(&out, []gameFit{{GameID: uuid.New()}}, false)

	if !strings.Contains(out.String(), "no two modes share a player") {
		t.Errorf("a game with no measurable pairs is not explained:\n%s", out.String())
	}
}
