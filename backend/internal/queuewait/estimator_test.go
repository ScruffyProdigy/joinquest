package queuewait

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// fakeSamples serves fixed fills per queue and records the query it was asked,
// so a test can assert on the window an estimator requested, and on how many
// round trips it took to answer for several queues.
type fakeSamples struct {
	byQueue map[uuid.UUID][]Fill
	err     error
	got     FillQuery
	calls   int
}

func (f *fakeSamples) RecentFills(_ context.Context, q FillQuery) (map[uuid.UUID][]Fill, error) {
	f.got = q
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.byQueue, nil
}

// fillsOf builds fills with the given waits, in seconds. The join times are all
// the same instant: the median strategy cares only about the durations.
func fillsOf(seconds ...int) []Fill {
	base := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	fills := make([]Fill, 0, len(seconds))
	for _, s := range seconds {
		fills = append(fills, Fill{
			JoinedAt:  base,
			MatchedAt: base.Add(time.Duration(s) * time.Second),
		})
	}
	return fills
}

func TestMedianEstimatorReturnsMiddleWaitForOddSampleCount(t *testing.T) {
	queueID := uuid.New()
	samples := &fakeSamples{byQueue: map[uuid.UUID][]Fill{queueID: fillsOf(60, 10, 30, 15, 20)}}
	est := MedianEstimator{Samples: samples, MinSamples: 5}

	got, err := est.EstimateByModeQueue(context.Background(), []uuid.UUID{queueID}, time.Now())
	if err != nil {
		t.Fatalf("EstimateByModeQueue: %v", err)
	}
	if got[queueID] != 20*time.Second {
		t.Errorf("got %v, want 20s", got[queueID])
	}
}

func TestMedianEstimatorAveragesTheTwoMiddleWaitsForEvenSampleCount(t *testing.T) {
	queueID := uuid.New()
	samples := &fakeSamples{byQueue: map[uuid.UUID][]Fill{queueID: fillsOf(40, 10, 30, 20)}}
	est := MedianEstimator{Samples: samples, MinSamples: 4}

	got, err := est.EstimateByModeQueue(context.Background(), []uuid.UUID{queueID}, time.Now())
	if err != nil {
		t.Fatalf("EstimateByModeQueue: %v", err)
	}
	if got[queueID] != 25*time.Second {
		t.Errorf("got %v, want 25s", got[queueID])
	}
}

func TestMedianEstimatorEstimatesEveryQueueInOneRoundTrip(t *testing.T) {
	quick, slow, cold := uuid.New(), uuid.New(), uuid.New()
	samples := &fakeSamples{byQueue: map[uuid.UUID][]Fill{
		quick: fillsOf(5, 10, 15, 20, 25),
		slow:  fillsOf(100, 200, 300, 400, 500),
		cold:  fillsOf(9, 9), // below the floor
	}}
	est := MedianEstimator{Samples: samples, MinSamples: 5}

	got, err := est.EstimateByModeQueue(context.Background(), []uuid.UUID{quick, slow, cold}, time.Now())
	if err != nil {
		t.Fatalf("EstimateByModeQueue: %v", err)
	}

	// The whole point of the batch shape: one fetch, however many queues.
	if samples.calls != 1 {
		t.Errorf("took %d fetches for 3 queues, want 1", samples.calls)
	}
	if got[quick] != 15*time.Second {
		t.Errorf("quick queue got %v, want 15s", got[quick])
	}
	if got[slow] != 300*time.Second {
		t.Errorf("slow queue got %v, want 300s", got[slow])
	}
	if _, ok := got[cold]; ok {
		t.Errorf("cold queue got %v, want it left out of the map", got[cold])
	}
}

func TestMedianEstimatorLeavesOutQueuesBelowTheSampleFloor(t *testing.T) {
	queueID := uuid.New()
	samples := &fakeSamples{byQueue: map[uuid.UUID][]Fill{queueID: fillsOf(10, 20, 30, 40)}}
	est := MedianEstimator{Samples: samples, MinSamples: 5}

	got, err := est.EstimateByModeQueue(context.Background(), []uuid.UUID{queueID}, time.Now())
	if err != nil {
		t.Fatalf("EstimateByModeQueue: %v", err)
	}
	if _, ok := got[queueID]; ok {
		t.Errorf("got %v from 4 samples under a floor of 5, want no entry", got[queueID])
	}
}

func TestMedianEstimatorReturnsNothingForAQueueThatHasNeverFilled(t *testing.T) {
	queueID := uuid.New()
	samples := &fakeSamples{}
	est := MedianEstimator{Samples: samples, MinSamples: 5}

	got, err := est.EstimateByModeQueue(context.Background(), []uuid.UUID{queueID}, time.Now())
	if err != nil {
		t.Fatalf("EstimateByModeQueue: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d estimates, want none", len(got))
	}
}

func TestMedianEstimatorAsksOnlyForFillsInsideItsWindow(t *testing.T) {
	queueID := uuid.New()
	samples := &fakeSamples{byQueue: map[uuid.UUID][]Fill{queueID: fillsOf(10, 20, 30, 40, 50)}}
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	est := MedianEstimator{Samples: samples, Window: 48 * time.Hour, MinSamples: 5, LimitPerQueue: 200}

	if _, err := est.EstimateByModeQueue(context.Background(), []uuid.UUID{queueID}, now); err != nil {
		t.Fatalf("EstimateByModeQueue: %v", err)
	}

	if len(samples.got.ModeQueueIDs) != 1 || samples.got.ModeQueueIDs[0] != queueID {
		t.Errorf("asked for queues %v, want just %v", samples.got.ModeQueueIDs, queueID)
	}
	if want := now.Add(-48 * time.Hour); !samples.got.Since.Equal(want) {
		t.Errorf("asked for fills since %v, want %v", samples.got.Since, want)
	}
	if samples.got.LimitPerQueue != 200 {
		t.Errorf("asked for %d fills per queue, want 200", samples.got.LimitPerQueue)
	}
}

func TestMedianEstimatorFallsBackToDefaultsWhenUnconfigured(t *testing.T) {
	samples := &fakeSamples{}
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	est := MedianEstimator{Samples: samples}

	if _, err := est.EstimateByModeQueue(context.Background(), nil, now); err != nil {
		t.Fatalf("EstimateByModeQueue: %v", err)
	}

	if want := now.Add(-DefaultWindow); !samples.got.Since.Equal(want) {
		t.Errorf("asked for fills since %v, want the default window back from now (%v)", samples.got.Since, want)
	}
	if samples.got.LimitPerQueue != DefaultLimitPerQueue {
		t.Errorf("asked for %d per queue, want DefaultLimitPerQueue %d", samples.got.LimitPerQueue, DefaultLimitPerQueue)
	}
}

func TestMedianEstimatorReportsSampleFailureRatherThanCallingEveryQueueCold(t *testing.T) {
	wantErr := errors.New("database is down")
	samples := &fakeSamples{err: wantErr}
	est := MedianEstimator{Samples: samples, MinSamples: 5}

	got, err := est.EstimateByModeQueue(context.Background(), []uuid.UUID{uuid.New()}, time.Now())
	if !errors.Is(err, wantErr) {
		t.Fatalf("got error %v, want it to wrap %v", err, wantErr)
	}
	if got != nil {
		t.Errorf("got estimates %v alongside an error, want none", got)
	}
}
