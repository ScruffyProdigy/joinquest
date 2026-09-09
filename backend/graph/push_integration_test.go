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
	PublicKey         *string
}

const pushCapabilityFields = `reachable subscriptionCount publicKey`

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

func TestSavePushSubscriptionMakesThePlayerReachable(t *testing.T) {
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

	if !saved.SavePushSubscription.Reachable {
		t.Fatal("a player with a stored subscription must be reachable")
	}
	if saved.SavePushSubscription.SubscriptionCount != 1 {
		t.Fatalf("expected one subscription, got %d", saved.SavePushSubscription.SubscriptionCount)
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
	if !guest.SavePushSubscription.Reachable {
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
	if !saved.SavePushSubscription.Reachable {
		t.Fatal("precondition: the player should be reachable before logging out")
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
