package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

/*
Three independent states, easy to confuse:

  - disconnected -- the WebSocket is gone (see user_presence).
  - away         -- socket alive, attention gone (backgrounded tab).
  - unreachable  -- no push path to this player. This file.

This is not a presence check. It answers "can we get them back", never "are
they here right now".
*/

// ReachabilityReason explains a verdict. Each reason has a different fix, so
// the seat-hold tiers can be tuned on real data.
type ReachabilityReason string

const (
	// ReachabilityReachable -- at least one live subscription.
	ReachabilityReachable ReachabilityReason = "reachable"
	// ReachabilityNeverSubscribed -- never registered an install. A UX problem.
	ReachabilityNeverSubscribed ReachabilityReason = "never-subscribed"
	// ReachabilitySubscriptionExpired -- opted in, but every install has since
	// been rejected. A storage-invalidation problem.
	ReachabilitySubscriptionExpired ReachabilityReason = "subscription-expired"
	// ReachabilityPushNotConfigured -- no VAPID keys on this deployment, so
	// nothing can be delivered whatever is stored.
	ReachabilityPushNotConfigured ReachabilityReason = "push-not-configured"
)

// PushReachability is a user's push device state at one instant.
//
// Context-free -- no match, queue, or mode -- because it is a property of the
// user's devices and several callers ask about different subjects.
type PushReachability struct {
	UserID uuid.UUID
	// Live is every install still believed deliverable, newest last.
	Live []PushSubscription
	// ExpiredCount and LastExpiredAt cover installs the push service rejected.
	ExpiredCount  int
	LastExpiredAt *time.Time
	// LastDeliveredAt is the most recent successful send, nil if none ever was.
	LastDeliveredAt *time.Time
}

// Reason gives the verdict. pushConfigured says whether the deployment has
// VAPID keys; the store cannot see the sender, so the caller must supply it.
// Without it, stored subscriptions on a keyless deployment would read as
// reachable.
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

// Reachable is the boolean form of Reason.
func (r *PushReachability) Reachable(pushConfigured bool) bool {
	return r.Reason(pushConfigured) == ReachabilityReachable
}

// GetPushReachability reports a user's push device state.
//
// Read-only and side-effect free, so a caller may discard a late result.
// Call it live rather than caching: a player can become reachable mid-wait by
// accepting the notify prompt.
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
