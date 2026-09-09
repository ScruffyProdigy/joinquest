package graph

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/graph/model"
	"github.com/scruffyprodigy/joinquest/internal/queuewait"
)

// fakeEstimator stands in for a real strategy so the resolver's own behaviour —
// rounding, nulls, errors — is testable without a database.
type fakeEstimator struct {
	wait *time.Duration
	err  error
	got  uuid.UUID
}

func (f *fakeEstimator) Estimate(_ context.Context, modeQueueID uuid.UUID, _ time.Time) (*time.Duration, error) {
	f.got = modeQueueID
	return f.wait, f.err
}

func durationPtr(d time.Duration) *time.Duration { return &d }

func TestModeQueueEstimatedWaitSecondsRoundsToWholeSeconds(t *testing.T) {
	queueID := uuid.New()
	est := &fakeEstimator{wait: durationPtr(15600 * time.Millisecond)}
	r := &Resolver{WaitEstimator: est}

	got, err := r.ModeQueue().EstimatedWaitSeconds(context.Background(), &model.ModeQueue{ID: queueID.String()})
	if err != nil {
		t.Fatalf("EstimatedWaitSeconds: %v", err)
	}
	if got == nil {
		t.Fatal("got no estimate, want 16")
	}
	if *got != 16 {
		t.Errorf("got %d seconds, want 15.6s rounded to 16", *got)
	}
	if est.got != queueID {
		t.Errorf("estimated for queue %v, want %v", est.got, queueID)
	}
}

func TestModeQueueEstimatedWaitSecondsStaysNullForAColdQueue(t *testing.T) {
	r := &Resolver{WaitEstimator: &fakeEstimator{}}

	got, err := r.ModeQueue().EstimatedWaitSeconds(context.Background(), &model.ModeQueue{ID: uuid.NewString()})
	if err != nil {
		t.Fatalf("EstimatedWaitSeconds: %v", err)
	}
	if got != nil {
		t.Errorf("got %d, want null when the estimator declines to guess", *got)
	}
}

func TestModeQueueEstimatedWaitSecondsPropagatesEstimatorFailure(t *testing.T) {
	wantErr := errors.New("database is down")
	r := &Resolver{WaitEstimator: &fakeEstimator{err: wantErr}}

	got, err := r.ModeQueue().EstimatedWaitSeconds(context.Background(), &model.ModeQueue{ID: uuid.NewString()})
	if !errors.Is(err, wantErr) {
		t.Fatalf("got error %v, want it to wrap %v — a broken lookup must not read as a cold queue", err, wantErr)
	}
	if got != nil {
		t.Errorf("got %d alongside an error, want null", *got)
	}
}

func TestModeQueueEstimatedWaitSecondsRejectsAMalformedQueueID(t *testing.T) {
	r := &Resolver{WaitEstimator: &fakeEstimator{wait: durationPtr(time.Second)}}

	if _, err := r.ModeQueue().EstimatedWaitSeconds(context.Background(), &model.ModeQueue{ID: "not-a-uuid"}); err == nil {
		t.Fatal("got no error for a malformed queue id, want one")
	}
}

func TestWaitEstimatorDefaultsToTheMedianStrategy(t *testing.T) {
	r := &Resolver{}

	if _, ok := r.waitEstimator().(queuewait.MedianEstimator); !ok {
		t.Errorf("got %T, want a MedianEstimator by default", r.waitEstimator())
	}
}

func TestWaitEstimatorReadsItsWindowAndSampleFloorFromTheEnvironment(t *testing.T) {
	t.Setenv("LOBBY_WAIT_ESTIMATE_WINDOW_DAYS", "2")
	t.Setenv("LOBBY_WAIT_ESTIMATE_MIN_SAMPLES", "12")
	r := &Resolver{}

	est, ok := r.waitEstimator().(queuewait.MedianEstimator)
	if !ok {
		t.Fatalf("got %T, want a MedianEstimator", r.waitEstimator())
	}
	if want := 48 * time.Hour; est.Window != want {
		t.Errorf("got window %v, want %v", est.Window, want)
	}
	if est.MinSamples != 12 {
		t.Errorf("got sample floor %d, want 12", est.MinSamples)
	}
}

func TestWaitEstimatorIgnoresNonsenseEnvironmentOverrides(t *testing.T) {
	t.Setenv("LOBBY_WAIT_ESTIMATE_WINDOW_DAYS", "not-a-number")
	t.Setenv("LOBBY_WAIT_ESTIMATE_MIN_SAMPLES", "-3")
	r := &Resolver{}

	est, ok := r.waitEstimator().(queuewait.MedianEstimator)
	if !ok {
		t.Fatalf("got %T, want a MedianEstimator", r.waitEstimator())
	}
	if est.Window != 0 || est.MinSamples != 0 {
		t.Errorf("got window %v and floor %d, want both left unset so the package defaults apply", est.Window, est.MinSamples)
	}
}
