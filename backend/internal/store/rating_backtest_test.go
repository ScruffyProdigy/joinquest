package store

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/scruffyprodigy/joinquest/internal/rating"
	"github.com/scruffyprodigy/joinquest/internal/ratingbacktest"
)

// TestListRatedModesReportsModesAlreadyReplayed is the difference between this
// query and ListModesNeedingReplay, which is the reason it exists. A mode
// whose ratings are fully caught up is invisible to the replay sweep and is
// still a perfectly good backtest target: the harness reads the input log, not
// the cache. Enumerating from the sweep's query instead would silently skip
// every healthy mode — that is, all the ones with history worth scoring.
func TestListRatedModesReportsModesAlreadyReplayed(t *testing.T) {
	st, sessionID, gameID, users := newCompetitiveSessionFixture(t, 2)
	ctx := context.Background()

	rated, err := st.RecordMatchResult(ctx, sessionID, "COMPLETED", users[:1], nil, time.Now())
	if err != nil {
		t.Fatalf("RecordMatchResult: %v", err)
	}

	engine, err := rating.NewWengLin("plackett-luce")
	if err != nil {
		t.Fatalf("NewWengLin: %v", err)
	}
	if _, err := rating.NewReplayer(engine, st.RatingSource()).ReplayMode(ctx, gameID.String(), rated.ModeKey); err != nil {
		t.Fatalf("ReplayMode: %v", err)
	}

	// Caught up, so the replay sweep has nothing to say about it.
	needing, err := st.ListModesNeedingReplay(ctx)
	if err != nil {
		t.Fatalf("ListModesNeedingReplay: %v", err)
	}
	for _, m := range needing {
		if m.GameID == gameID && m.ModeKey == rated.ModeKey {
			t.Fatal("mode still needs replay; this test cannot distinguish the two queries")
		}
	}

	modes, err := st.ListRatedModes(ctx)
	if err != nil {
		t.Fatalf("ListRatedModes: %v", err)
	}
	for _, m := range modes {
		if m.GameID == gameID && m.ModeKey == rated.ModeKey {
			return
		}
	}
	t.Fatalf("a mode with retained history was not listed as backtestable: %+v", modes)
}

// TestBacktestOverRealHistoryLeavesRatingsUntouched is the dry-run guarantee
// checked end to end, against Postgres rather than a fake. The harness holds a
// read-only view of a store that is entirely capable of writing, so the
// compile-time narrowing in ratingbacktest.Source is the mechanism and this is
// the proof that the mechanism is wired to the real thing.
func TestBacktestOverRealHistoryLeavesRatingsUntouched(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	gameID, modeKey, _, _ := seedThreeFinishedMatches(t, st, ctx, cleaner)

	engine, err := rating.NewWengLin("plackett-luce")
	if err != nil {
		t.Fatalf("NewWengLin: %v", err)
	}
	if _, err := rating.NewReplayer(engine, st.RatingSource()).ReplayMode(ctx, gameID.String(), modeKey); err != nil {
		t.Fatalf("ReplayMode: %v", err)
	}

	before, err := st.LoadPlayerRatings(ctx, gameID, modeKey)
	if err != nil {
		t.Fatalf("LoadPlayerRatings: %v", err)
	}
	entitiesBefore, err := st.LoadNonPlayerRatings(ctx, gameID, modeKey)
	if err != nil {
		t.Fatalf("LoadNonPlayerRatings: %v", err)
	}
	if len(before) == 0 {
		t.Fatal("no cached ratings to protect; the test would pass vacuously")
	}

	h, err := ratingbacktest.New(st.RatingSource(), ratingbacktest.Split{TrainMatches: 2})
	if err != nil {
		t.Fatalf("backtest.New: %v", err)
	}
	report, err := h.Run(ctx, []ratingbacktest.Target{{GameID: gameID.String(), ModeKey: modeKey}}, engine)
	if err != nil {
		t.Fatalf("backtest Run: %v", err)
	}
	if len(report.Modes) != 1 || report.Modes[0].Matches != 3 {
		t.Fatalf("backtest read %+v, want one mode with 3 matches", report.Modes)
	}
	if report.Modes[0].HeldOutMatches != 1 {
		t.Errorf("held out %d matches, want 1", report.Modes[0].HeldOutMatches)
	}

	after, err := st.LoadPlayerRatings(ctx, gameID, modeKey)
	if err != nil {
		t.Fatalf("LoadPlayerRatings after: %v", err)
	}
	entitiesAfter, err := st.LoadNonPlayerRatings(ctx, gameID, modeKey)
	if err != nil {
		t.Fatalf("LoadNonPlayerRatings after: %v", err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Errorf("backtest mutated player_ratings:\nbefore = %+v\nafter  = %+v", before, after)
	}
	if !reflect.DeepEqual(entitiesBefore, entitiesAfter) {
		t.Errorf("backtest mutated nonplayer_ratings:\nbefore = %+v\nafter  = %+v", entitiesBefore, entitiesAfter)
	}
}
