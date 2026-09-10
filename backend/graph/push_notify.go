package graph

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/pubsub"
	"github.com/scruffyprodigy/joinquest/internal/push"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

// PushMatchReady tells one absent player their match is ready, across every
// install they have, and publishes the outcome.
//
// # Why this returns an outcome AND publishes an event
//
// Push services only report a dead subscription (HTTP 410 Gone) on an actual
// send, so stored reachability is a prediction that is not confirmed until we
// try. JQ-199's seat hold has to pick a tier before that -- it starts a player
// with a live subscription in the long tier, and needs to find out promptly if
// that was wrong, rather than running a two-minute ceiling for someone who will
// never be told. The event is how the hold learns; the return value is for a
// caller that happens to be synchronous.
//
// # Best-effort, always
//
// Never returns an error that should abort the caller. This runs inside
// match-formation fan-out, and a push service hiccup must not fail a match that
// formed correctly. Errors are folded into the outcome instead.
func (r *Resolver) PushMatchReady(ctx context.Context, userID uuid.UUID, sessionID string, note push.Notification) pubsub.PushDeliveryEvent {
	event := pubsub.PushDeliveryEvent{
		UserID:    userID.String(),
		SessionID: sessionID,
		Status:    pubsub.PushSkipped,
	}

	st, err := r.requireStore()
	if err != nil {
		r.publishPushDelivery(ctx, event)
		return event
	}
	// An unbounded match-ready push is a bug, not a lenient default. Without a
	// TTL the push service applies its own retention, which outlives any stall
	// budget -- the player would be told to claim a seat that is already gone.
	// Build the notification with MatchReadyNotification, which cannot produce
	// one of these.
	if note.TTL <= 0 {
		log.Printf("push: refusing an unbounded match-ready notification for %s", userID)
		r.publishPushDelivery(ctx, event)
		return event
	}

	sender := r.pushSender()
	if sender.PublicKey() == "" {
		// Nothing can be signed. Skipped rather than failed: no attempt was
		// made, and the hold was never entitled to the optimistic tier.
		r.publishPushDelivery(ctx, event)
		return event
	}

	subs, err := st.ListPushSubscriptions(ctx, userID)
	if err != nil {
		log.Printf("push: could not load subscriptions for %s: %v", userID, err)
		event.Status = pubsub.PushFailed
		r.publishPushDelivery(ctx, event)
		return event
	}
	if len(subs) == 0 {
		r.publishPushDelivery(ctx, event)
		return event
	}

	event.Attempted = len(subs)
	var transientFailures int

	for _, sub := range subs {
		err := sender.Send(ctx, push.Subscription{
			Endpoint: sub.Endpoint,
			P256dh:   sub.P256dh,
			Auth:     sub.Auth,
		}, note)

		switch {
		case err == nil:
			event.Delivered++
			// Records that this install genuinely works, which is what
			// separates a subscription that has delivered from one that merely
			// exists. Failure to record it is not worth failing a delivery.
			if touchErr := st.TouchPushSubscription(ctx, sub.Endpoint); touchErr != nil {
				log.Printf("push: could not record delivery for %s: %v", sub.Endpoint, touchErr)
			}

		case errors.Is(err, push.ErrSubscriptionGone):
			// Permanently dead. Mark it so reachability stops counting it --
			// leaving it would make the next hold optimistic on the same bad
			// evidence. Marked, not deleted, so "opted in and it died" stays
			// distinguishable from "never opted in".
			event.Expired++
			if markErr := st.MarkPushSubscriptionExpired(ctx, sub.Endpoint); markErr != nil {
				log.Printf("push: could not mark %s expired: %v", sub.Endpoint, markErr)
			}

		default:
			transientFailures++
			log.Printf("push: send to %s failed: %v", sub.Endpoint, err)
		}
	}

	switch {
	case event.Delivered > 0:
		// Partial success is success: one working install is enough to get the
		// player back, whatever happened to the others.
		event.Status = pubsub.PushDelivered
	case transientFailures > 0:
		// Says nothing about reachability -- only that this attempt did not
		// land. Collapsing this into UNDELIVERABLE would evict players over a
		// push-service hiccup.
		event.Status = pubsub.PushFailed
	default:
		// Every install was permanently rejected. The player is now known
		// unreachable and the hold should collapse to its floor.
		event.Status = pubsub.PushUndeliverable
	}

	r.publishPushDelivery(ctx, event)
	return event
}

// publishPushDelivery emits the outcome, swallowing broker errors.
func (r *Resolver) publishPushDelivery(ctx context.Context, event pubsub.PushDeliveryEvent) {
	if r.PubSub == nil {
		return
	}
	if err := pubsub.PublishPushDelivery(ctx, r.PubSub, event); err != nil {
		log.Printf("push: could not publish delivery outcome for %s: %v", event.UserID, err)
	}
}

// MatchReadyNotification builds the match-ready ping for a seat with
// `remaining` left on its stall budget.
//
// Returns ok=false when there is no budget left. The seat is already gone at
// that point, and a notification that arrives after it is the failure the
// ticket calls worse than sending nothing -- the player taps, lands on a
// released seat, and gets the dead end JQ-199 AC #5 exists to prevent.
//
// `remaining` is the budget left AT SEND TIME, deliberately not the ceiling.
// Under JQ-199's match-level stall budget the hold is explicitly not a fixed
// promise: a busy queue frees the seat early. A TTL of "the ceiling" would
// promise two minutes on a seat with forty seconds left.
//
// There is no default. An unset TTL would fall through to the push service's
// own retention (or push.DefaultTTL), both of which outlive any hold -- so the
// duration is required, and the ok=false return makes an expired budget
// impossible to send on by accident rather than merely discouraged.
func MatchReadyNotification(joinURL string, remaining time.Duration) (push.Notification, bool) {
	if remaining <= 0 {
		return push.Notification{}, false
	}
	return push.Notification{
		Title: "Your match is ready",
		// Deliberately no countdown in the copy. The body is rendered whenever
		// the push is delivered, which may be seconds after it was built, so a
		// stated time would be wrong exactly when it mattered most.
		Body: "Tap to take your seat.",
		URL:  joinURL,
		Tag:  "joinquest-match-ready",
		TTL:  remaining,
	}, true
}

// ReachabilityForHold is a convenience for the seat-hold path: the combined
// verdict plus the record, in one call, failing to the conservative answer
// rather than making the caller decide what an error means.
//
// Exists so the hold logic never reaches for store.GetPushReachability
// directly, which would answer only "is something stored" and would put an
// absent player in the expensive tier on a deployment that cannot send.
func (r *Resolver) ReachabilityForHold(ctx context.Context, userID uuid.UUID) (*store.PushReachability, store.ReachabilityReason) {
	reach, reason, err := r.PushReachability(ctx, userID)
	if err != nil {
		// Fail to the conservative answer. A false positive costs every player
		// who did show up; a false negative costs one absent player their seat.
		log.Printf("push: reachability lookup failed for %s: %v", userID, err)
		return nil, store.ReachabilityNeverSubscribed
	}
	return reach, reason
}
