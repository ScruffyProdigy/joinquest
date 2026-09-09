package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

/*
Vocabulary (agreed across JQ-198 / JQ-199 / JQ-216, which were each using one
word for three independent states):

  - disconnected -- the player's WebSocket is gone. JQ-216's trigger.
  - away         -- socket alive, attention gone (backgrounded tab). JQ-199's
    hold trigger.
  - unreachable  -- no push path to this player. What this file answers.

They are independent axes. A player can be present and unreachable (desktop, no
push) or absent and reachable (phone in pocket, opted in), which is the whole
reason the seat-hold tiers are worth having.

NOTHING HERE IS A PRESENCE CHECK. This answers "can we get them back", never
"are they here right now" -- that is JQ-199's to own, and conflating the two
would silently collapse the tiers back into one.
*/

// ReachabilityReason explains a reachability verdict.
//
// The distinction is what makes JQ-199's tier values tunable on real data: each
// reason has a different fix, and a bare boolean collapses all of them into
// "false" so production teaches us nothing.
type ReachabilityReason string

const (
	// ReachabilityReachable -- at least one live subscription.
	ReachabilityReachable ReachabilityReason = "reachable"
	// ReachabilityNeverSubscribed -- this player has never registered an
	// install. A UX problem: the affordance was not shown, not understood, or
	// not taken.
	ReachabilityNeverSubscribed ReachabilityReason = "never-subscribed"
	// ReachabilitySubscriptionExpired -- they opted in and every install has
	// since been rejected by the push service. A storage-invalidation problem,
	// not a UX one.
	ReachabilitySubscriptionExpired ReachabilityReason = "subscription-expired"
	// ReachabilityPushNotConfigured -- this deployment has no VAPID keys, so
	// nothing can be delivered regardless of what is stored. Nobody's fault
	// and the conservative tier is simply correct.
	ReachabilityPushNotConfigured ReachabilityReason = "push-not-configured"
)

// PushReachability is a user's push device state at one instant.
//
// Deliberately context-free -- no match, queue, or mode. It is a property of
// the user's devices, and JQ-216 asks the same question about a different
// subject (a queued player who dropped) than JQ-199 does (a matched player who
// is absent). Taking context would grow a subject parameter and split this
// back into three variants.
type PushReachability struct {
	UserID uuid.UUID
	// Live is every install still believed deliverable, newest last.
	Live []PushSubscription
	// ExpiredCount and LastExpiredAt describe installs the push service has
	// rejected. Retained so "never opted in" stays distinguishable from
	// "opted in, then it died".
	ExpiredCount  int
	LastExpiredAt *time.Time
	// LastDeliveredAt is the most recent successful send across live installs,
	// nil when none has ever succeeded. A subscription that has never actually
	// delivered is weaker evidence than one that has.
	LastDeliveredAt *time.Time
}

// Reason gives the verdict. pushConfigured is whether the deployment has VAPID
// keys, which the store cannot see -- the sender is not visible from here.
//
// Passing it is mandatory rather than assumed because getting it wrong is
// exactly the failure this whole design guards against: stored subscriptions on
// a deployment with no keys would otherwise read as reachable, putting an
// absent player in the long hold tier when nothing will ever tell them to come
// back.
func (r *PushReachability) Reason(pushConfigured bool) ReachabilityReason {
	if !pushConfigured {
		return ReachabilityPushNotConfigured
	}
	if r == nil {
		return ReachabilityNeverSubscribed
	}
	if len(r.Live) > 0 {
		return ReachabilityReachable
	}
	if r.ExpiredCount > 0 {
		return ReachabilitySubscriptionExpired
	}
	return ReachabilityNeverSubscribed
}

// Reachable is the boolean, derived from Reason so the two can never disagree.
func (r *PushReachability) Reachable(pushConfigured bool) bool {
	return r.Reason(pushConfigured) == ReachabilityReachable
}

// GetPushReachability reports whether push can reach a user, and why not when
// it cannot.
//
// Read-only and free of side effects by contract: JQ-199 starts the seat hold
// at its conservative floor and runs this alongside, upgrading the hold only if
// the answer arrives in time. A result that arrives after the seat was already
// backfilled or released is discarded, so this must be safe to ignore.
//
// Read live, never snapshotted at queue-join: a player can go unreachable ->
// reachable during the wait, which is precisely what JQ-198's notify affordance
// is for. A join-time snapshot would report "not reachable" for exactly the
// players the feature just converted.
//
// Always returns a non-nil record for an existing user; see the doc on the
// package-level vocabulary for why a nil-means-unreachable signature would be a
// trap (nil vs non-nil maps to "has rows", which is not the same question).
func (s *Store) GetPushReachability(ctx context.Context, userID uuid.UUID) (*PushReachability, error) {
	const query = `
		SELECT id, user_id, endpoint, p256dh, auth, user_agent, created_at, last_used_at, expired_at
		FROM push_subscriptions
		WHERE user_id = $1
		ORDER BY created_at`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := &PushReachability{UserID: userID}
	for rows.Next() {
		var sub PushSubscription
		if err := rows.Scan(
			&sub.ID, &sub.UserID, &sub.Endpoint, &sub.P256dh, &sub.Auth,
			&sub.UserAgent, &sub.CreatedAt, &sub.LastUsedAt, &sub.ExpiredAt,
		); err != nil {
			return nil, err
		}
		if sub.ExpiredAt != nil {
			out.ExpiredCount++
			if out.LastExpiredAt == nil || sub.ExpiredAt.After(*out.LastExpiredAt) {
				out.LastExpiredAt = sub.ExpiredAt
			}
			continue
		}
		out.Live = append(out.Live, sub)
		if sub.LastUsedAt != nil && (out.LastDeliveredAt == nil || sub.LastUsedAt.After(*out.LastDeliveredAt)) {
			out.LastDeliveredAt = sub.LastUsedAt
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
