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
	byQueue map[uuid.UUID]time.Duration
	err     error
	calls   int
}

func (f *fakeEstimator) EstimateByModeQueue(_ context.Context, _ []uuid.UUID, _ time.Time) (map[uuid.UUID]time.Duration, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.byQueue, nil
}

// resolverFor wires a resolver to a fake estimate, through a real cache.
func resolverFor(est *fakeEstimator) *Resolver {
	return &Resolver{WaitEstimates: queuewait.NewCache(est, time.Minute)}
}

func TestModeQueueEstimatedWaitSecondsRoundsToWholeSeconds(t *testing.T) {
	queueID := uuid.New()
	r := resolverFor(&fakeEstimator{byQueue: map[uuid.UUID]time.Duration{
		queueID: 15600 * time.Millisecond,
	}})

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
}

func TestModeQueueEstimatedWaitSecondsCostsOneEstimateForAWholePageOfCards(t *testing.T) {
	first, second, third := uuid.New(), uuid.New(), uuid.New()
	est := &fakeEstimator{byQueue: map[uuid.UUID]time.Duration{
		first:  10 * time.Second,
		second: 20 * time.Second,
		third:  30 * time.Second,
	}}
	r := resolverFor(est)

	for _, queueID := range []uuid.UUID{first, second, third} {
		got, err := r.ModeQueue().EstimatedWaitSeconds(context.Background(), &model.ModeQueue{ID: queueID.String()})
		if err != nil {
			t.Fatalf("EstimatedWaitSeconds(%v): %v", queueID, err)
		}
		if got == nil {
			t.Fatalf("queue %v got no estimate, want one", queueID)
		}
	}

	if est.calls != 1 {
		t.Errorf("estimated %d times for 3 mode cards, want 1 — this field must not be an N+1", est.calls)
	}
}

func TestModeQueueEstimatedWaitSecondsStaysNullForAColdQueue(t *testing.T) {
	r := resolverFor(&fakeEstimator{})

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
	r := resolverFor(&fakeEstimator{err: wantErr})

	got, err := r.ModeQueue().EstimatedWaitSeconds(context.Background(), &model.ModeQueue{ID: uuid.NewString()})
	if !errors.Is(err, wantErr) {
		t.Fatalf("got error %v, want it to wrap %v — a broken lookup must not read as a cold queue", err, wantErr)
	}
	if got != nil {
		t.Errorf("got %d alongside an error, want null", *got)
	}
}

func TestModeQueueEstimatedWaitSecondsRejectsAMalformedQueueID(t *testing.T) {
	r := resolverFor(&fakeEstimator{})

	if _, err := r.ModeQueue().EstimatedWaitSeconds(context.Background(), &model.ModeQueue{ID: "not-a-uuid"}); err == nil {
		t.Fatal("got no error for a malformed queue id, want one")
	}
}

func TestWaitEstimatesDefaultToACacheOverTheMedianStrategy(t *testing.T) {
	r := &Resolver{}

	if r.waitEstimates() == nil {
		t.Error("got no wait-estimate cache, want one built by default")
	}
}

func TestWaitEstimatorReadsItsWindowAndSampleFloorFromTheEnvironment(t *testing.T) {
	t.Setenv("LOBBY_WAIT_ESTIMATE_WINDOW_DAYS", "2")
	t.Setenv("LOBBY_WAIT_ESTIMATE_MIN_SAMPLES", "12")

	est := newMedianEstimator(nil)
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

	est := newMedianEstimator(nil)
	if est.Window != 0 || est.MinSamples != 0 {
		t.Errorf("got window %v and floor %d, want both left unset so the package defaults apply", est.Window, est.MinSamples)
	}
}
