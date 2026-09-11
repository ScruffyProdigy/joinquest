package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/scruffyprodigy/joinquest/internal/rating"
)

// defaultRatingEngineID is the engine identity a mode with no measured
// constants is rated by — what server.go passes to the replay sweep.
func defaultRatingEngineID(t *testing.T) string {
	t.Helper()
	engine, err := rating.NewWengLin("plackett-luce")
	if err != nil {
		t.Fatalf("NewWengLin: %v", err)
	}
	return engine.ID()
}

// measuredEngine builds an engine at a beta well away from the default, so the
// id it carries is unmistakably a different set of constants.
func measuredEngine(t *testing.T, beta float64) rating.Engine {
	t.Helper()
	engine, err := rating.NewWengLinBeta("plackett-luce", beta)
	if err != nil {
		t.Fatalf("NewWengLinBeta: %v", err)
	}
	return engine
}

// TestModeRatingConstantsRoundTrip covers the read side's central distinction:
// absence means unmeasured, and it is not the same as a stored default.
func TestModeRatingConstantsRoundTrip(t *testing.T) {
	st, _, gameID, _ := newCompetitiveSessionFixture(t, 2)
	ctx := context.Background()

	if _, measured, err := st.GetModeRatingConstants(ctx, gameID, "duel"); err != nil {
		t.Fatalf("GetModeRatingConstants: %v", err)
	} else if measured {
		t.Fatal("a mode nobody has fitted reported measured constants")
	}

	engine := measuredEngine(t, rating.DefaultBeta*4)
	at := time.Now().UTC().Truncate(time.Millisecond)
	want := ModeRatingConstants{
		Beta:        rating.DefaultBeta * 4,
		EngineID:    engine.ID(),
		SampleSize:  1234,
		EstimatedAt: at,
	}
	if err := st.SetModeRatingConstants(ctx, gameID, "duel", want); err != nil {
		t.Fatalf("SetModeRatingConstants: %v", err)
	}

	got, measured, err := st.GetModeRatingConstants(ctx, gameID, "duel")
	if err != nil {
		t.Fatalf("GetModeRatingConstants: %v", err)
	}
	if !measured {
		t.Fatal("stored constants came back unmeasured")
	}
	if got.Beta != want.Beta || got.EngineID != want.EngineID || got.SampleSize != want.SampleSize {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if !got.EstimatedAt.Equal(at) {
		t.Errorf("EstimatedAt = %v, want %v", got.EstimatedAt, at)
	}

	// A second measurement replaces the first rather than accumulating.
	second := measuredEngine(t, rating.DefaultBeta/4)
	if err := st.SetModeRatingConstants(ctx, gameID, "duel", ModeRatingConstants{
		Beta: rating.DefaultBeta / 4, EngineID: second.ID(), SampleSize: 99, EstimatedAt: at,
	}); err != nil {
		t.Fatalf("second SetModeRatingConstants: %v", err)
	}
	got, _, err = st.GetModeRatingConstants(ctx, gameID, "duel")
	if err != nil {
		t.Fatalf("GetModeRatingConstants: %v", err)
	}
	if got.EngineID != second.ID() || got.SampleSize != 99 {
		t.Errorf("re-measuring left %+v, want the newer fit", got)
	}

	if err := st.ClearModeRatingConstants(ctx, gameID, "duel"); err != nil {
		t.Fatalf("ClearModeRatingConstants: %v", err)
	}
	if _, measured, err := st.GetModeRatingConstants(ctx, gameID, "duel"); err != nil {
		t.Fatalf("GetModeRatingConstants after clear: %v", err)
	} else if measured {
		t.Error("cleared constants still read as measured; clearing must return the mode to unmeasured, not to a stored default")
	}
}

// TestSetModeRatingConstantsRejectsAnEstimateWithNoEvidence keeps the sample
// size from being optional. A stored beta with no sample behind it cannot be
// told apart later from one fitted on forty thousand matches.
func TestSetModeRatingConstantsRejectsAnEstimateWithNoEvidence(t *testing.T) {
	st, _, gameID, _ := newCompetitiveSessionFixture(t, 2)
	ctx := context.Background()
	engine := measuredEngine(t, rating.DefaultBeta*2)

	for name, c := range map[string]ModeRatingConstants{
		"no sample size":  {Beta: 8, EngineID: engine.ID(), SampleSize: 0},
		"negative sample": {Beta: 8, EngineID: engine.ID(), SampleSize: -1},
		"no engine id":    {Beta: 8, SampleSize: 10},
		"zero beta":       {Beta: 0, EngineID: engine.ID(), SampleSize: 10},
		"negative beta":   {Beta: -1, EngineID: engine.ID(), SampleSize: 10},
	} {
		if err := st.SetModeRatingConstants(ctx, gameID, "duel", c); err == nil {
			t.Errorf("%s: got nil error, want a rejection", name)
		}
	}
	if err := st.SetModeRatingConstants(ctx, gameID, "", ModeRatingConstants{
		Beta: 8, EngineID: engine.ID(), SampleSize: 10,
	}); err == nil {
		t.Error("empty mode key: got nil error, want a rejection")
	}
}

// TestChangingBetaMakesTheModeNeedReplay is the acceptance criterion that ties
// the constants table to the machinery that acts on it. Ratings computed under
// one beta are not comparable with ratings computed under another, so a mode
// whose constants moved has to come back as stale — through the same sweep
// that catches a mode with unapplied inputs, and for the same repair.
func TestChangingBetaMakesTheModeNeedReplay(t *testing.T) {
	st, sessionID, gameID, users := newCompetitiveSessionFixture(t, 2)
	ctx := context.Background()

	rated, err := st.RecordMatchResult(ctx, sessionID, "COMPLETED", users[:1], nil, time.Now())
	if err != nil {
		t.Fatalf("RecordMatchResult: %v", err)
	}
	modeKey := rated.ModeKey
	defaultID := defaultRatingEngineID(t)

	// Replay under the default, so the mode starts caught up.
	engine, err := rating.NewWengLin("plackett-luce")
	if err != nil {
		t.Fatalf("NewWengLin: %v", err)
	}
	if _, err := rating.NewReplayer(engine, st.RatingSource()).ReplayMode(ctx, gameID.String(), modeKey); err != nil {
		t.Fatalf("ReplayMode: %v", err)
	}
	if needsReplay(t, st, ctx, defaultID, gameID, modeKey) {
		t.Fatal("a freshly replayed mode already needs replay; this test cannot show anything")
	}

	// Measuring a beta makes it stale, with no new match and no new input.
	measured := measuredEngine(t, rating.DefaultBeta*4)
	if err := st.SetModeRatingConstants(ctx, gameID, modeKey, ModeRatingConstants{
		Beta: rating.DefaultBeta * 4, EngineID: measured.ID(), SampleSize: 500, EstimatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SetModeRatingConstants: %v", err)
	}
	if !needsReplay(t, st, ctx, defaultID, gameID, modeKey) {
		t.Fatal("a mode whose beta changed was not reported as needing replay: its ratings are still on the old scale")
	}

	// Replaying under the new constants settles it again.
	if _, err := rating.NewReplayer(measured, st.RatingSource()).ReplayMode(ctx, gameID.String(), modeKey); err != nil {
		t.Fatalf("ReplayMode under measured beta: %v", err)
	}
	if needsReplay(t, st, ctx, defaultID, gameID, modeKey) {
		t.Fatal("a mode replayed under its own constants still reads as stale")
	}

	// And clearing the measurement makes it stale again, because the mode is
	// back on the default engine and its ratings are not.
	if err := st.ClearModeRatingConstants(ctx, gameID, modeKey); err != nil {
		t.Fatalf("ClearModeRatingConstants: %v", err)
	}
	if !needsReplay(t, st, ctx, defaultID, gameID, modeKey) {
		t.Error("a mode returned to the default constants was not reported as needing replay")
	}
}

// TestReplayUnderTheWrongEngineStaysStale is the safety net under the whole
// scheme: the sweep keys off the engine id the ratings were actually computed
// with, so replaying a measured mode under the default cannot quiet it.
func TestReplayUnderTheWrongEngineStaysStale(t *testing.T) {
	st, sessionID, gameID, users := newCompetitiveSessionFixture(t, 2)
	ctx := context.Background()

	rated, err := st.RecordMatchResult(ctx, sessionID, "COMPLETED", users[:1], nil, time.Now())
	if err != nil {
		t.Fatalf("RecordMatchResult: %v", err)
	}
	modeKey := rated.ModeKey
	defaultID := defaultRatingEngineID(t)

	measured := measuredEngine(t, rating.DefaultBeta*4)
	if err := st.SetModeRatingConstants(ctx, gameID, modeKey, ModeRatingConstants{
		Beta: rating.DefaultBeta * 4, EngineID: measured.ID(), SampleSize: 500, EstimatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SetModeRatingConstants: %v", err)
	}

	wrong, err := rating.NewWengLin("plackett-luce")
	if err != nil {
		t.Fatalf("NewWengLin: %v", err)
	}
	if _, err := rating.NewReplayer(wrong, st.RatingSource()).ReplayMode(ctx, gameID.String(), modeKey); err != nil {
		t.Fatalf("ReplayMode: %v", err)
	}

	if !needsReplay(t, st, ctx, defaultID, gameID, modeKey) {
		t.Error("a measured mode replayed under the default engine reads as caught up; wrong-scale ratings would sit there unnoticed")
	}
}

// TestListModeRatingConstantsReportsEveryMeasuredMode covers the listing an
// operator reads.
func TestListModeRatingConstantsReportsEveryMeasuredMode(t *testing.T) {
	st, _, gameID, _ := newCompetitiveSessionFixture(t, 2)
	ctx := context.Background()

	engine := measuredEngine(t, rating.DefaultBeta*2)
	for _, mode := range []string{"duel", "party"} {
		if err := st.SetModeRatingConstants(ctx, gameID, mode, ModeRatingConstants{
			Beta: rating.DefaultBeta * 2, EngineID: engine.ID(), SampleSize: 250, EstimatedAt: time.Now().UTC(),
		}); err != nil {
			t.Fatalf("SetModeRatingConstants(%s): %v", mode, err)
		}
	}

	rows, err := st.ListModeRatingConstants(ctx)
	if err != nil {
		t.Fatalf("ListModeRatingConstants: %v", err)
	}

	seen := map[string]int{}
	for _, r := range rows {
		if r.GameID == gameID {
			seen[r.ModeKey] = r.SampleSize
		}
	}
	if seen["duel"] != 250 || seen["party"] != 250 {
		t.Errorf("listed %v, want both modes with their sample sizes", seen)
	}
}

func needsReplay(t *testing.T, st *Store, ctx context.Context, defaultEngineID string, gameID uuid.UUID, modeKey string) bool {
	t.Helper()
	modes, err := st.ListModesNeedingReplay(ctx, defaultEngineID)
	if err != nil {
		t.Fatalf("ListModesNeedingReplay: %v", err)
	}
	for _, m := range modes {
		if m.GameID == gameID && m.ModeKey == modeKey {
			return true
		}
	}
	return false
}
