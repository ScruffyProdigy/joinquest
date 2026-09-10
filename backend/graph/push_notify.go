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

// PushComeBack notifies one away player across every install they have, marks
// any dead endpoints, and publishes the outcome so the caller can react.
//
// Carries either notification: SeatHeldNotification when a chair is already
// theirs, SeatOpenNotification when they are the missing piece but hold
// nothing.
//
// The outcome is both returned and published because a push service only
// reveals a dead subscription on an actual send, after the hold has already
// picked a tier.
//
// Best-effort: it never returns an error. This runs inside match-formation
// fan-out, so a push failure must not fail a match that formed correctly.
func (r *Resolver) PushComeBack(ctx context.Context, userID uuid.UUID, formingMatchID string, note push.Notification) pubsub.PushDeliveryEvent {
	event := pubsub.PushDeliveryEvent{
		UserID:         userID.String(),
		FormingMatchID: formingMatchID,
		Status:         pubsub.PushSkipped,
	}

	st, err := r.requireStore()
	if err != nil {
		r.publishPushDelivery(ctx, event)
		return event
	}
	// Without a TTL the push service applies its own retention, which outlives
	// the situation being described. Build these with SeatHeldNotification or
	// SeatOpenNotification.
	if note.TTL <= 0 {
		log.Printf("push: refusing an unbounded come-back notification for %s", userID)
		r.publishPushDelivery(ctx, event)
		return event
	}

	sender := r.pushSender()
	if sender.PublicKey() == "" {
		// Nothing can be signed, so nothing was attempted.
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
			// Not worth failing a delivery over.
			if touchErr := st.TouchPushSubscription(ctx, sub.Endpoint); touchErr != nil {
				log.Printf("push: could not record delivery for %s: %v", sub.Endpoint, touchErr)
			}

		case errors.Is(err, push.ErrSubscriptionGone):
			// Stop counting it, or the next hold repeats this mistake.
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
		// One working install is enough to get the player back.
		event.Status = pubsub.PushDelivered
	case transientFailures > 0:
		// Transient. Says nothing about reachability, so do not evict them.
		event.Status = pubsub.PushFailed
	default:
		// Every install was rejected: the player is now known unreachable.
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

// comeBackTag collapses every "return to your queue" notification into one
// alert. The claims supersede each other, so the newest should replace the
// last rather than stack beside it.
const comeBackTag = "joinquest-come-back"

// SeatHeldNotification builds the ping for a chair already held for this
// player, with `remaining` left on the hold. ok=false when there is none: the
// chair is gone and they would tap through to nothing.
//
// The loss-framing is licensed by the mechanic -- the chair really is vacated
// if they do not return. Do not reuse this phrasing where the seat is not
// already theirs; there it would be a dark pattern. SeatOpenNotification is
// the version for that case.
//
// `remaining` is the time left at SEND time, not the ceiling. There is no
// default: an unset TTL outlives the hold.
func SeatHeldNotification(joinURL string, remaining time.Duration) (push.Notification, bool) {
	if remaining <= 0 {
		return push.Notification{}, false
	}
	return push.Notification{
		Title: "Your game is nearly ready",
		// No countdown: the body renders at delivery, so any stated time is
		// already stale.
		Body: "Come back now to keep your spot.",
		URL:  joinURL,
		Tag:  comeBackTag,
		TTL:  remaining,
	}, true
}

// SeatOpenNotification builds the ping for a player who is NOT seated, where a
// table is one seat short and nobody present can fill it. Their return is what
// makes the match happen.
//
// Says nothing spot-shaped. No chair is held here and a present player arriving
// first should get it, so any phrasing implying the seat is theirs would be a
// promise the system has deliberately not made.
//
// It also avoids racing language. This only fires when no present player can
// take the seat, so urgency borrowed from a contest would be urgency about a
// contest that does not exist at send time.
//
// `validFor` bounds how long the claim is expected to hold. Required for the
// same reason as SeatHeldNotification's remaining: an unset TTL outlives the
// situation it describes.
func SeatOpenNotification(joinURL string, validFor time.Duration) (push.Notification, bool) {
	if validFor <= 0 {
		return push.Notification{}, false
	}
	return push.Notification{
		Title: "One more player and your game starts",
		Body:  "Come back now to join.",
		URL:   joinURL,
		Tag:   comeBackTag,
		TTL:   validFor,
	}, true
}

// ReachabilityForHold is PushReachability with errors folded into the
// conservative answer. Prefer it over store.GetPushReachability, which does not
// know whether this deployment can send.
func (r *Resolver) ReachabilityForHold(ctx context.Context, userID uuid.UUID) (*store.PushReachability, store.ReachabilityReason) {
	reach, reason, err := r.PushReachability(ctx, userID)
	if err != nil {
		// A false positive costs every player who did show up.
		log.Printf("push: reachability lookup failed for %s: %v", userID, err)
		return nil, store.ReachabilityNeverSubscribed
	}
	return reach, reason
}
