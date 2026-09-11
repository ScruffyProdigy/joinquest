package store

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// saveTestSubscription registers one install and returns its endpoint.
func saveTestSubscription(t *testing.T, st *Store, userID uuid.UUID, keys ...string) string {
	t.Helper()
	p256dh, auth := "test-p256dh-key", "test-auth-secret"
	if len(keys) == 2 {
		p256dh, auth = keys[0], keys[1]
	}
	endpoint := "https://push.example.com/" + uuid.NewString()
	if _, err := st.SavePushSubscription(context.Background(), SavePushSubscriptionParams{
		UserID:   userID,
		Endpoint: endpoint,
		P256dh:   p256dh,
		Auth:     auth,
	}); err != nil {
		t.Fatalf("SavePushSubscription: %v", err)
	}
	return endpoint
}

// The distinction the whole feature rests on: a browser handing over an
// endpoint is a claim, and holding a queue place on a claim strands the player
// it was held for.
func TestSavingASubscriptionDoesNotMakeItVerified(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	userID := createPushTestUser(t, st, cleaner, "verify-save")
	saveTestSubscription(t, st, userID)

	verified, err := st.HasVerifiedPushSubscription(ctx, userID)
	if err != nil {
		t.Fatalf("HasVerifiedPushSubscription: %v", err)
	}
	if verified {
		t.Fatal("a registration on its own must not count as verified")
	}

	reach, err := st.GetPushReachability(ctx, userID)
	if err != nil {
		t.Fatalf("GetPushReachability: %v", err)
	}
	if reach.Reason(true) != ReachabilityUnverified {
		t.Fatalf("expected unverified, got %q", reach.Reason(true))
	}
	if reach.Reachable(true) {
		t.Fatal("an unverified install must not read as reachable")
	}
}

func TestConfirmingAVerificationTokenMakesTheUserReachable(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	userID := createPushTestUser(t, st, cleaner, "verify-confirm")
	endpoint := saveTestSubscription(t, st, userID)

	sub, token, err := st.StartPushSubscriptionVerification(ctx, userID, endpoint)
	if err != nil {
		t.Fatalf("StartPushSubscriptionVerification: %v", err)
	}
	if sub.P256dh == "" || sub.Auth == "" {
		t.Fatal("the keys are needed to encrypt the verification push")
	}
	if token == "" {
		t.Fatal("a verification without a token cannot be acked")
	}

	confirmedFor, err := st.ConfirmPushSubscriptionVerification(ctx, token)
	if err != nil {
		t.Fatalf("ConfirmPushSubscriptionVerification: %v", err)
	}
	if confirmedFor != userID {
		t.Fatalf("token redeemed for the wrong user: %s", confirmedFor)
	}

	reach, err := st.GetPushReachability(ctx, userID)
	if err != nil {
		t.Fatalf("GetPushReachability: %v", err)
	}
	if reach.Reason(true) != ReachabilityReachable {
		t.Fatalf("expected reachable after an ack, got %q", reach.Reason(true))
	}
}

// Replay is the cheapest way to fake reachability, so the token has to die on
// first use.
func TestAVerificationTokenIsSingleUse(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	userID := createPushTestUser(t, st, cleaner, "verify-replay")
	endpoint := saveTestSubscription(t, st, userID)

	_, token, err := st.StartPushSubscriptionVerification(ctx, userID, endpoint)
	if err != nil {
		t.Fatalf("StartPushSubscriptionVerification: %v", err)
	}
	if _, err := st.ConfirmPushSubscriptionVerification(ctx, token); err != nil {
		t.Fatalf("first ack: %v", err)
	}

	_, err = st.ConfirmPushSubscriptionVerification(ctx, token)
	if !errors.Is(err, ErrPushVerificationNotFound) {
		t.Fatalf("expected a replayed token to be refused, got %v", err)
	}
}

func TestAnUnknownVerificationTokenProvesNothing(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	_, err := st.ConfirmPushSubscriptionVerification(ctx, "not-a-real-token")
	if !errors.Is(err, ErrPushVerificationNotFound) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

// Starting a fresh round trip retracts the old proof. Otherwise a verification
// that never completes would leave the player looking reachable on the
// strength of a previous one.
func TestStartingVerificationClearsTheExistingProof(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	userID := createPushTestUser(t, st, cleaner, "verify-restart")
	endpoint := saveTestSubscription(t, st, userID)

	_, token, err := st.StartPushSubscriptionVerification(ctx, userID, endpoint)
	if err != nil {
		t.Fatalf("StartPushSubscriptionVerification: %v", err)
	}
	if _, err := st.ConfirmPushSubscriptionVerification(ctx, token); err != nil {
		t.Fatalf("ack: %v", err)
	}

	if _, _, err := st.StartPushSubscriptionVerification(ctx, userID, endpoint); err != nil {
		t.Fatalf("second StartPushSubscriptionVerification: %v", err)
	}

	verified, err := st.HasVerifiedPushSubscription(ctx, userID)
	if err != nil {
		t.Fatalf("HasVerifiedPushSubscription: %v", err)
	}
	if verified {
		t.Fatal("a verification in flight must not leave the old proof standing")
	}
}

// A proof is about a set of keys, not an endpoint string.
func TestRotatedKeysHaveToEarnVerificationAgain(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	userID := createPushTestUser(t, st, cleaner, "verify-rotate")
	endpoint := saveTestSubscription(t, st, userID)

	_, token, err := st.StartPushSubscriptionVerification(ctx, userID, endpoint)
	if err != nil {
		t.Fatalf("StartPushSubscriptionVerification: %v", err)
	}
	if _, err := st.ConfirmPushSubscriptionVerification(ctx, token); err != nil {
		t.Fatalf("ack: %v", err)
	}

	// Same endpoint, new keys -- the push service rotated it.
	if _, err := st.SavePushSubscription(ctx, SavePushSubscriptionParams{
		UserID:   userID,
		Endpoint: endpoint,
		P256dh:   "rotated-p256dh-key",
		Auth:     "rotated-auth-secret",
	}); err != nil {
		t.Fatalf("re-save: %v", err)
	}

	verified, err := st.HasVerifiedPushSubscription(ctx, userID)
	if err != nil {
		t.Fatalf("HasVerifiedPushSubscription: %v", err)
	}
	if verified {
		t.Fatal("rotated keys must not inherit the old proof")
	}
}

// A player who opted in, left, and came back must not be asked twice: the
// browser re-saves the same subscription on every visit.
func TestAnUnchangedReSaveKeepsVerification(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	userID := createPushTestUser(t, st, cleaner, "verify-resave")
	endpoint := saveTestSubscription(t, st, userID)

	_, token, err := st.StartPushSubscriptionVerification(ctx, userID, endpoint)
	if err != nil {
		t.Fatalf("StartPushSubscriptionVerification: %v", err)
	}
	if _, err := st.ConfirmPushSubscriptionVerification(ctx, token); err != nil {
		t.Fatalf("ack: %v", err)
	}

	if _, err := st.SavePushSubscription(ctx, SavePushSubscriptionParams{
		UserID:   userID,
		Endpoint: endpoint,
		P256dh:   "test-p256dh-key",
		Auth:     "test-auth-secret",
	}); err != nil {
		t.Fatalf("re-save: %v", err)
	}

	verified, err := st.HasVerifiedPushSubscription(ctx, userID)
	if err != nil {
		t.Fatalf("HasVerifiedPushSubscription: %v", err)
	}
	if !verified {
		t.Fatal("an unchanged re-save must keep the proof it already earned")
	}
}

// Endpoints are unique across users, so a signed-in browser can carry one to a
// new account. The old owner's proof says nothing about the new one.
func TestANewOwnerHasToEarnVerificationAgain(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	first := createPushTestUser(t, st, cleaner, "verify-owner-a")
	second := createPushTestUser(t, st, cleaner, "verify-owner-b")
	endpoint := saveTestSubscription(t, st, first)

	_, token, err := st.StartPushSubscriptionVerification(ctx, first, endpoint)
	if err != nil {
		t.Fatalf("StartPushSubscriptionVerification: %v", err)
	}
	if _, err := st.ConfirmPushSubscriptionVerification(ctx, token); err != nil {
		t.Fatalf("ack: %v", err)
	}

	if _, err := st.SavePushSubscription(ctx, SavePushSubscriptionParams{
		UserID:   second,
		Endpoint: endpoint,
		P256dh:   "test-p256dh-key",
		Auth:     "test-auth-secret",
	}); err != nil {
		t.Fatalf("re-save under a new owner: %v", err)
	}

	verified, err := st.HasVerifiedPushSubscription(ctx, second)
	if err != nil {
		t.Fatalf("HasVerifiedPushSubscription: %v", err)
	}
	if verified {
		t.Fatal("a new owner must not inherit the previous one's proof")
	}
}

// User-scoped, so one account cannot start verification on another's endpoint
// and learn from the answer whether it exists.
func TestVerificationCannotBeStartedForAnotherPlayersEndpoint(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	owner := createPushTestUser(t, st, cleaner, "verify-scope-owner")
	stranger := createPushTestUser(t, st, cleaner, "verify-scope-other")
	endpoint := saveTestSubscription(t, st, owner)

	_, _, err := st.StartPushSubscriptionVerification(ctx, stranger, endpoint)
	if !errors.Is(err, ErrPushVerificationNotFound) {
		t.Fatalf("expected not-found for another player's endpoint, got %v", err)
	}
}

// An endpoint the push service has rejected is not a candidate: an ack could
// never arrive.
func TestAnExpiredSubscriptionCannotBeVerified(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	userID := createPushTestUser(t, st, cleaner, "verify-expired")
	endpoint := saveTestSubscription(t, st, userID)
	if err := st.MarkPushSubscriptionExpired(ctx, endpoint); err != nil {
		t.Fatalf("MarkPushSubscriptionExpired: %v", err)
	}

	_, _, err := st.StartPushSubscriptionVerification(ctx, userID, endpoint)
	if !errors.Is(err, ErrPushVerificationNotFound) {
		t.Fatalf("expected not-found for an expired endpoint, got %v", err)
	}
}
