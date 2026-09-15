package graph

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// The away reading is not cancelled by the expiry being cancelled: watchers have already
// been told the player is gone, and nothing re-reads the roster on its own. So the tracker
// has to announce the return on the connect edge, symmetrically with the expiry that
// announced the departure.
func TestTrackerCallsBackOnTheConnectEdge(t *testing.T) {
	returned := make(chan uuid.UUID, 4)
	tracker := newPresenceTrackerWithBackend(&fakePresenceStore{}, nil, time.Hour, nil).
		WithReconnect(func(ctx context.Context, userID uuid.UUID) {
			returned <- userID
		})

	userID := uuid.New()
	release := tracker.Track(context.Background(), userID)

	select {
	case got := <-returned:
		if got != userID {
			t.Fatalf("called back for %s, want %s", got, userID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the connect edge never called back, so a returning player stays drawn as away")
	}

	release()
}

// One tab holds three sockets. Calling back for each of them would publish three times per
// reconnect for a reading that changed once — the "bounded, not a tick" half of this, from
// the other end.
func TestTrackerDoesNotCallBackWhileASocketIsAlreadyOpen(t *testing.T) {
	returned := make(chan uuid.UUID, 4)
	tracker := newPresenceTrackerWithBackend(&fakePresenceStore{}, nil, time.Hour, nil).
		WithReconnect(func(ctx context.Context, userID uuid.UUID) {
			returned <- userID
		})

	userID := uuid.New()
	first := tracker.Track(context.Background(), userID)
	<-returned
	second := tracker.Track(context.Background(), userID)

	select {
	case <-returned:
		t.Fatal("a second socket on a player who was already connected called back as a return")
	case <-time.After(200 * time.Millisecond):
	}

	second()
	first()
}
