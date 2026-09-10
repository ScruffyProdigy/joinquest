package graph

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/graph/model"
	"github.com/scruffyprodigy/joinquest/internal/pubsub"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

func TestQueueUpdateFromViewMatched(t *testing.T) {
	sessionID := uuid.New()
	gameID := uuid.MustParse("a1000000-0000-4000-8000-000000000001")
	queueID := uuid.New()

	launch := "http://localhost:5174/?match=" + sessionID.String() + "&token=eyJ.test"
	update := queueUpdateFromView(&store.UserQueueView{
		GameID:      gameID,
		ModeQueueID: queueID,
		InQueue:     true,
		Matched:     true,
		SessionID:   &sessionID,
	}, launch)

	if update == nil || update.Status != model.QueueStatusMatched {
		t.Fatalf("expected matched update, got %+v", update)
	}
	if update.JoinURL == nil || *update.JoinURL == "" {
		t.Fatalf("expected join URL on matched snapshot")
	}
}

func TestQueueUpdateFromViewWaiting(t *testing.T) {
	gameID := uuid.MustParse("a1000000-0000-4000-8000-000000000001")
	queueID := uuid.New()
	update := queueUpdateFromView(&store.UserQueueView{
		GameID:      gameID,
		ModeQueueID: queueID,
		InQueue:     true,
		Waiting:     true,
		QueuedCount: 2,
	}, "")

	if update == nil || update.Status != model.QueueStatusWaiting || update.QueuedCount != 2 {
		t.Fatalf("expected waiting update with count 2, got %+v", update)
	}
}

var errBrokerBlip = errors.New("broker blip")

// flakyBroker fails the publish for one channel and records every channel it was
// asked to publish on, so a test can tell "never attempted" apart from "attempted
// and failed".
type flakyBroker struct {
	mu           sync.Mutex
	failChannel  string
	published    []string
	attemptedAll []string
}

func (b *flakyBroker) Publish(_ context.Context, channel string, _ []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.attemptedAll = append(b.attemptedAll, channel)
	if channel == b.failChannel {
		return errBrokerBlip
	}
	b.published = append(b.published, channel)
	return nil
}

func (b *flakyBroker) Subscribe(context.Context, string) (<-chan []byte, func(), error) {
	return nil, func() {}, nil
}

func (b *flakyBroker) Close() error { return nil }

func (b *flakyBroker) publishedTo(userID uuid.UUID) bool {
	return b.sawChannel(b.published, userID)
}

func (b *flakyBroker) attempted(userID uuid.UUID) bool {
	return b.sawChannel(b.attemptedAll, userID)
}

func (b *flakyBroker) sawChannel(log []string, userID uuid.UUID) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	want := pubsub.UserQueueChannel(userID.String())
	for _, channel := range log {
		if channel == want {
			return true
		}
	}
	return false
}

// A publish failure for one player used to abort the whole fan-out, leaving every
// player after them waiting behind a match that had already formed (JQ-247).
func TestPublishQueueResultNotifiesEveryoneAfterAFailedPublish(t *testing.T) {
	users := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	const failIndex = 1 // not first, not last

	broker := &flakyBroker{failChannel: pubsub.UserQueueChannel(users[failIndex].String())}
	sessionID := uuid.New()
	r := &Resolver{PubSub: broker}

	result := &store.QueueJoinResult{
		GameID:        uuid.New(),
		ModeQueueID:   uuid.New(),
		Status:        store.QueueStatusMatched,
		SessionID:     &sessionID,
		NotifyUserIDs: users,
	}

	err := r.publishQueueResult(context.Background(), result, nil)

	if err == nil {
		t.Fatal("publishQueueResult returned nil, want the failed publish reported")
	}
	if !errors.Is(err, errBrokerBlip) {
		t.Fatalf("publishQueueResult error = %v, want it to wrap the broker failure", err)
	}
	if !broker.attempted(users[failIndex]) {
		t.Errorf("user %d (%s) was never attempted", failIndex, users[failIndex])
	}
	for i, userID := range users {
		if i == failIndex {
			continue
		}
		if !broker.publishedTo(userID) {
			t.Errorf("user %d (%s) was never notified", i, userID)
		}
	}
}

// The same shape in publishQueueSwitchFrom's waiter fan-out: the players left
// behind in the old queue all need the new count, not just the ones before the
// first broker hiccup.
func TestPublishQueueWaitingNotifiesEveryoneAfterAFailedPublish(t *testing.T) {
	waiters := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	const failIndex = 2 // not first, not last

	broker := &flakyBroker{failChannel: pubsub.UserQueueChannel(waiters[failIndex].String())}
	r := &Resolver{PubSub: broker}

	err := r.publishQueueWaiting(context.Background(), uuid.New(), uuid.New(), waiters, len(waiters))

	if err == nil {
		t.Fatal("publishQueueWaiting returned nil, want the failed publish reported")
	}
	if !errors.Is(err, errBrokerBlip) {
		t.Fatalf("publishQueueWaiting error = %v, want it to wrap the broker failure", err)
	}
	if !broker.attempted(waiters[failIndex]) {
		t.Errorf("waiter %d (%s) was never attempted", failIndex, waiters[failIndex])
	}
	for i, userID := range waiters {
		if i == failIndex {
			continue
		}
		if !broker.publishedTo(userID) {
			t.Errorf("waiter %d (%s) was never notified", i, userID)
		}
	}
}
