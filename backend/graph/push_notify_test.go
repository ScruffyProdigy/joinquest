package graph

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/pubsub"
	"github.com/scruffyprodigy/joinquest/internal/push"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

// fakeSender records sends and replies with a scripted outcome per endpoint.
type fakeSender struct {
	publicKey string
	results   map[string]error
	sent      []string
}

func (f *fakeSender) Send(_ context.Context, sub push.Subscription, _ push.Notification) error {
	f.sent = append(f.sent, sub.Endpoint)
	return f.results[sub.Endpoint]
}

func (f *fakeSender) PublicKey() string { return f.publicKey }

// subscribeUser registers endpoints for a user and returns them.
func subscribeUser(t *testing.T, ctx context.Context, env *queueIntegrationEnv, userID uuid.UUID, n int) []string {
	t.Helper()
	var endpoints []string
	for i := 0; i < n; i++ {
		endpoint := "https://push.example.com/" + uuid.NewString()
		if _, err := env.Store.SavePushSubscription(ctx, store.SavePushSubscriptionParams{
			UserID: userID, Endpoint: endpoint, P256dh: "k", Auth: "a",
		}); err != nil {
			t.Fatalf("SavePushSubscription: %v", err)
		}
		endpoints = append(endpoints, endpoint)
	}
	return endpoints
}

func newPushTestUser(t *testing.T, ctx context.Context, env *queueIntegrationEnv, cleaner *store.TestCleaner, label string) uuid.UUID {
	t.Helper()
	user, err := env.Store.CreateUser(ctx, store.CreateUserParams{
		Email: label + "-" + uuid.NewString() + "@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(user.ID)
	return user.ID
}

func TestPushSeatHeldReportsDeliveryAndRecordsIt(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	ctx := context.Background()
	cleaner := env.newCleaner(t)

	userID := newPushTestUser(t, ctx, env, cleaner, "notify-ok")
	endpoints := subscribeUser(t, ctx, env, userID, 2)
	env.resolver.Push = &fakeSender{publicKey: "test-key"}

	outcome := env.resolver.PushSeatHeld(ctx, userID, "session-1", push.Notification{Title: "ready", TTL: 45 * time.Second})

	if outcome.Status != pubsub.PushDelivered {
		t.Fatalf("expected DELIVERED, got %q", outcome.Status)
	}
	if outcome.Attempted != 2 || outcome.Delivered != 2 {
		t.Fatalf("expected 2 attempted / 2 delivered, got %d/%d", outcome.Attempted, outcome.Delivered)
	}

	// A delivered push is evidence the install actually works.
	reach, err := env.Store.GetPushReachability(ctx, userID)
	if err != nil {
		t.Fatalf("GetPushReachability: %v", err)
	}
	if reach.LastDeliveredAt == nil {
		t.Fatal("a successful send should be recorded on the subscription")
	}
	_ = endpoints
}

// The behaviour JQ-199's tier depends on: a 410 must collapse reachability
// immediately, because the hold was taken on the belief the player was
// reachable and nothing will otherwise correct it before the ceiling expires.
func TestPushSeatHeldCollapsesToUndeliverableWhenEveryInstallIsGone(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	ctx := context.Background()
	cleaner := env.newCleaner(t)

	userID := newPushTestUser(t, ctx, env, cleaner, "notify-gone")
	endpoints := subscribeUser(t, ctx, env, userID, 2)

	results := map[string]error{}
	for _, e := range endpoints {
		results[e] = push.ErrSubscriptionGone
	}
	env.resolver.Push = &fakeSender{publicKey: "test-key", results: results}

	// Precondition: the hold would have started in the optimistic tier.
	if reachable, err := env.resolver.PushReachable(ctx, userID); err != nil || !reachable {
		t.Fatalf("precondition: expected reachable, got %v (err %v)", reachable, err)
	}

	outcome := env.resolver.PushSeatHeld(ctx, userID, "session-1", push.Notification{Title: "ready", TTL: 45 * time.Second})

	if outcome.Status != pubsub.PushUndeliverable {
		t.Fatalf("expected UNDELIVERABLE, got %q", outcome.Status)
	}
	if outcome.Expired != 2 {
		t.Fatalf("expected both installs marked expired, got %d", outcome.Expired)
	}

	// The hold can now see the player is unreachable and collapse to its floor.
	reachable, err := env.resolver.PushReachable(ctx, userID)
	if err != nil {
		t.Fatalf("PushReachable: %v", err)
	}
	if reachable {
		t.Fatal("a player whose every install was rejected must not still read as reachable")
	}

	_, reason, err := env.resolver.PushReachability(ctx, userID)
	if err != nil {
		t.Fatalf("PushReachability: %v", err)
	}
	// And crucially, distinguishable from never having opted in.
	if reason != store.ReachabilitySubscriptionExpired {
		t.Fatalf("expected subscription-expired, got %q", reason)
	}
}

func TestPushSeatHeldKeepsSubscriptionsWhenTheFailureIsTransient(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	ctx := context.Background()
	cleaner := env.newCleaner(t)

	userID := newPushTestUser(t, ctx, env, cleaner, "notify-transient")
	endpoints := subscribeUser(t, ctx, env, userID, 1)
	env.resolver.Push = &fakeSender{
		publicKey: "test-key",
		results:   map[string]error{endpoints[0]: errors.New("push service returned 503")},
	}

	outcome := env.resolver.PushSeatHeld(ctx, userID, "session-1", push.Notification{Title: "ready", TTL: 45 * time.Second})

	// FAILED, not UNDELIVERABLE: a push-service hiccup says nothing about
	// whether the player can be reached, and evicting them over it would be a
	// bug with real cost to the player.
	if outcome.Status != pubsub.PushFailed {
		t.Fatalf("expected FAILED, got %q", outcome.Status)
	}
	if outcome.Expired != 0 {
		t.Fatal("a transient failure must not expire the player's subscription")
	}
	if reachable, err := env.resolver.PushReachable(ctx, userID); err != nil || !reachable {
		t.Fatalf("the player should still be reachable after a transient failure, got %v", reachable)
	}
}

func TestPushSeatHeldTreatsOneLiveInstallAsSuccess(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	ctx := context.Background()
	cleaner := env.newCleaner(t)

	userID := newPushTestUser(t, ctx, env, cleaner, "notify-partial")
	endpoints := subscribeUser(t, ctx, env, userID, 2)
	env.resolver.Push = &fakeSender{
		publicKey: "test-key",
		results:   map[string]error{endpoints[0]: push.ErrSubscriptionGone},
	}

	outcome := env.resolver.PushSeatHeld(ctx, userID, "session-1", push.Notification{Title: "ready", TTL: 45 * time.Second})

	// The old phone is dead but the desktop rang. The player can still come
	// back, so the hold should not collapse.
	if outcome.Status != pubsub.PushDelivered {
		t.Fatalf("expected DELIVERED, got %q", outcome.Status)
	}
	if outcome.Delivered != 1 || outcome.Expired != 1 {
		t.Fatalf("expected 1 delivered / 1 expired, got %d/%d", outcome.Delivered, outcome.Expired)
	}
	if reachable, err := env.resolver.PushReachable(ctx, userID); err != nil || !reachable {
		t.Fatal("one working install is enough to stay reachable")
	}
}

func TestPushSeatHeldSkipsWhenThereIsNothingToSendTo(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	ctx := context.Background()
	cleaner := env.newCleaner(t)

	t.Run("no subscriptions", func(t *testing.T) {
		userID := newPushTestUser(t, ctx, env, cleaner, "notify-none")
		env.resolver.Push = &fakeSender{publicKey: "test-key"}

		outcome := env.resolver.PushSeatHeld(ctx, userID, "s", push.Notification{Title: "ready", TTL: 45 * time.Second})
		if outcome.Status != pubsub.PushSkipped || outcome.Attempted != 0 {
			t.Fatalf("expected SKIPPED with nothing attempted, got %q / %d", outcome.Status, outcome.Attempted)
		}
	})

	t.Run("deployment cannot send", func(t *testing.T) {
		userID := newPushTestUser(t, ctx, env, cleaner, "notify-unconfigured")
		subscribeUser(t, ctx, env, userID, 1)
		sender := &fakeSender{publicKey: ""}
		env.resolver.Push = sender

		outcome := env.resolver.PushSeatHeld(ctx, userID, "s", push.Notification{Title: "ready", TTL: 45 * time.Second})
		if outcome.Status != pubsub.PushSkipped {
			t.Fatalf("expected SKIPPED, got %q", outcome.Status)
		}
		// Skipped, not failed: no attempt was made, and the hold was never
		// entitled to the optimistic tier in the first place.
		if len(sender.sent) != 0 {
			t.Fatal("nothing should be sent on a deployment with no VAPID keys")
		}
	})
}

// The hold subscribes to the outcome rather than polling, so it has to actually
// be published -- including in the cases where nothing was sent.
func TestPushSeatHeldPublishesTheOutcomeForTheSeatHold(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	ctx := context.Background()
	cleaner := env.newCleaner(t)

	userID := newPushTestUser(t, ctx, env, cleaner, "notify-publish")
	endpoints := subscribeUser(t, ctx, env, userID, 1)

	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	messages, unsubscribe, err := env.resolver.PubSub.Subscribe(subCtx, pubsub.UserPushChannel(userID.String()))
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer unsubscribe()

	env.resolver.Push = &fakeSender{
		publicKey: "test-key",
		results:   map[string]error{endpoints[0]: push.ErrSubscriptionGone},
	}
	env.resolver.PushSeatHeld(ctx, userID, "session-42", push.Notification{Title: "ready", TTL: 45 * time.Second})

	select {
	case payload := <-messages:
		event, err := pubsub.UnmarshalPushDeliveryEvent(payload)
		if err != nil {
			t.Fatalf("UnmarshalPushDeliveryEvent: %v", err)
		}
		if event.Status != pubsub.PushUndeliverable {
			t.Fatalf("expected UNDELIVERABLE on the wire, got %q", event.Status)
		}
		if event.UserID != userID.String() {
			t.Fatalf("wrong user on the event: %q", event.UserID)
		}
		// The session id lets a late event for a previous match be discarded.
		if event.HoldID != "session-42" {
			t.Fatalf("expected the hold id to ride along, got %q", event.HoldID)
		}
	case <-time.After(5 * time.Second):
		// Bounded rather than waiting on ctx.Done(): the test context has no
		// deadline, so a missing event would hang the suite instead of failing.
		t.Fatal("no delivery outcome was published")
	}
}

// Best-effort by contract: this runs inside match-formation fan-out, and a
// match that formed correctly must not fail because a push service hiccupped.
func TestPushSeatHeldNeverReturnsAnErrorThatCouldAbortAMatch(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	ctx := context.Background()
	cleaner := env.newCleaner(t)

	userID := newPushTestUser(t, ctx, env, cleaner, "notify-besteffort")
	endpoints := subscribeUser(t, ctx, env, userID, 1)
	env.resolver.Push = &fakeSender{
		publicKey: "test-key",
		results:   map[string]error{endpoints[0]: errors.New("catastrophic")},
	}
	// No broker at all, on top of the send failing.
	env.resolver.PubSub = nil

	// The signature has no error return by design; this asserts it also does
	// not panic with everything failing at once.
	outcome := env.resolver.PushSeatHeld(ctx, userID, "s", push.Notification{Title: "ready", TTL: 45 * time.Second})
	if outcome.Status != pubsub.PushFailed {
		t.Fatalf("expected FAILED, got %q", outcome.Status)
	}
}

func TestSeatHeldNotificationCarriesTheRemainingBudget(t *testing.T) {
	note, ok := SeatHeldNotification("https://joinquest.cc/launch/abc", 40*time.Second)
	if !ok {
		t.Fatal("a seat with budget left should be notifiable")
	}
	// Exactly the remaining budget, not a ceiling: under a match-level stall
	// budget a busy queue frees the seat early, and a TTL of "the ceiling"
	// would promise two minutes on a seat with forty seconds left.
	if note.TTL != 40*time.Second {
		t.Fatalf("expected the TTL to be the remaining budget, got %s", note.TTL)
	}
	if note.URL != "https://joinquest.cc/launch/abc" {
		t.Fatalf("the tap target must be the launch step, got %q", note.URL)
	}
	if note.Tag == "" {
		t.Fatal("a tag is needed so a retry replaces rather than stacks")
	}
}

func TestSeatHeldNotificationRefusesAnExpiredBudget(t *testing.T) {
	for _, remaining := range []time.Duration{0, -1 * time.Second} {
		if _, ok := SeatHeldNotification("https://joinquest.cc/launch/abc", remaining); ok {
			// The seat is already gone. Telling the player to claim it lands
			// them on a released seat -- the dead end JQ-199 AC #5 exists to
			// prevent, and worse than sending nothing.
			t.Fatalf("a seat with %s left must not be notifiable", remaining)
		}
	}
}

func TestPushSeatHeldRefusesAnUnboundedNotification(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	ctx := context.Background()
	cleaner := env.newCleaner(t)

	userID := newPushTestUser(t, ctx, env, cleaner, "notify-unbounded")
	subscribeUser(t, ctx, env, userID, 1)
	sender := &fakeSender{publicKey: "test-key"}
	env.resolver.Push = sender

	// No TTL. Without the guard the push service applies its own retention,
	// which can be days -- the player would be told to claim a seat that was
	// released long ago.
	outcome := env.resolver.PushSeatHeld(ctx, userID, "s", push.Notification{Title: "ready"})

	if outcome.Status != pubsub.PushSkipped {
		t.Fatalf("expected SKIPPED, got %q", outcome.Status)
	}
	if len(sender.sent) != 0 {
		t.Fatal("an unbounded seat-held notification must never reach the push service")
	}
}
