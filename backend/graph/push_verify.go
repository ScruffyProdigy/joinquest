package graph

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/push"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

// verificationTTL is how long the push service should keep trying to deliver a
// verification push.
//
// Short on purpose. A player is standing in front of the waiting page having
// just pressed a button: an ack that arrives two minutes later answers a
// question nobody is still asking, and the UI has long since told them we
// could not reach them.
const verificationTTL = 45 * time.Second

// VerificationNotification builds the silent round-trip push.
//
// Title and Body are present but are not meant to be seen: sw.js recognises the
// kind and acks without showing anything. They exist for the one case where it
// does not -- a service worker too old to know about verification -- where a
// browser enforcing userVisibleOnly would otherwise substitute its own "this
// site was updated in the background" notice. Better our words than that.
func VerificationNotification(token string) push.Notification {
	return push.Notification{
		Kind:  push.KindVerify,
		Token: token,
		Title: "Notifications are on",
		Body:  "We can reach you here when your game is ready.",
		// Not the come-back tag: this must never replace a live seat alert.
		Tag: "joinquest-verify",
		TTL: verificationTTL,
	}
}

// StartPushVerification sends the round-trip push for one of a player's
// installs and reports whether anything went out.
//
// Sending is the whole test. It returns as soon as the push service has taken
// the message, because the answer that matters -- did it arrive -- comes back
// through ConfirmPushVerification, on a path this call cannot wait for.
//
// A permanent rejection is settled here and now: the endpoint is marked expired
// rather than left looking like an ack that is still coming.
func (r *Resolver) StartPushVerification(ctx context.Context, userID uuid.UUID, endpoint string) (bool, error) {
	st, err := r.requireStore()
	if err != nil {
		return false, err
	}

	sender := r.pushSender()
	if sender.PublicKey() == "" {
		// Nothing can be signed, so nothing was attempted. Not an error: a
		// deployment without VAPID keys is a normal local one.
		return false, nil
	}

	sub, token, err := st.StartPushSubscriptionVerification(ctx, userID, endpoint)
	if err != nil {
		if errors.Is(err, store.ErrPushVerificationNotFound) {
			// The endpoint is not this player's, or is already expired.
			// Either way there is nothing to verify.
			return false, nil
		}
		return false, err
	}

	err = sender.Send(ctx, push.Subscription{
		Endpoint: sub.Endpoint,
		P256dh:   sub.P256dh,
		Auth:     sub.Auth,
	}, VerificationNotification(token))

	switch {
	case err == nil:
		return true, nil

	case errors.Is(err, push.ErrSubscriptionGone):
		// The push service says this endpoint is dead. Waiting for an ack
		// would be waiting for nothing.
		if markErr := st.MarkPushSubscriptionExpired(ctx, sub.Endpoint); markErr != nil {
			log.Printf("push: could not mark %s expired: %v", sub.Endpoint, markErr)
		}
		return false, nil

	default:
		// Transient. Nothing was delivered, so the caller must not wait, but
		// the subscription is still worth keeping.
		log.Printf("push: verification send to %s failed: %v", sub.Endpoint, err)
		return false, nil
	}
}
