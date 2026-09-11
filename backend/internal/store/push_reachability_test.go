package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// verifyTestSubscription stands in for the round trip: a push went out and the
// service worker acked it. A bare save is only the browser's claim.
func verifyTestSubscription(t *testing.T, st *Store, userID uuid.UUID, endpoint string) {
	t.Helper()
	ctx := context.Background()
	_, token, err := st.StartPushSubscriptionVerification(ctx, userID, endpoint)
	if err != nil {
		t.Fatalf("StartPushSubscriptionVerification: %v", err)
	}
	if _, err := st.ConfirmPushSubscriptionVerification(ctx, token); err != nil {
		t.Fatalf("ConfirmPushSubscriptionVerification: %v", err)
	}
}

// The distinctions are the whole point of the record: each reason has a
// different fix, so a bare boolean would make JQ-199's tier values untunable.
func TestGetPushReachabilityDistinguishesWhyAPlayerIsUnreachable(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	t.Run("never subscribed", func(t *testing.T) {
		userID := createPushTestUser(t, st, cleaner, "reach-never")

		reach, err := st.GetPushReachability(ctx, userID)
		if err != nil {
			t.Fatalf("GetPushReachability: %v", err)
		}
		if reach.Reason(true) != ReachabilityNeverSubscribed {
			t.Fatalf("expected never-subscribed, got %q", reach.Reason(true))
		}
		if reach.Reachable(true) {
			t.Fatal("a player who never opted in is not reachable")
		}
	})

	t.Run("registered but never reached", func(t *testing.T) {
		userID := createPushTestUser(t, st, cleaner, "reach-unverified")
		if _, err := st.SavePushSubscription(ctx, SavePushSubscriptionParams{
			UserID: userID, Endpoint: "https://push.example.com/" + uuid.NewString(),
			P256dh: "k", Auth: "a",
		}); err != nil {
			t.Fatalf("SavePushSubscription: %v", err)
		}

		reach, err := st.GetPushReachability(ctx, userID)
		if err != nil {
			t.Fatalf("GetPushReachability: %v", err)
		}
		// Its own reason, and its own fix: the endpoint may be stale, the keys
		// wrong, or the push service dropping us. None of those is a player
		// who did not opt in.
		if reach.Reason(true) != ReachabilityUnverified {
			t.Fatalf("expected unverified, got %q", reach.Reason(true))
		}
		if reach.Reachable(true) {
			t.Fatal("a claim is not a delivery path")
		}
	})

	t.Run("opted in and still live", func(t *testing.T) {
		userID := createPushTestUser(t, st, cleaner, "reach-live")
		endpoint := "https://push.example.com/" + uuid.NewString()
		if _, err := st.SavePushSubscription(ctx, SavePushSubscriptionParams{
			UserID: userID, Endpoint: endpoint,
			P256dh: "k", Auth: "a", UserAgent: "Mozilla/5.0 (iPhone)",
		}); err != nil {
			t.Fatalf("SavePushSubscription: %v", err)
		}
		verifyTestSubscription(t, st, userID, endpoint)

		reach, err := st.GetPushReachability(ctx, userID)
		if err != nil {
			t.Fatalf("GetPushReachability: %v", err)
		}
		if reach.Reason(true) != ReachabilityReachable {
			t.Fatalf("expected reachable, got %q", reach.Reason(true))
		}
		if len(reach.Live) != 1 {
			t.Fatalf("expected one live install, got %d", len(reach.Live))
		}
		// The User-Agent rides along so the iOS/Android split is answerable
		// from real registrations.
		if reach.Live[0].UserAgent == "" {
			t.Fatal("the install's user agent should be available for diagnosis")
		}
	})

	t.Run("opted in but every install expired", func(t *testing.T) {
		userID := createPushTestUser(t, st, cleaner, "reach-expired")
		endpoint := "https://push.example.com/" + uuid.NewString()
		if _, err := st.SavePushSubscription(ctx, SavePushSubscriptionParams{
			UserID: userID, Endpoint: endpoint, P256dh: "k", Auth: "a",
		}); err != nil {
			t.Fatalf("SavePushSubscription: %v", err)
		}
		if err := st.MarkPushSubscriptionExpired(ctx, endpoint); err != nil {
			t.Fatalf("MarkPushSubscriptionExpired: %v", err)
		}

		reach, err := st.GetPushReachability(ctx, userID)
		if err != nil {
			t.Fatalf("GetPushReachability: %v", err)
		}
		// Distinct from never-subscribed: this is a storage-invalidation
		// problem, not a UX one, and they have different fixes.
		if reach.Reason(true) != ReachabilitySubscriptionExpired {
			t.Fatalf("expected subscription-expired, got %q", reach.Reason(true))
		}
		if reach.Reachable(true) {
			t.Fatal("an expired subscription is not a delivery path")
		}
	})
}

func TestGetPushReachabilityReportsNotConfiguredWhateverIsStored(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	userID := createPushTestUser(t, st, cleaner, "reach-unconfigured")
	endpoint := "https://push.example.com/" + uuid.NewString()
	if _, err := st.SavePushSubscription(ctx, SavePushSubscriptionParams{
		UserID: userID, Endpoint: endpoint,
		P256dh: "k", Auth: "a",
	}); err != nil {
		t.Fatalf("SavePushSubscription: %v", err)
	}
	verifyTestSubscription(t, st, userID, endpoint)

	reach, err := st.GetPushReachability(ctx, userID)
	if err != nil {
		t.Fatalf("GetPushReachability: %v", err)
	}

	// This is the trap the mandatory parameter exists to prevent: a live
	// subscription on a deployment with no VAPID keys would otherwise read as
	// reachable, putting an absent player in the long hold tier when nothing
	// will ever tell them to come back.
	if reach.Reachable(false) {
		t.Fatal("stored subscriptions on a deployment that cannot send are not reachability")
	}
	if reach.Reason(false) != ReachabilityPushNotConfigured {
		t.Fatalf("expected push-not-configured, got %q", reach.Reason(false))
	}
	// ...and it must still be reachable once keys are configured.
	if !reach.Reachable(true) {
		t.Fatal("the same record should read as reachable on a configured deployment")
	}
}

func TestGetPushReachabilityKeepsALiveInstallWhenAnotherExpires(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	userID := createPushTestUser(t, st, cleaner, "reach-mixed")
	deadEndpoint := "https://push.example.com/" + uuid.NewString()

	for _, endpoint := range []string{deadEndpoint, "https://push.example.com/" + uuid.NewString()} {
		if _, err := st.SavePushSubscription(ctx, SavePushSubscriptionParams{
			UserID: userID, Endpoint: endpoint, P256dh: "k", Auth: "a",
		}); err != nil {
			t.Fatalf("SavePushSubscription: %v", err)
		}
		verifyTestSubscription(t, st, userID, endpoint)
	}
	if err := st.MarkPushSubscriptionExpired(ctx, deadEndpoint); err != nil {
		t.Fatalf("MarkPushSubscriptionExpired: %v", err)
	}

	reach, err := st.GetPushReachability(ctx, userID)
	if err != nil {
		t.Fatalf("GetPushReachability: %v", err)
	}

	// A player whose old phone died but whose desktop still works is reachable.
	if !reach.Reachable(true) {
		t.Fatal("one dead install must not make a player with a live one unreachable")
	}
	if len(reach.Live) != 1 || reach.ExpiredCount != 1 {
		t.Fatalf("expected 1 live and 1 expired, got %d live / %d expired", len(reach.Live), reach.ExpiredCount)
	}
}

func TestGetPushReachabilityReportsWhetherDeliveryHasEverSucceeded(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	userID := createPushTestUser(t, st, cleaner, "reach-delivered")
	endpoint := "https://push.example.com/" + uuid.NewString()
	if _, err := st.SavePushSubscription(ctx, SavePushSubscriptionParams{
		UserID: userID, Endpoint: endpoint, P256dh: "k", Auth: "a",
	}); err != nil {
		t.Fatalf("SavePushSubscription: %v", err)
	}

	reach, err := st.GetPushReachability(ctx, userID)
	if err != nil {
		t.Fatalf("GetPushReachability: %v", err)
	}
	// A subscription that has never delivered is weaker evidence than one that
	// has -- worth having when the hold values get tuned.
	if reach.LastDeliveredAt != nil {
		t.Fatal("a subscription that has never been sent to has no delivery time")
	}

	if err := st.TouchPushSubscription(ctx, endpoint); err != nil {
		t.Fatalf("TouchPushSubscription: %v", err)
	}
	reach, err = st.GetPushReachability(ctx, userID)
	if err != nil {
		t.Fatalf("GetPushReachability: %v", err)
	}
	if reach.LastDeliveredAt == nil {
		t.Fatal("a successful delivery should be visible on the record")
	}
}

// The contract JQ-199 depends on: the hold starts at its floor and this runs
// alongside, so a result arriving late is discarded. It must therefore be safe
// to ignore -- no writes, nothing fired.
func TestGetPushReachabilityHasNoSideEffects(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	userID := createPushTestUser(t, st, cleaner, "reach-pure")
	endpoint := "https://push.example.com/" + uuid.NewString()
	saved, err := st.SavePushSubscription(ctx, SavePushSubscriptionParams{
		UserID: userID, Endpoint: endpoint, P256dh: "k", Auth: "a",
	})
	if err != nil {
		t.Fatalf("SavePushSubscription: %v", err)
	}

	for i := 0; i < 3; i++ {
		if _, err := st.GetPushReachability(ctx, userID); err != nil {
			t.Fatalf("GetPushReachability: %v", err)
		}
	}

	after, err := st.ListPushSubscriptions(ctx, userID)
	if err != nil {
		t.Fatalf("ListPushSubscriptions: %v", err)
	}
	if len(after) != 1 {
		t.Fatalf("reading reachability changed the stored rows: %d", len(after))
	}
	if after[0].ID != saved.ID || after[0].LastUsedAt != nil || after[0].ExpiredAt != nil {
		t.Fatalf("reading reachability mutated the subscription: %+v", after[0])
	}
}

func TestPushReachabilityReasonHandlesANilRecord(t *testing.T) {
	var reach *PushReachability
	// A nil record must not panic and must not read as reachable -- the
	// conservative answer is the safe one for a seat hold.
	if reach.Reachable(true) {
		t.Fatal("a nil reachability record is not reachable")
	}
	if reach.Reason(true) != ReachabilityNeverSubscribed {
		t.Fatalf("expected never-subscribed for a nil record, got %q", reach.Reason(true))
	}
	if reach.Reason(false) != ReachabilityPushNotConfigured {
		t.Fatalf("an unconfigured deployment outranks the record, got %q", reach.Reason(false))
	}
}
