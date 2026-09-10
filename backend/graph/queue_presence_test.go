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
	mu    sync.Mutex
	count int
	stamp time.Time
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
