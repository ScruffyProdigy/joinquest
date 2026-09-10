package graph

import (
	"context"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

// PushReachability reports whether push can reach a user, and why not.
//
// The combined answer, and the one to use: it folds in whether this deployment
// has VAPID keys, which store.GetPushReachability cannot see.
//
// Read-only, so a late result is safe to discard. Call it live rather than
// caching. Not a presence check -- see internal/store/push_reachability.go.
func (r *Resolver) PushReachability(ctx context.Context, userID uuid.UUID) (*store.PushReachability, store.ReachabilityReason, error) {
	st, err := r.requireStore()
	if err != nil {
		return nil, store.ReachabilityPushNotConfigured, err
	}
	reach, err := st.GetPushReachability(ctx, userID)
	if err != nil {
		return nil, store.ReachabilityPushNotConfigured, err
	}
	return reach, reach.Reason(r.pushConfigured()), nil
}

// PushReachable is the boolean form of PushReachability.
func (r *Resolver) PushReachable(ctx context.Context, userID uuid.UUID) (bool, error) {
	_, reason, err := r.PushReachability(ctx, userID)
	if err != nil {
		return false, err
	}
	return reason == store.ReachabilityReachable, nil
}

// pushConfigured reports whether this deployment can sign and send.
func (r *Resolver) pushConfigured() bool {
	return r.pushSender().PublicKey() != ""
}
