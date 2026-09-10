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

// PushSeatHeld notifies one away player across every install they have, marks
// any dead endpoints, and publishes the outcome so the seat hold can react.
//
// The outcome is both returned and published because a push service only
// reveals a dead subscription on an actual send, after the hold has already
// picked a tier.
//
// Best-effort: it never returns an error. This runs inside match-formation
// fan-out, so a push failure must not fail a match that formed correctly.
func (r *Resolver) PushSeatHeld(ctx context.Context, userID uuid.UUID, holdID string, note push.Notification) pubsub.PushDeliveryEvent {
	event := pubsub.PushDeliveryEvent{
		UserID: userID.String(),
		HoldID: holdID,
		Status: pubsub.PushSkipped,
	}

	st, err := r.requireStore()
	if err != nil {
		r.publishPushDelivery(ctx, event)
		return event
	}
	// Without a TTL the push service applies its own retention, which outlives
	// any seat. Build these with SeatHeldNotification.
	if note.TTL <= 0 {
		log.Printf("push: refusing an unbounded seat-held notification for %s", userID)
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

// SeatHeldNotification builds the ping for a chair held for `remaining`.
// ok=false when there is none, because the chair is already gone and the player
// would tap through to nothing.
//
// The hold happens BEFORE the match is announced, so there is no formed match
// yet and the copy must not claim one.
//
// The loss-framing is licensed by that mechanic: the chair really is vacated if
// the player does not return. Do not reuse this phrasing anywhere the seat is
// already theirs -- there it would be a dark pattern.
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
		Tag:  "joinquest-seat-held",
		TTL:  remaining,
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
