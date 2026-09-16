package formingworker

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/scruffyprodigy/joinquest/internal/store"
	"github.com/scruffyprodigy/joinquest/internal/testdb"
)

func newTestWorker(t *testing.T, hook ReconcileHook, debounce time.Duration) *Worker {
	t.Helper()

	db, err := sql.Open("postgres", testdb.RequireURL(t))
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	return New(store.New(db), hook, debounce, time.Minute)
}

func drainContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// TestDrainPendingOutlastsAFiredReconcile is the guarantee callers actually need.
// Once a debounced reconcile has fired, stopping the timer cannot reach it, so the
// only way to read the queue without racing the worker is to wait for the run —
// hook included, since that is where a match is provisioned or rolled back.
func TestDrainPendingOutlastsAFiredReconcile(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})

	w := newTestWorker(t, func(context.Context, *store.FormingReconcileResult) error {
		close(started)
		<-release
		close(finished)
		return nil
	}, time.Millisecond)
	t.Cleanup(w.Shutdown)

	// An empty queue is enough: the reconcile finds nothing to do and still runs
	// the hook, which is the part of the run that outlives the timer.
	queueID := uuid.New()
	w.Schedule(queueID)
	<-started

	drained := make(chan error, 1)
	go func() { drained <- w.DrainPending(drainContext(t), queueID) }()

	select {
	case err := <-drained:
		t.Fatalf("DrainPending returned while the reconcile was still running (err=%v)", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	<-finished

	select {
	case err := <-drained:
		if err != nil {
			t.Fatalf("DrainPending: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("DrainPending never returned after the reconcile finished")
	}
}

// TestDrainPendingStopsAnArmedReconcile covers the other half: a reconcile that
// has been scheduled but has not fired yet must not run after the drain, or it
// would rewrite rows the caller is about to read.
func TestDrainPendingStopsAnArmedReconcile(t *testing.T) {
	ran := make(chan struct{}, 1)
	w := newTestWorker(t, func(context.Context, *store.FormingReconcileResult) error {
		ran <- struct{}{}
		return nil
	}, 50*time.Millisecond)
	t.Cleanup(w.Shutdown)

	queueID := uuid.New()
	w.Schedule(queueID)
	if err := w.DrainPending(drainContext(t), queueID); err != nil {
		t.Fatalf("DrainPending: %v", err)
	}

	select {
	case <-ran:
		t.Fatal("a drained reconcile ran anyway")
	case <-time.After(200 * time.Millisecond):
	}
}

// TestDrainPendingHonoursItsContext keeps the wait from becoming a hang: a worker
// that never goes quiet must surface as a deadline, not a stuck test.
func TestDrainPendingHonoursItsContext(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	w := newTestWorker(t, func(context.Context, *store.FormingReconcileResult) error {
		close(started)
		<-release
		return nil
	}, time.Millisecond)
	t.Cleanup(func() {
		close(release)
		w.Shutdown()
	})

	queueID := uuid.New()
	w.Schedule(queueID)
	<-started

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := w.DrainPending(ctx, queueID); err == nil {
		t.Fatal("expected DrainPending to report its deadline, got nil")
	}
}

// TestShutdownOutlastsAFiredReconcile stops a finished test's leftovers from
// writing to a queue the next test is using.
func TestShutdownOutlastsAFiredReconcile(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var ranToCompletion bool

	w := newTestWorker(t, func(context.Context, *store.FormingReconcileResult) error {
		close(started)
		<-release
		ranToCompletion = true
		return nil
	}, time.Millisecond)

	w.Schedule(uuid.New())
	<-started

	done := make(chan struct{})
	go func() {
		w.Shutdown()
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("Shutdown returned while a reconcile was still running")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown never returned after the reconcile finished")
	}
	if !ranToCompletion {
		t.Fatal("Shutdown returned before the hook finished")
	}
}
