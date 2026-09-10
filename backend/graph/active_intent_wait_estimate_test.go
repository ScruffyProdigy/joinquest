package graph

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/graph/model"
)

// fakePlayerEstimator stands in for the live strategy so the resolver's own
// behaviour — rounding, nulls, errors — is testable without a database.
type fakePlayerEstimator struct {
	wait  time.Duration
	ok    bool
	err   error
	calls int
}

func (f *fakePlayerEstimator) EstimateForPlayer(_ context.Context, _ uuid.UUID, _ time.Time) (time.Duration, bool, error) {
	f.calls++
	if f.err != nil {
		return 0, false, f.err
	}
	return f.wait, f.ok, nil
}

// waitingIntent is one player queued and waiting — the only shape that has a
// wait to estimate.
func waitingIntent() *model.ActiveIntent {
	queueID := uuid.NewString()
	return &model.ActiveIntent{QueueID: &queueID, Status: model.QueueStatusWaiting}
}

func liveResolverFor(est *fakePlayerEstimator) *Resolver {
	return &Resolver{LiveWait: est}
}

func TestActiveIntentEstimatedWaitSecondsRoundsToWholeSeconds(t *testing.T) {
	r := liveResolverFor(&fakePlayerEstimator{wait: 15600 * time.Millisecond, ok: true})

	got, err := r.ActiveIntent().EstimatedWaitSeconds(context.Background(), waitingIntent())
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

// No estimate is null rather than zero: a badge that says "0 sec" is a worse
// answer than no badge, which is the JQ-58 decision this inherits.
func TestActiveIntentEstimatedWaitSecondsIsNullWithoutAnEstimate(t *testing.T) {
	r := liveResolverFor(&fakePlayerEstimator{ok: false})

	got, err := r.ActiveIntent().EstimatedWaitSeconds(context.Background(), waitingIntent())
	if err != nil {
		t.Fatalf("EstimatedWaitSeconds: %v", err)
	}
	if got != nil {
		t.Errorf("got %d, want null", *got)
	}
}

// A matched player is not in line any more, so there is nothing to ask about —
// and asking would be a query per read for no reason.
func TestActiveIntentEstimatedWaitSecondsIsNullOnceMatched(t *testing.T) {
	est := &fakePlayerEstimator{wait: time.Minute, ok: true}
	intent := waitingIntent()
	intent.Status = model.QueueStatusMatched

	got, err := liveResolverFor(est).ActiveIntent().EstimatedWaitSeconds(context.Background(), intent)
	if err != nil {
		t.Fatalf("EstimatedWaitSeconds: %v", err)
	}
	if got != nil {
		t.Errorf("got %d, want null", *got)
	}
	if est.calls != 0 {
		t.Errorf("estimator consulted %d times, want 0 for a matched player", est.calls)
	}
}

// An intent with no queue row — a player seated at a forming table rather than
// standing in a line — has no position to divide.
func TestActiveIntentEstimatedWaitSecondsIsNullWithoutAQueueID(t *testing.T) {
	est := &fakePlayerEstimator{wait: time.Minute, ok: true}
	intent := waitingIntent()
	intent.QueueID = nil

	got, err := liveResolverFor(est).ActiveIntent().EstimatedWaitSeconds(context.Background(), intent)
	if err != nil {
		t.Fatalf("EstimatedWaitSeconds: %v", err)
	}
	if got != nil {
		t.Errorf("got %d, want null", *got)
	}
	if est.calls != 0 {
		t.Errorf("estimator consulted %d times, want 0 without a queue id", est.calls)
	}
}

// A broken query stays distinguishable from a cold line rather than failing
// open to null, matching how the catalog field handles it.
func TestActiveIntentEstimatedWaitSecondsPropagatesErrors(t *testing.T) {
	r := liveResolverFor(&fakePlayerEstimator{err: errors.New("boom")})

	if _, err := r.ActiveIntent().EstimatedWaitSeconds(context.Background(), waitingIntent()); err == nil {
		t.Error("got nil error, want the estimator's error propagated")
	}
}

// One player, one resolve: the per-player path deliberately does not go through
// the whole-catalog snapshot, and must not become a query per card either.
func TestActiveIntentEstimatedWaitSecondsCostsOneEstimatePerRead(t *testing.T) {
	est := &fakePlayerEstimator{wait: time.Minute, ok: true}

	if _, err := liveResolverFor(est).ActiveIntent().EstimatedWaitSeconds(context.Background(), waitingIntent()); err != nil {
		t.Fatalf("EstimatedWaitSeconds: %v", err)
	}
	if est.calls != 1 {
		t.Errorf("estimator consulted %d times, want 1", est.calls)
	}
}
