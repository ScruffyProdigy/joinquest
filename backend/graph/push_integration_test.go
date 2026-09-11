package graph

import (
	"context"
	"testing"
	"time"

	"github.com/99designs/gqlgen/client"
	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/push"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

// pushCapabilityResponse mirrors the PushCapability selection set used below.
type pushCapabilityResponse struct {
	Reachable         bool
	SubscriptionCount int
	VerifiedCount     int
	PublicKey         *string
}

const pushCapabilityFields = `reachable subscriptionCount verifiedCount publicKey`

// configurePush gives the resolver a real VAPID public key so capability
// reflects a deployment that can actually send.
func configurePush(t *testing.T, env *queueIntegrationEnv) string {
	t.Helper()
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatalf("GenerateVAPIDKeys: %v", err)
	}
	env.resolver.Push = push.NewWebPushSender(push.Config{
		PublicKey: pub, PrivateKey: priv, Subject: "mailto:test@example.com",
	})
	env.rebuildHTTPServer(t)
	return pub
}

func testEndpoint() string {
	return "https://push.example.com/" + uuid.NewString()
}

func TestSavePushSubscriptionStoresTheInstallWithoutClaimingReachability(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	ctx := context.Background()
	cleaner := env.newCleaner(t)
	publicKey := configurePush(t, env)
	bearer, _ := createTestUserSession(t, ctx, env, cleaner)

	var before struct {
		PushCapability pushCapabilityResponse
	}
	env.Client.MustPost(
		`query { pushCapability { `+pushCapabilityFields+` } }`, &before,
		client.AddHeader("Authorization", bearer),
	)
	if before.PushCapability.Reachable {
		t.Fatal("a player who has not subscribed is not reachable")
	}
	if before.PushCapability.PublicKey == nil || *before.PushCapability.PublicKey != publicKey {
		t.Fatal("a configured deployment must hand the browser its VAPID public key")
	}

	var saved struct {
		SavePushSubscription pushCapabilityResponse
	}
	env.Client.MustPost(
		`mutation ($e: String!) {
			savePushSubscription(input: {endpoint: $e, p256dh: "test-key", auth: "test-auth"}) { `+pushCapabilityFields+` }
		}`, &saved,
		client.Var("e", testEndpoint()),
		client.AddHeader("Authorization", bearer),
	)

	// Stored, and still not reachable. The browser has handed over an endpoint
	// and nothing has come back from it, which is a claim rather than a
	// capability -- see verifyPushSubscription.
	if saved.SavePushSubscription.Reachable {
		t.Fatal("a saved-but-unverified subscription must not read as reachable")
	}
	if saved.SavePushSubscription.SubscriptionCount != 1 {
		t.Fatalf("expected one subscription, got %d", saved.SavePushSubscription.SubscriptionCount)
	}
	if saved.SavePushSubscription.VerifiedCount != 0 {
		t.Fatalf("expected no verified subscriptions, got %d", saved.SavePushSubscription.VerifiedCount)
	}
}

func TestPushCapabilityReportsUnreachableWhenTheDeploymentCannotSend(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	ctx := context.Background()
	cleaner := env.newCleaner(t)

	// No VAPID keys: the local-dev default.
	env.resolver.Push = push.LogSender{}
	env.rebuildHTTPServer(t)
	bearer, _ := createTestUserSession(t, ctx, env, cleaner)

	var saved struct {
		SavePushSubscription pushCapabilityResponse
	}
	env.Client.MustPost(
		`mutation ($e: String!) {
			savePushSubscription(input: {endpoint: $e, p256dh: "k", auth: "a"}) { `+pushCapabilityFields+` }
		}`, &saved,
		client.Var("e", testEndpoint()),
		client.AddHeader("Authorization", bearer),
	)

	// The subscription is stored, but nothing can send to it. Claiming
	// reachability here is exactly the false promise the ticket warns against.
	if saved.SavePushSubscription.Reachable {
		t.Fatal("a stored subscription on a deployment with no VAPID keys is not reachability")
	}
	if saved.SavePushSubscription.PublicKey != nil {
		t.Fatal("an unconfigured deployment must report no public key so the UI hides the affordance")
	}
	if saved.SavePushSubscription.SubscriptionCount != 1 {
		t.Fatalf("the subscription should still be recorded, got count %d", saved.SavePushSubscription.SubscriptionCount)
	}
}

func TestSavePushSubscriptionIsIdempotentForTheSameBrowser(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	ctx := context.Background()
	cleaner := env.newCleaner(t)
	configurePush(t, env)
	bearer, _ := createTestUserSession(t, ctx, env, cleaner)

	endpoint := testEndpoint()
	mutation := `mutation ($e: String!) {
		savePushSubscription(input: {endpoint: $e, p256dh: "k", auth: "a"}) { ` + pushCapabilityFields + ` }
	}`

	var latest struct {
		SavePushSubscription pushCapabilityResponse
	}
	for i := 0; i < 3; i++ {
		env.Client.MustPost(mutation, &latest,
			client.Var("e", endpoint),
			client.AddHeader("Authorization", bearer),
		)
	}

	if latest.SavePushSubscription.SubscriptionCount != 1 {
		t.Fatalf("re-subscribing the same browser must not accumulate rows, got %d",
			latest.SavePushSubscription.SubscriptionCount)
	}
}

func TestDeletePushSubscriptionMakesThePlayerUnreachableAgain(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	ctx := context.Background()
	cleaner := env.newCleaner(t)
	configurePush(t, env)
	bearer, _ := createTestUserSession(t, ctx, env, cleaner)

	endpoint := testEndpoint()
	var saved struct {
		SavePushSubscription pushCapabilityResponse
	}
	env.Client.MustPost(
		`mutation ($e: String!) {
			savePushSubscription(input: {endpoint: $e, p256dh: "k", auth: "a"}) { `+pushCapabilityFields+` }
		}`, &saved,
		client.Var("e", endpoint),
		client.AddHeader("Authorization", bearer),
	)

	var deleted struct {
		DeletePushSubscription pushCapabilityResponse
	}
	env.Client.MustPost(
		`mutation ($e: String!) { deletePushSubscription(endpoint: $e) { `+pushCapabilityFields+` } }`,
		&deleted,
		client.Var("e", endpoint),
		client.AddHeader("Authorization", bearer),
	)

	if deleted.DeletePushSubscription.Reachable {
		t.Fatal("a player who turned notifications off must not be reported reachable")
	}
	if deleted.DeletePushSubscription.SubscriptionCount != 0 {
		t.Fatalf("expected no subscriptions, got %d", deleted.DeletePushSubscription.SubscriptionCount)
	}
}

func TestPushSubscriptionRequiresASessionButNotAChosenIdentity(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	ctx := context.Background()
	cleaner := env.newCleaner(t)
	configurePush(t, env)

	// Signed out: capability is simply unavailable, not an error.
	var anonymous struct {
		PushCapability pushCapabilityResponse
	}
	env.Client.MustPost(`query { pushCapability { `+pushCapabilityFields+` } }`, &anonymous)
	if anonymous.PushCapability.Reachable {
		t.Fatal("a signed-out visitor is not reachable")
	}

	// Registering must be rejected without a session.
	err := env.Client.Post(
		`mutation ($e: String!) {
			savePushSubscription(input: {endpoint: $e, p256dh: "k", auth: "a"}) { reachable }
		}`,
		&struct {
			SavePushSubscription pushCapabilityResponse
		}{},
		client.Var("e", testEndpoint()),
	)
	if err == nil {
		t.Fatal("registering a push subscription requires a session")
	}

	// A guest with a session but no name or avatar must still be able to
	// register: being told your match is ready is not entering play.
	user, createErr := env.Store.CreateUser(ctx, store.CreateUserParams{
		Email: "push-guest-" + uuid.NewString() + "@example.com",
	})
	if createErr != nil {
		t.Fatalf("CreateUser: %v", createErr)
	}
	cleaner.TrackUser(user.ID)
	token, signErr := env.Signer.SignUserToken(user.ID, time.Hour)
	if signErr != nil {
		t.Fatalf("SignUserToken: %v", signErr)
	}

	var guest struct {
		SavePushSubscription pushCapabilityResponse
	}
	env.Client.MustPost(
		`mutation ($e: String!) {
			savePushSubscription(input: {endpoint: $e, p256dh: "k", auth: "a"}) { `+pushCapabilityFields+` }
		}`, &guest,
		client.Var("e", testEndpoint()),
		client.AddHeader("Authorization", "Bearer "+token),
	)
	if guest.SavePushSubscription.SubscriptionCount != 1 {
		t.Fatal("a guest who has not chosen an identity must still be able to register for notifications")
	}
}

func TestLogoutClearsThePlayersPushSubscriptions(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	ctx := context.Background()
	cleaner := env.newCleaner(t)
	configurePush(t, env)
	bearer, _ := createTestUserSession(t, ctx, env, cleaner)

	var saved struct {
		SavePushSubscription pushCapabilityResponse
	}
	env.Client.MustPost(
		`mutation ($e: String!) {
			savePushSubscription(input: {endpoint: $e, p256dh: "k", auth: "a"}) { `+pushCapabilityFields+` }
		}`, &saved,
		client.Var("e", testEndpoint()),
		client.AddHeader("Authorization", bearer),
	)
	if saved.SavePushSubscription.SubscriptionCount != 1 {
		t.Fatal("precondition: the player should hold a subscription before logging out")
	}

	var loggedOut struct {
		Logout bool
	}
	env.Client.MustPost(`mutation { logout }`, &loggedOut,
		client.AddHeader("Authorization", bearer),
	)

	// The browser install stays subscribed at the OS level, so leaving the row
	// behind would ring this device for whoever signs in next.
	var after struct {
		PushCapability pushCapabilityResponse
	}
	env.Client.MustPost(`query { pushCapability { `+pushCapabilityFields+` } }`, &after,
		client.AddHeader("Authorization", bearer),
	)
	if after.PushCapability.SubscriptionCount != 0 {
		t.Fatalf("logout must clear push subscriptions, %d remain", after.PushCapability.SubscriptionCount)
	}
}

// pushCapability (what the UI gates on) and PushReachability (what JQ-199's
// seat hold branches on) answer the same question for two audiences. If they
// ever disagree, one of them is lying to somebody -- and the seat-hold tiers
// are only sound if the answer is honest.
func TestPushCapabilityAndSeatHoldReachabilityAgree(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	ctx := context.Background()
	cleaner := env.newCleaner(t)
	configurePush(t, env)
	bearer, _ := createTestUserSession(t, ctx, env, cleaner)

	userID := currentUserID(t, ctx, env, bearer)

	assertAgreement := func(t *testing.T, stage string) {
		t.Helper()
		var q struct {
			PushCapability pushCapabilityResponse
		}
		env.Client.MustPost(`query { pushCapability { `+pushCapabilityFields+` } }`, &q,
			client.AddHeader("Authorization", bearer),
		)
		reachable, err := env.resolver.PushReachable(ctx, userID)
		if err != nil {
			t.Fatalf("PushReachable (%s): %v", stage, err)
		}
		if q.PushCapability.Reachable != reachable {
			t.Fatalf("%s: pushCapability says reachable=%v but the seat-hold predicate says %v",
				stage, q.PushCapability.Reachable, reachable)
		}
	}

	assertAgreement(t, "before subscribing")

	endpoint := testEndpoint()
	var saved struct {
		SavePushSubscription pushCapabilityResponse
	}
	env.Client.MustPost(
		`mutation ($e: String!) {
			savePushSubscription(input: {endpoint: $e, p256dh: "k", auth: "a"}) { `+pushCapabilityFields+` }
		}`, &saved,
		client.Var("e", endpoint),
		client.AddHeader("Authorization", bearer),
	)
	assertAgreement(t, "after subscribing")

	// The push service rejects the endpoint: both views must drop together.
	if err := env.Store.MarkPushSubscriptionExpired(ctx, endpoint); err != nil {
		t.Fatalf("MarkPushSubscriptionExpired: %v", err)
	}
	assertAgreement(t, "after the push service rejected the endpoint")

	reach, reason, err := env.resolver.PushReachability(ctx, userID)
	if err != nil {
		t.Fatalf("PushReachability: %v", err)
	}
	if reason != store.ReachabilitySubscriptionExpired {
		t.Fatalf("expected subscription-expired, got %q", reason)
	}
	if reach.ExpiredCount != 1 {
		t.Fatalf("the expired install should be retained for tuning, got %d", reach.ExpiredCount)
	}
}

// currentUserID resolves the signed-in user's UUID via the API.
func currentUserID(t *testing.T, ctx context.Context, env *queueIntegrationEnv, bearer string) uuid.UUID {
	t.Helper()
	var me struct {
		Me struct{ ID string }
	}
	env.Client.MustPost(`query { me { id } }`, &me, client.AddHeader("Authorization", bearer))
	parsed, err := uuid.Parse(me.Me.ID)
	if err != nil {
		t.Fatalf("parse user id %q: %v", me.Me.ID, err)
	}
	return parsed
}

// configureFakePush swaps in a sender whose outcome the test controls, so the
// verification round trip can be exercised without a real push service.
func configureFakePush(t *testing.T, env *queueIntegrationEnv, results map[string]error) *fakeSender {
	t.Helper()
	sender := &fakeSender{publicKey: "test-vapid-public-key", results: results}
	env.resolver.Push = sender
	env.rebuildHTTPServer(t)
	return sender
}

func TestVerifyPushSubscriptionSendsAPushAndOnlyTheAckMakesThePlayerReachable(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	ctx := context.Background()
	cleaner := env.newCleaner(t)
	sender := configureFakePush(t, env, nil)
	userID := newPushTestUser(t, ctx, env, cleaner, "verify-round-trip")
	bearer, _ := createTestUserSessionForUser(t, env, userID)

	endpoint := testEndpoint()
	var saved struct {
		SavePushSubscription pushCapabilityResponse
	}
	env.Client.MustPost(
		`mutation ($e: String!) {
			savePushSubscription(input: {endpoint: $e, p256dh: "k", auth: "a"}) { `+pushCapabilityFields+` }
		}`, &saved,
		client.Var("e", endpoint),
		client.AddHeader("Authorization", bearer),
	)

	var verify struct {
		VerifyPushSubscription struct {
			Sent       bool
			Capability pushCapabilityResponse
		}
	}
	env.Client.MustPost(
		`mutation ($e: String!) {
			verifyPushSubscription(endpoint: $e) { sent capability { `+pushCapabilityFields+` } }
		}`, &verify,
		client.Var("e", endpoint),
		client.AddHeader("Authorization", bearer),
	)

	if !verify.VerifyPushSubscription.Sent {
		t.Fatal("a configured deployment should have handed the push to the service")
	}
	if len(sender.sent) != 1 || sender.sent[0] != endpoint {
		t.Fatalf("expected one send to %s, got %v", endpoint, sender.sent)
	}
	// Sending is not proof. The answer arrives with the ack, and until then the
	// player must not be told they can leave the page.
	if verify.VerifyPushSubscription.Capability.Reachable {
		t.Fatal("verification in flight is not reachability")
	}

	// Stand in for the service worker, reading the token where a real one
	// does: out of the push that reached it.
	if len(sender.notes) != 1 {
		t.Fatalf("expected one notification, got %d", len(sender.notes))
	}
	note := sender.notes[0]
	if note.Kind != push.KindVerify {
		t.Fatalf("expected a verification push, got kind %q", note.Kind)
	}
	token := note.Token
	if token == "" {
		t.Fatal("a verification push with no token can never be acked")
	}
	var confirmed struct {
		ConfirmPushVerification struct{ Verified bool }
	}
	env.Client.MustPost(
		`mutation ($t: String!) { confirmPushVerification(token: $t) { verified } }`,
		&confirmed,
		client.Var("t", token),
	)
	if !confirmed.ConfirmPushVerification.Verified {
		t.Fatal("a live token must verify the subscription that received it")
	}

	var after struct {
		PushCapability pushCapabilityResponse
	}
	env.Client.MustPost(
		`query { pushCapability { `+pushCapabilityFields+` } }`, &after,
		client.AddHeader("Authorization", bearer),
	)
	if !after.PushCapability.Reachable {
		t.Fatal("an acked subscription makes the player reachable")
	}
	if after.PushCapability.VerifiedCount != 1 {
		t.Fatalf("expected one verified subscription, got %d", after.PushCapability.VerifiedCount)
	}
}

// The ack carries no session on purpose: by design it can arrive from a
// service worker whose page has already gone away.
func TestConfirmPushVerificationNeedsNoSessionButRejectsAnUnknownToken(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	configureFakePush(t, env, nil)

	var result struct {
		ConfirmPushVerification struct{ Verified bool }
	}
	env.Client.MustPost(
		`mutation { confirmPushVerification(token: "not-a-real-token") { verified } }`,
		&result,
	)
	// A normal outcome, not a fault: nothing to tell apart from a real error.
	if result.ConfirmPushVerification.Verified {
		t.Fatal("an unknown token must not verify anything")
	}
}

// A push service that rejects the endpoint settles the question immediately.
// Leaving it looking like an ack still to come would strand the player on a
// spinner that resolves into a false promise.
func TestVerifyPushSubscriptionExpiresAnEndpointThePushServiceRejects(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	ctx := context.Background()
	cleaner := env.newCleaner(t)
	userID := newPushTestUser(t, ctx, env, cleaner, "verify-gone")
	bearer, _ := createTestUserSessionForUser(t, env, userID)

	endpoint := testEndpoint()
	configureFakePush(t, env, map[string]error{endpoint: push.ErrSubscriptionGone})

	if _, err := env.Store.SavePushSubscription(ctx, store.SavePushSubscriptionParams{
		UserID: userID, Endpoint: endpoint, P256dh: "k", Auth: "a",
	}); err != nil {
		t.Fatalf("SavePushSubscription: %v", err)
	}

	var verify struct {
		VerifyPushSubscription struct {
			Sent       bool
			Capability pushCapabilityResponse
		}
	}
	env.Client.MustPost(
		`mutation ($e: String!) {
			verifyPushSubscription(endpoint: $e) { sent capability { `+pushCapabilityFields+` } }
		}`, &verify,
		client.Var("e", endpoint),
		client.AddHeader("Authorization", bearer),
	)

	if verify.VerifyPushSubscription.Sent {
		t.Fatal("a rejected endpoint must not report a send the caller would wait on")
	}
	if verify.VerifyPushSubscription.Capability.SubscriptionCount != 0 {
		t.Fatalf("a rejected endpoint should no longer count as live, got %d",
			verify.VerifyPushSubscription.Capability.SubscriptionCount)
	}
}

// No VAPID keys means nothing was attempted, and the caller must not sit
// waiting for an ack that was never provoked.
func TestVerifyPushSubscriptionReportsNoSendWhenTheDeploymentCannotSend(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	ctx := context.Background()
	cleaner := env.newCleaner(t)
	env.resolver.Push = push.LogSender{}
	env.rebuildHTTPServer(t)
	userID := newPushTestUser(t, ctx, env, cleaner, "verify-unconfigured")
	bearer, _ := createTestUserSessionForUser(t, env, userID)

	endpoint := testEndpoint()
	if _, err := env.Store.SavePushSubscription(ctx, store.SavePushSubscriptionParams{
		UserID: userID, Endpoint: endpoint, P256dh: "k", Auth: "a",
	}); err != nil {
		t.Fatalf("SavePushSubscription: %v", err)
	}

	var verify struct {
		VerifyPushSubscription struct {
			Sent bool
		}
	}
	env.Client.MustPost(
		`mutation ($e: String!) { verifyPushSubscription(endpoint: $e) { sent } }`, &verify,
		client.Var("e", endpoint),
		client.AddHeader("Authorization", bearer),
	)
	if verify.VerifyPushSubscription.Sent {
		t.Fatal("a deployment with no VAPID keys sends nothing")
	}
}
