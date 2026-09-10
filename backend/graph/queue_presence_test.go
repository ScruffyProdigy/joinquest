package graph

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// fakePresenceStore counts sockets in memory, so tracker behaviour can be tested
// without a database. It mirrors the store's contract: only the last socket closing
// stamps, and only edges are reported.
type fakePresenceStore struct {
	mu       sync.Mutex
	count    int
	stamp    time.Time
	released []time.Time
}

func (f *fakePresenceStore) Connected(ctx context.Context, userID uuid.UUID) (int, *time.Time, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.count++
	return f.count, nil, f.count == 1, nil
}

func (f *fakePresenceStore) Disconnected(ctx context.Context, userID uuid.UUID) (int, *time.Time, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.count > 0 {
		f.count--
	}
	if f.count > 0 {
		return f.count, nil, false, nil
	}
	f.stamp = time.Now()
	stamp := f.stamp
	return 0, &stamp, true, nil
}

func (f *fakePresenceStore) ReleaseFormingSlot(ctx context.Context, userID uuid.UUID, stamp time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.released = append(f.released, stamp)
	return nil
}

// The tracker must deprioritise on the 1 -> 0 edge, not only at expiry: a player
// placed onto the filling match ~25ms after joining is otherwise matched into a game
// with their phone in their pocket.
func TestTrackerReleasesTheFormingSlotOnTheDisconnectEdge(t *testing.T) {
	fake := &fakePresenceStore{}
	tracker := newPresenceTrackerWithBackend(fake, nil, time.Hour, nil)

	userID := uuid.New()
	first := tracker.Track(context.Background(), userID)
	second := tracker.Track(context.Background(), userID)

	second()
	fake.mu.Lock()
	afterOne := len(fake.released)
	fake.mu.Unlock()
	if afterOne != 0 {
		t.Fatalf("released the seat while a socket was still open (%d releases)", afterOne)
	}

	first()
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.released) != 1 {
		t.Fatalf("seat releases after the last socket closed: got %d, want 1", len(fake.released))
	}
	if !fake.released[0].Equal(fake.stamp) {
		t.Fatal("released against a stamp other than the one this disconnect wrote")
	}
}

func TestTrackerFiresExpiryAfterTheGraceWindow(t *testing.T) {
	fired := make(chan time.Time, 1)
	tracker := newPresenceTrackerWithBackend(&fakePresenceStore{}, nil, 20*time.Millisecond,
		func(ctx context.Context, userID uuid.UUID, stamp time.Time) {
			fired <- stamp
		})

	release := tracker.Track(context.Background(), uuid.New())
	release()

	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatal("expiry never fired")
	}
}

// Coming back inside the window is the whole point of having one.
func TestTrackerCancelsExpiryOnReconnect(t *testing.T) {
	fired := make(chan time.Time, 1)
	tracker := newPresenceTrackerWithBackend(&fakePresenceStore{}, nil, 60*time.Millisecond,
		func(ctx context.Context, userID uuid.UUID, stamp time.Time) {
			fired <- stamp
		})

	userID := uuid.New()
	release := tracker.Track(context.Background(), userID)
	release()
	tracker.Track(context.Background(), userID)

	select {
	case <-fired:
		t.Fatal("expiry fired for a player who reconnected inside the window")
	case <-time.After(200 * time.Millisecond):
	}
}

// Closing one of two sockets must arm nothing: the player still holds a socket, and
// a single tab routinely holds three.
func TestTrackerDoesNotArmWhileASocketRemains(t *testing.T) {
	fired := make(chan time.Time, 1)
	tracker := newPresenceTrackerWithBackend(&fakePresenceStore{}, nil, 20*time.Millisecond,
		func(ctx context.Context, userID uuid.UUID, stamp time.Time) {
			fired <- stamp
		})

	userID := uuid.New()
	tracker.Track(context.Background(), userID)
	releaseSecond := tracker.Track(context.Background(), userID)
	releaseSecond()

	select {
	case <-fired:
		t.Fatal("expiry fired while the player still held a socket")
	case <-time.After(150 * time.Millisecond):
	}
}

// A resolver's early-error path may call release twice; that must not double-count a
// socket down and evict a player who is still connected.
func TestTrackerReleaseIsIdempotent(t *testing.T) {
	fake := &fakePresenceStore{}
	tracker := newPresenceTrackerWithBackend(fake, nil, time.Hour, nil)

	userID := uuid.New()
	tracker.Track(context.Background(), userID)
	release := tracker.Track(context.Background(), userID)
	release()
	release()

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.count != 1 {
		t.Fatalf("socket count after a doubled release: got %d, want 1", fake.count)
	}
}

// A nil tracker is what tests and any resolver built without presence get; Track must
// stay callable there rather than making every call site nil-check.
func TestNilTrackerIsSafe(t *testing.T) {
	var tracker *PresenceTracker
	release := tracker.Track(context.Background(), uuid.New())
	release()
}

// Two windows off one socket edge, on different clocks. The short one must fire on its
// own schedule while the long one is still running — a player 40s into a disconnect has
// lost their room and kept their queue place, and collapsing the two would mean the
// stricter window silently governed both.
func TestTrackerFiresEachWindowOnItsOwnClock(t *testing.T) {
	short := make(chan time.Time, 1)
	long := make(chan time.Time, 1)
	tracker := newPresenceTrackerWithBackend(&fakePresenceStore{}, nil, 2*time.Second,
		func(ctx context.Context, userID uuid.UUID, stamp time.Time) {
			long <- stamp
		}).WithExpiry(20*time.Millisecond,
		func(ctx context.Context, userID uuid.UUID, stamp time.Time) {
			short <- stamp
		})

	release := tracker.Track(context.Background(), uuid.New())
	release()

	select {
	case <-short:
	case <-time.After(2 * time.Second):
		t.Fatal("the short window never fired")
	}
	select {
	case <-long:
		t.Fatal("the long window fired on the short window's clock")
	default:
	}
}

// A reconnect ends every window at once. Whatever each was about to take away, the
// player is back before it did — so cancelling one and leaving the other armed would
// cost them something they returned in time to keep.
func TestTrackerReconnectCancelsEveryWindow(t *testing.T) {
	fired := make(chan string, 2)
	tracker := newPresenceTrackerWithBackend(&fakePresenceStore{}, nil, 40*time.Millisecond,
		func(ctx context.Context, userID uuid.UUID, stamp time.Time) {
			fired <- "long"
		}).WithExpiry(20*time.Millisecond,
		func(ctx context.Context, userID uuid.UUID, stamp time.Time) {
			fired <- "short"
		})

	userID := uuid.New()
	release := tracker.Track(context.Background(), userID)
	release()
	tracker.Track(context.Background(), userID)

	select {
	case which := <-fired:
		t.Fatalf("the %s window fired for a player who reconnected inside it", which)
	case <-time.After(300 * time.Millisecond):
	}
}

// Every window firing must drop the player's map entry. A player who disconnects and
// never returns is exactly the case the timers exist for, and exactly the case where no
// reconnect ever arrives to clean up after them — so a leak here grows for as long as
// the process lives.
func TestTrackerForgetsAPlayerOnceEveryWindowHasFired(t *testing.T) {
	done := make(chan struct{}, 2)
	noop := func(ctx context.Context, userID uuid.UUID, stamp time.Time) { done <- struct{}{} }
	tracker := newPresenceTrackerWithBackend(&fakePresenceStore{}, nil, 10*time.Millisecond, noop).
		WithExpiry(20*time.Millisecond, noop)

	release := tracker.Track(context.Background(), uuid.New())
	release()

	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("not every window fired")
		}
	}

	deadline := time.Now().Add(time.Second)
	for {
		tracker.mu.Lock()
		remaining := len(tracker.timers)
		tracker.mu.Unlock()
		if remaining == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("tracker still holds %d player(s) after every window fired", remaining)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
