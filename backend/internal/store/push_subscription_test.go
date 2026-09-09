package store

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func createPushTestUser(t *testing.T, st *Store, cleaner *TestCleaner, label string) uuid.UUID {
	t.Helper()
	user, err := st.CreateUser(context.Background(), CreateUserParams{
		Email: label + "-" + uuid.NewString() + "@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(user.ID)
	return user.ID
}

func TestSavePushSubscriptionStoresAnInstallAndMakesTheUserReachable(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	userID := createPushTestUser(t, st, cleaner, "push-save")

	reachable, err := st.HasPushSubscription(ctx, userID)
	if err != nil {
		t.Fatalf("HasPushSubscription: %v", err)
	}
	if reachable {
		t.Fatal("a user with no subscription must not be reported as reachable")
	}

	endpoint := "https://push.example.com/" + uuid.NewString()
	saved, err := st.SavePushSubscription(ctx, SavePushSubscriptionParams{
		UserID:    userID,
		Endpoint:  endpoint,
		P256dh:    "test-p256dh-key",
		Auth:      "test-auth-secret",
		UserAgent: "Mozilla/5.0 (iPhone)",
	})
	if err != nil {
		t.Fatalf("SavePushSubscription: %v", err)
	}
	if saved.UserID != userID || saved.Endpoint != endpoint {
		t.Fatalf("saved subscription does not match input: %+v", saved)
	}
	if saved.LastUsedAt != nil {
		t.Fatal("a freshly registered subscription has not been used yet")
	}

	reachable, err = st.HasPushSubscription(ctx, userID)
	if err != nil {
		t.Fatalf("HasPushSubscription after save: %v", err)
	}
	if !reachable {
		t.Fatal("a user with a stored subscription must be reported as reachable")
	}
}

func TestSavePushSubscriptionRejectsAnIncompleteSubscription(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	userID := createPushTestUser(t, st, cleaner, "push-invalid")

	cases := map[string]SavePushSubscriptionParams{
		"missing endpoint": {UserID: userID, P256dh: "k", Auth: "a"},
		"missing p256dh":   {UserID: userID, Endpoint: "https://push.example.com/x", Auth: "a"},
		"missing auth":     {UserID: userID, Endpoint: "https://push.example.com/x", P256dh: "k"},
		"blank endpoint":   {UserID: userID, Endpoint: "   ", P256dh: "k", Auth: "a"},
	}

	for name, params := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := st.SavePushSubscription(ctx, params); !errors.Is(err, ErrInvalidPushSubscription) {
				t.Fatalf("expected ErrInvalidPushSubscription, got %v", err)
			}
		})
	}
}

func TestSavePushSubscriptionUpdatesInPlaceWhenTheSameBrowserResubscribes(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	userID := createPushTestUser(t, st, cleaner, "push-resub")
	endpoint := "https://push.example.com/" + uuid.NewString()

	first, err := st.SavePushSubscription(ctx, SavePushSubscriptionParams{
		UserID: userID, Endpoint: endpoint, P256dh: "key-one", Auth: "auth-one",
	})
	if err != nil {
		t.Fatalf("SavePushSubscription (first): %v", err)
	}

	second, err := st.SavePushSubscription(ctx, SavePushSubscriptionParams{
		UserID: userID, Endpoint: endpoint, P256dh: "key-two", Auth: "auth-two",
	})
	if err != nil {
		t.Fatalf("SavePushSubscription (second): %v", err)
	}

	if first.ID != second.ID {
		t.Fatal("resubscribing in the same browser must update the row, not create a second one")
	}
	if second.P256dh != "key-two" || second.Auth != "auth-two" {
		t.Fatalf("rotated keys were not persisted: %+v", second)
	}

	subs, err := st.ListPushSubscriptions(ctx, userID)
	if err != nil {
		t.Fatalf("ListPushSubscriptions: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected exactly one subscription, got %d", len(subs))
	}
}

func TestSavePushSubscriptionMovesAnEndpointToTheNewOwner(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	firstOwner := createPushTestUser(t, st, cleaner, "push-owner-a")
	secondOwner := createPushTestUser(t, st, cleaner, "push-owner-b")
	endpoint := "https://push.example.com/" + uuid.NewString()

	if _, err := st.SavePushSubscription(ctx, SavePushSubscriptionParams{
		UserID: firstOwner, Endpoint: endpoint, P256dh: "k", Auth: "a",
	}); err != nil {
		t.Fatalf("SavePushSubscription (first owner): %v", err)
	}
	if _, err := st.SavePushSubscription(ctx, SavePushSubscriptionParams{
		UserID: secondOwner, Endpoint: endpoint, P256dh: "k", Auth: "a",
	}); err != nil {
		t.Fatalf("SavePushSubscription (second owner): %v", err)
	}

	stillReachable, err := st.HasPushSubscription(ctx, firstOwner)
	if err != nil {
		t.Fatalf("HasPushSubscription (first owner): %v", err)
	}
	if stillReachable {
		t.Fatal("the previous owner of a browser must not keep receiving its notifications")
	}

	reachable, err := st.HasPushSubscription(ctx, secondOwner)
	if err != nil {
		t.Fatalf("HasPushSubscription (second owner): %v", err)
	}
	if !reachable {
		t.Fatal("the new owner of the browser must be reachable")
	}
}

func TestListPushSubscriptionsReturnsEveryInstallForTheUserOnly(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	userID := createPushTestUser(t, st, cleaner, "push-list")
	otherID := createPushTestUser(t, st, cleaner, "push-list-other")

	for _, label := range []string{"phone", "desktop"} {
		if _, err := st.SavePushSubscription(ctx, SavePushSubscriptionParams{
			UserID:    userID,
			Endpoint:  "https://push.example.com/" + label + "-" + uuid.NewString(),
			P256dh:    "k",
			Auth:      "a",
			UserAgent: label,
		}); err != nil {
			t.Fatalf("SavePushSubscription (%s): %v", label, err)
		}
	}
	if _, err := st.SavePushSubscription(ctx, SavePushSubscriptionParams{
		UserID: otherID, Endpoint: "https://push.example.com/" + uuid.NewString(), P256dh: "k", Auth: "a",
	}); err != nil {
		t.Fatalf("SavePushSubscription (other user): %v", err)
	}

	subs, err := st.ListPushSubscriptions(ctx, userID)
	if err != nil {
		t.Fatalf("ListPushSubscriptions: %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("expected two subscriptions for the user, got %d", len(subs))
	}
	for _, sub := range subs {
		if sub.UserID != userID {
			t.Fatalf("another user's subscription leaked into the list: %+v", sub)
		}
	}
}

func TestDeletePushSubscriptionOnlyRemovesTheCallersOwnInstall(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	userID := createPushTestUser(t, st, cleaner, "push-delete")
	otherID := createPushTestUser(t, st, cleaner, "push-delete-other")

	othersEndpoint := "https://push.example.com/" + uuid.NewString()
	if _, err := st.SavePushSubscription(ctx, SavePushSubscriptionParams{
		UserID: otherID, Endpoint: othersEndpoint, P256dh: "k", Auth: "a",
	}); err != nil {
		t.Fatalf("SavePushSubscription (other user): %v", err)
	}

	if err := st.DeletePushSubscription(ctx, userID, othersEndpoint); err != nil {
		t.Fatalf("DeletePushSubscription: %v", err)
	}

	stillReachable, err := st.HasPushSubscription(ctx, otherID)
	if err != nil {
		t.Fatalf("HasPushSubscription: %v", err)
	}
	if !stillReachable {
		t.Fatal("one user must not be able to unsubscribe another user's device by endpoint")
	}
}

func TestDeleteExpiredPushSubscriptionRemovesADeadEndpointRegardlessOfOwner(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	userID := createPushTestUser(t, st, cleaner, "push-expire")
	endpoint := "https://push.example.com/" + uuid.NewString()

	if _, err := st.SavePushSubscription(ctx, SavePushSubscriptionParams{
		UserID: userID, Endpoint: endpoint, P256dh: "k", Auth: "a",
	}); err != nil {
		t.Fatalf("SavePushSubscription: %v", err)
	}

	if err := st.DeleteExpiredPushSubscription(ctx, endpoint); err != nil {
		t.Fatalf("DeleteExpiredPushSubscription: %v", err)
	}

	reachable, err := st.HasPushSubscription(ctx, userID)
	if err != nil {
		t.Fatalf("HasPushSubscription: %v", err)
	}
	if reachable {
		t.Fatal("a user whose only endpoint the push service rejected is no longer reachable")
	}
}

func TestDeletePushSubscriptionsForUserClearsEveryInstallOnLogout(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	userID := createPushTestUser(t, st, cleaner, "push-logout")
	for i := 0; i < 3; i++ {
		if _, err := st.SavePushSubscription(ctx, SavePushSubscriptionParams{
			UserID: userID, Endpoint: "https://push.example.com/" + uuid.NewString(), P256dh: "k", Auth: "a",
		}); err != nil {
			t.Fatalf("SavePushSubscription: %v", err)
		}
	}

	if err := st.DeletePushSubscriptionsForUser(ctx, userID); err != nil {
		t.Fatalf("DeletePushSubscriptionsForUser: %v", err)
	}

	subs, err := st.ListPushSubscriptions(ctx, userID)
	if err != nil {
		t.Fatalf("ListPushSubscriptions: %v", err)
	}
	if len(subs) != 0 {
		t.Fatalf("expected no subscriptions after logout, got %d", len(subs))
	}
}

func TestTouchPushSubscriptionRecordsThatTheInstallStillWorks(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	userID := createPushTestUser(t, st, cleaner, "push-touch")
	endpoint := "https://push.example.com/" + uuid.NewString()

	if _, err := st.SavePushSubscription(ctx, SavePushSubscriptionParams{
		UserID: userID, Endpoint: endpoint, P256dh: "k", Auth: "a",
	}); err != nil {
		t.Fatalf("SavePushSubscription: %v", err)
	}
	if err := st.TouchPushSubscription(ctx, endpoint); err != nil {
		t.Fatalf("TouchPushSubscription: %v", err)
	}

	subs, err := st.ListPushSubscriptions(ctx, userID)
	if err != nil {
		t.Fatalf("ListPushSubscriptions: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected one subscription, got %d", len(subs))
	}
	if subs[0].LastUsedAt == nil {
		t.Fatal("a delivered notification must be recorded on the subscription")
	}
}

func TestDeletingAUserCascadesToTheirPushSubscriptions(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	userID := createPushTestUser(t, st, cleaner, "push-cascade")
	endpoint := "https://push.example.com/" + uuid.NewString()
	if _, err := st.SavePushSubscription(ctx, SavePushSubscriptionParams{
		UserID: userID, Endpoint: endpoint, P256dh: "k", Auth: "a",
	}); err != nil {
		t.Fatalf("SavePushSubscription: %v", err)
	}

	if _, err := st.db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	var count int
	if err := st.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM push_subscriptions WHERE endpoint = $1`, endpoint,
	).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatal("push subscriptions must not outlive the user they belong to")
	}
}
