package formingworker

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/pubsub"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

// ReconcileHook runs after a queue reconcile attempt (match provision + pubsub).
type ReconcileHook func(ctx context.Context, result *store.FormingReconcileResult) error

// ProvisionHook retries game-server provision for a matched session.
type ProvisionHook func(ctx context.Context, pending store.UnprovisionedSession) error

// Worker periodically reconciles forming matches and runs on demand after queue joins.
type Worker struct {
	store       *store.Store
	onResult    ReconcileHook
	onProvision ProvisionHook
	debounce    time.Duration
	tickEvery   time.Duration

	mu              sync.Mutex
	idle            *sync.Cond
	running         map[uuid.UUID]int
	timers          map[uuid.UUID]*time.Timer
	provisionTimers map[uuid.UUID]*time.Timer
	provisionRetry  map[uuid.UUID]int
}

func New(st *store.Store, hook ReconcileHook, debounce, tickEvery time.Duration) *Worker {
	if debounce <= 0 {
		debounce = 25 * time.Millisecond
	}
	if tickEvery <= 0 {
		tickEvery = 30 * time.Second
	}
	w := &Worker{
		store:           st,
		onResult:        hook,
		debounce:        debounce,
		tickEvery:       tickEvery,
		timers:          make(map[uuid.UUID]*time.Timer),
		running:         make(map[uuid.UUID]int),
		provisionTimers: make(map[uuid.UUID]*time.Timer),
		provisionRetry:  make(map[uuid.UUID]int),
	}
	w.idle = sync.NewCond(&w.mu)
	return w
}

// SetProvisionHook registers the handler for matched sessions awaiting game provision.
func (w *Worker) SetProvisionHook(hook ProvisionHook) {
	if w == nil {
		return
	}
	w.onProvision = hook
}

// DrainPending stops a debounced reconcile for this queue and waits for one that
// has already started. Stopping the timer is not enough on its own: a fired timer
// is a goroutine mid-reconcile, and that goroutine owns the provision and any
// rollback that follows it — so a caller about to read those rows has to wait for
// it rather than reconcile around it. Provision retries are not drained; their
// backoff is deliberately long. The context bounds the wait, so a worker that
// never goes quiet surfaces as a deadline instead of a hung test.
func (w *Worker) DrainPending(ctx context.Context, modeQueueID uuid.UUID) error {
	if w == nil || modeQueueID == uuid.Nil {
		return nil
	}

	// A cond carries no deadline of its own, so wake the waiter when ctx ends.
	stopOnDone := context.AfterFunc(ctx, func() {
		w.mu.Lock()
		defer w.mu.Unlock()
		w.idle.Broadcast()
	})
	defer stopOnDone()

	w.mu.Lock()
	defer w.mu.Unlock()
	if timer, ok := w.timers[modeQueueID]; ok {
		timer.Stop()
		delete(w.timers, modeQueueID)
	}
	for w.running[modeQueueID] > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		w.idle.Wait()
	}
	return nil
}

// beginRunLocked marks a reconcile as in flight. Call with w.mu held.
func (w *Worker) beginRunLocked(modeQueueID uuid.UUID) {
	w.running[modeQueueID]++
}

// endRun clears that mark and wakes anyone draining.
func (w *Worker) endRun(modeQueueID uuid.UUID) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.running[modeQueueID] > 1 {
		w.running[modeQueueID]--
	} else {
		delete(w.running, modeQueueID)
	}
	w.idle.Broadcast()
}

// Shutdown stops all pending reconcile and provision timers (test cleanup).
func (w *Worker) Shutdown() {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for id, timer := range w.timers {
		timer.Stop()
		delete(w.timers, id)
	}
	for id, timer := range w.provisionTimers {
		timer.Stop()
		delete(w.provisionTimers, id)
	}
	// A reconcile already running outlives the timer that started it, and in a
	// test it would go on writing to a shared queue after the test that started
	// it ended. Each run carries its own 30s timeout, so this cannot wait forever.
	for len(w.running) > 0 {
		w.idle.Wait()
	}
}

// Schedule coalesces rapid joins on the same queue and reconciles shortly after.
func (w *Worker) Schedule(modeQueueID uuid.UUID) {
	if w == nil || w.store == nil || modeQueueID == uuid.Nil {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if existing, ok := w.timers[modeQueueID]; ok {
		existing.Stop()
	}
	var timer *time.Timer
	timer = time.AfterFunc(w.debounce, func() {
		w.mu.Lock()
		// A timer that had already fired when it was drained or superseded runs
		// this func anyway; the map says whether the run is still wanted.
		if w.timers[modeQueueID] != timer {
			w.mu.Unlock()
			return
		}
		delete(w.timers, modeQueueID)
		// Marked in flight before the lock drops, so a drain arriving now waits
		// for this run instead of sailing past it.
		w.beginRunLocked(modeQueueID)
		w.mu.Unlock()
		defer w.endRun(modeQueueID)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := w.ReconcileNow(ctx, modeQueueID); err != nil {
			log.Printf("formingworker: reconcile %s: %v", modeQueueID, err)
		}
	})
	w.timers[modeQueueID] = timer
}

// ScheduleProvisionRetry re-attempts game provision after a transient failure.
func (w *Worker) ScheduleProvisionRetry(pending store.UnprovisionedSession) {
	if w == nil || w.onProvision == nil || pending.SessionID == uuid.Nil {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	attempt := w.provisionRetry[pending.SessionID]
	delay := ProvisionBackoff(attempt)
	w.provisionRetry[pending.SessionID] = attempt + 1

	if timer, ok := w.provisionTimers[pending.SessionID]; ok {
		timer.Stop()
	}
	sessionID := pending.SessionID
	w.provisionTimers[sessionID] = time.AfterFunc(delay, func() {
		w.mu.Lock()
		delete(w.provisionTimers, sessionID)
		w.mu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := w.ProvisionNow(ctx, pending); err != nil {
			log.Printf("formingworker: provision retry session=%s attempt=%d: %v", sessionID, attempt, err)
		}
	})
	pubsub.DebugLog(
		"provision retry scheduled session=%s queue=%s attempt=%d delay=%s",
		pending.SessionID,
		pending.ModeQueueID,
		attempt,
		delay,
	)
}

// ClearProvisionRetry resets backoff after a successful provision.
func (w *Worker) ClearProvisionRetry(sessionID uuid.UUID) {
	if w == nil || sessionID == uuid.Nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.provisionRetry, sessionID)
	if timer, ok := w.provisionTimers[sessionID]; ok {
		timer.Stop()
		delete(w.provisionTimers, sessionID)
	}
}

// ProvisionNow runs game provision for one matched session.
func (w *Worker) ProvisionNow(ctx context.Context, pending store.UnprovisionedSession) error {
	if w == nil || w.onProvision == nil || pending.SessionID == uuid.Nil {
		return nil
	}
	complete, err := w.store.SessionProvisionComplete(ctx, pending.SessionID)
	if err != nil {
		return err
	}
	if complete {
		w.ClearProvisionRetry(pending.SessionID)
		return nil
	}
	pubsub.DebugLog(
		"provision retry run session=%s queue=%s users=%d",
		pending.SessionID,
		pending.ModeQueueID,
		len(pending.NotifyUserIDs),
	)
	if err := w.onProvision(ctx, pending); err != nil {
		w.ScheduleProvisionRetry(pending)
		return err
	}
	w.ClearProvisionRetry(pending.SessionID)
	return nil
}

// ReconcileNow runs sync + fire for one mode queue immediately.
func (w *Worker) ReconcileNow(ctx context.Context, modeQueueID uuid.UUID) error {
	if w == nil || w.store == nil || modeQueueID == uuid.Nil {
		return nil
	}

	result, err := w.store.ReconcileFormingModeQueue(ctx, modeQueueID)
	if err != nil {
		return err
	}
	if result != nil {
		sessionID := ""
		if result.SessionID != nil {
			sessionID = result.SessionID.String()
		}
		pubsub.DebugLog(
			"reconcile queue=%s fired=%t queuedCount=%d notifyUsers=%d session=%s",
			modeQueueID,
			result.Fired,
			result.QueuedCount,
			len(result.NotifyUserIDs),
			sessionID,
		)
	}
	if w.onResult != nil && result != nil {
		return w.onResult(ctx, result)
	}
	return nil
}

// Start runs the periodic sweep until ctx is cancelled.
func (w *Worker) Start(ctx context.Context) {
	if w == nil || w.store == nil {
		return
	}

	ticker := time.NewTicker(w.tickEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.reconcileAll(ctx)
			w.provisionAll(ctx)
		}
	}
}

func (w *Worker) reconcileAll(ctx context.Context) {
	ids, err := w.store.ListModeQueuesNeedingReconcile(ctx)
	if err != nil {
		log.Printf("formingworker: list queues: %v", err)
		return
	}
	for _, id := range ids {
		runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		if err := w.ReconcileNow(runCtx, id); err != nil {
			log.Printf("formingworker: periodic reconcile %s: %v", id, err)
		}
		cancel()
	}
}

func (w *Worker) provisionAll(ctx context.Context) {
	if w.onProvision == nil {
		return
	}
	pending, err := w.store.ListSessionsNeedingProvision(ctx)
	if err != nil {
		log.Printf("formingworker: list unprovisioned sessions: %v", err)
		return
	}
	for _, session := range pending {
		runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		if err := w.ProvisionNow(runCtx, session); err != nil {
			log.Printf("formingworker: periodic provision session=%s: %v", session.SessionID, err)
		}
		cancel()
	}
}
