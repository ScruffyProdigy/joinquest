package graph

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/pubsub"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

// ExpiryFunc runs when a disconnected player's grace window elapses without a
// reconnect. What happens then is a decision, not a foregone conclusion — see
// DisconnectOutcome.
type ExpiryFunc func(ctx context.Context, userID uuid.UUID, stamp time.Time)

// presenceBackend is the slice of the store the tracker needs, named separately so
// the tracker's timer and refcount behaviour can be tested without a database.
type presenceBackend interface {
	Connected(ctx context.Context, userID uuid.UUID) (int, *time.Time, bool, error)
	Disconnected(ctx context.Context, userID uuid.UUID) (int, *time.Time, bool, error)
	// ReleaseFormingSlot deprioritises the player the moment their last socket
	// closes. It is a store method rather than forming internals reached from here,
	// so the graph layer keeps knowing nothing about seat assignments.
	ReleaseFormingSlot(ctx context.Context, userID uuid.UUID, stamp time.Time) error
}

// storePresenceBackend adapts *store.Store to presenceBackend.
type storePresenceBackend struct{ st *store.Store }

func (b storePresenceBackend) Connected(ctx context.Context, userID uuid.UUID) (int, *time.Time, bool, error) {
	t, err := b.st.PresenceConnected(ctx, userID)
	return t.ConnectionCount, t.DisconnectedAt, t.Edge, err
}

func (b storePresenceBackend) Disconnected(ctx context.Context, userID uuid.UUID) (int, *time.Time, bool, error) {
	t, err := b.st.PresenceDisconnected(ctx, userID)
	return t.ConnectionCount, t.DisconnectedAt, t.Edge, err
}

func (b storePresenceBackend) ReleaseFormingSlot(ctx context.Context, userID uuid.UUID, stamp time.Time) error {
	return b.st.ReleaseFormingSlotsForDisconnectedUser(ctx, userID, stamp)
}

// PresenceTracker turns per-user subscription lifetimes into presence edges.
//
// Every per-user subscription calls Track when it opens and the returned release when
// it closes. Only the LAST socket closing arms an expiry timer, and any reconnect
// cancels it — which is what stops a player losing their place because they closed one
// of the three sockets a single tab holds.
//
// The timer map is in-process, so it is correct at replicas: 1 and would need
// per-pod connection ownership on a scale-out. The durable half of the state lives in
// user_presence, so a restart is recoverable (see store.ResetPresenceOnBoot); only
// the pending timers are lost, and the queue sweep is the backstop for those.
type PresenceTracker struct {
	backend  presenceBackend
	broker   pubsub.Broker
	grace    time.Duration
	onExpire ExpiryFunc

	mu     sync.Mutex
	timers map[uuid.UUID]*time.Timer
}

// NewPresenceTracker builds a tracker over the real store.
func NewPresenceTracker(st *store.Store, broker pubsub.Broker, grace time.Duration, onExpire ExpiryFunc) *PresenceTracker {
	return newPresenceTrackerWithBackend(storePresenceBackend{st: st}, broker, grace, onExpire)
}

func newPresenceTrackerWithBackend(backend presenceBackend, broker pubsub.Broker, grace time.Duration, onExpire ExpiryFunc) *PresenceTracker {
	return &PresenceTracker{
		backend:  backend,
		broker:   broker,
		grace:    grace,
		onExpire: onExpire,
		timers:   make(map[uuid.UUID]*time.Timer),
	}
}

// Track records one live socket for the user and returns the release to call when
// that socket closes.
//
// Nil-safe on the receiver, so a Resolver built without presence (tests, tooling)
// needs no special casing at the six call sites. The returned release is idempotent,
// because a resolver that fails after Track may run both its error path and its
// deferred cleanup.
func (t *PresenceTracker) Track(ctx context.Context, userID uuid.UUID) func() {
	if t == nil {
		return func() {}
	}

	_, _, edge, err := t.backend.Connected(ctx, userID)
	if err != nil {
		// Presence is an optimisation for consumers, never a precondition for
		// serving the subscription: a failed write must not deny the player their
		// live updates.
		log.Printf("presence: connect for %s: %v", userID, err)
		return func() {}
	}
	if edge {
		t.cancelTimer(userID)
		t.publish(ctx, userID, pubsub.PresenceEvent{Status: pubsub.PresenceStatusConnected})
	}

	var once sync.Once
	return func() {
		once.Do(func() { t.release(userID) })
	}
}

func (t *PresenceTracker) release(userID uuid.UUID) {
	// The subscription's ctx is already cancelled by the time this runs — that
	// cancellation is the event — so the write needs a context of its own.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, stamp, edge, err := t.backend.Disconnected(ctx, userID)
	if err != nil {
		log.Printf("presence: disconnect for %s: %v", userID, err)
		return
	}
	if !edge || stamp == nil {
		// Another socket is still open; the player has not gone anywhere.
		return
	}

	// Deprioritisation starts here, not at expiry. A player is placed onto the
	// filling match within ~25ms of joining, and nothing else un-places them — so
	// without this the match fires with them in it while their phone is in their
	// pocket, and the ordering that sorts disconnected players last never applies to
	// the case it was written for. They stay queued and can be re-assigned if the
	// pool is otherwise too thin, which is what the rule actually asks for.
	if err := t.backend.ReleaseFormingSlot(ctx, userID, *stamp); err != nil {
		// Not fatal to the disconnect: the expiry path releases the slot again when
		// it removes the player, so a failure here costs deprioritisation, not
		// correctness.
		log.Printf("presence: release forming slot for %s: %v", userID, err)
	}

	t.publish(ctx, userID, pubsub.PresenceEvent{
		Status:         pubsub.PresenceStatusDisconnected,
		DisconnectedAt: stamp.UTC().Format(time.RFC3339Nano),
	})
	t.armTimer(userID, *stamp)
}

// armTimer schedules the expiry, capturing the stamp it was armed for. The store's
// eviction re-checks that stamp, so a timer that fires after a reconnect-and-
// disconnect cycle acts on nothing rather than on the newer window.
func (t *PresenceTracker) armTimer(userID uuid.UUID, stamp time.Time) {
	if t.onExpire == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if existing, ok := t.timers[userID]; ok {
		existing.Stop()
	}
	t.timers[userID] = time.AfterFunc(t.grace, func() {
		// time.AfterFunc runs this on its own goroutine, so a panic anywhere under
		// onExpire — a publish, a nil store — takes the whole API process down and
		// with it every other player's live socket. One player's expiry is not worth
		// that.
		defer func() {
			if r := recover(); r != nil {
				log.Printf("presence: expiry for %s panicked: %v", userID, r)
			}
		}()

		t.mu.Lock()
		delete(t.timers, userID)
		t.mu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		t.onExpire(ctx, userID, stamp)
	})
}

func (t *PresenceTracker) cancelTimer(userID uuid.UUID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if existing, ok := t.timers[userID]; ok {
		existing.Stop()
		delete(t.timers, userID)
	}
}

func (t *PresenceTracker) publish(ctx context.Context, userID uuid.UUID, event pubsub.PresenceEvent) {
	if t.broker == nil {
		return
	}
	if err := pubsub.PublishPresenceEvent(ctx, t.broker, userID.String(), event); err != nil {
		log.Printf("presence: publish %s for %s: %v", event.Status, userID, err)
	}
}
