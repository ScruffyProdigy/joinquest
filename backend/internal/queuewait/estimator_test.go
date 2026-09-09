package queuewait

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// fakeSamples serves a fixed set of fills and records the query it was asked,
// so a test can assert on the window an estimator requested as well as the
// number it produced.
type fakeSamples struct {
	fills []Fill
	err   error
	got   FillQuery
	calls int
}

func (f *fakeSamples) RecentFills(_ context.Context, q FillQuery) ([]Fill, error) {
	f.got = q
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.fills, nil
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
	samples := &fakeSamples{fills: fillsOf(60, 10, 30, 15, 20)}
	est := MedianEstimator{Samples: samples, MinSamples: 5}

	got, err := est.Estimate(context.Background(), uuid.New(), time.Now())
	if err != nil {
		t.Fatalf("Estimate: %v", err)
	}
	if got == nil {
		t.Fatal("got no estimate, want 20s")
	}
	if *got != 20*time.Second {
		t.Errorf("got %v, want 20s", *got)
	}
}

func TestMedianEstimatorAveragesTheTwoMiddleWaitsForEvenSampleCount(t *testing.T) {
	samples := &fakeSamples{fills: fillsOf(40, 10, 30, 20)}
	est := MedianEstimator{Samples: samples, MinSamples: 4}

	got, err := est.Estimate(context.Background(), uuid.New(), time.Now())
	if err != nil {
		t.Fatalf("Estimate: %v", err)
	}
	if got == nil {
		t.Fatal("got no estimate, want 25s")
	}
	if *got != 25*time.Second {
		t.Errorf("got %v, want 25s", *got)
	}
}

func TestMedianEstimatorDeclinesToGuessBelowTheSampleFloor(t *testing.T) {
	samples := &fakeSamples{fills: fillsOf(10, 20, 30, 40)}
	est := MedianEstimator{Samples: samples, MinSamples: 5}

	got, err := est.Estimate(context.Background(), uuid.New(), time.Now())
	if err != nil {
		t.Fatalf("Estimate: %v", err)
	}
	if got != nil {
		t.Errorf("got %v, want no estimate from 4 samples under a floor of 5", *got)
	}
}

func TestMedianEstimatorDeclinesToGuessWithNoHistoryAtAll(t *testing.T) {
	samples := &fakeSamples{}
	est := MedianEstimator{Samples: samples, MinSamples: 5}

	got, err := est.Estimate(context.Background(), uuid.New(), time.Now())
	if err != nil {
		t.Fatalf("Estimate: %v", err)
	}
	if got != nil {
		t.Errorf("got %v, want no estimate from a queue that has never filled", *got)
	}
}

func TestMedianEstimatorAsksOnlyForFillsInsideItsWindow(t *testing.T) {
	samples := &fakeSamples{fills: fillsOf(10, 20, 30, 40, 50)}
	queueID := uuid.New()
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	est := MedianEstimator{Samples: samples, Window: 48 * time.Hour, MinSamples: 5, Limit: 200}

	if _, err := est.Estimate(context.Background(), queueID, now); err != nil {
		t.Fatalf("Estimate: %v", err)
	}

	if samples.got.ModeQueueID != queueID {
		t.Errorf("asked for queue %v, want %v", samples.got.ModeQueueID, queueID)
	}
	if want := now.Add(-48 * time.Hour); !samples.got.Since.Equal(want) {
		t.Errorf("asked for fills since %v, want %v", samples.got.Since, want)
	}
	if samples.got.Limit != 200 {
		t.Errorf("asked for limit %d, want 200", samples.got.Limit)
	}
}

func TestMedianEstimatorFallsBackToDefaultsWhenUnconfigured(t *testing.T) {
	samples := &fakeSamples{fills: fillsOf(10, 20, 30, 40, 50)}
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	est := MedianEstimator{Samples: samples}

	if _, err := est.Estimate(context.Background(), uuid.New(), now); err != nil {
		t.Fatalf("Estimate: %v", err)
	}

	if want := now.Add(-DefaultWindow); !samples.got.Since.Equal(want) {
		t.Errorf("asked for fills since %v, want the default window back from now (%v)", samples.got.Since, want)
	}
	if samples.got.Limit != DefaultLimit {
		t.Errorf("asked for limit %d, want DefaultLimit %d", samples.got.Limit, DefaultLimit)
	}
}

func TestMedianEstimatorReportsSampleFailureRatherThanCallingItACoolQueue(t *testing.T) {
	wantErr := errors.New("database is down")
	samples := &fakeSamples{err: wantErr}
	est := MedianEstimator{Samples: samples, MinSamples: 5}

	got, err := est.Estimate(context.Background(), uuid.New(), time.Now())
	if !errors.Is(err, wantErr) {
		t.Fatalf("got error %v, want it to wrap %v", err, wantErr)
	}
	if got != nil {
		t.Errorf("got estimate %v alongside an error, want none", *got)
	}
}
