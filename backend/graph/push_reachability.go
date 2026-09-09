package graph

import (
	"context"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

// PushReachability reports whether push can reach a user, and why not when it
// cannot.
//
// This is the combined answer, and the one callers outside this package should
// use. store.GetPushReachability alone only knows what is stored; whether this
// deployment has VAPID keys to send with lives on the Resolver, and a stored
// subscription on a deployment that cannot send is not reachability. Calling
// the store directly and treating a live row as reachable would put an absent
// player in JQ-199's long hold tier when nothing will ever tell them to come
// back.
//
// Contract, per JQ-199's concurrency design:
//   - Read-only, no side effects. The hold starts at its conservative floor and
//     this runs alongside, upgrading the hold only if it resolves in time, so a
//     late result is discarded and must be safe to ignore.
//   - Context-free by user, deliberately: JQ-216 asks the same question about a
//     queued player who dropped, JQ-199 about a matched player who is absent.
//     Device state does not depend on the subject.
//   - Read live. Never snapshot it at queue-join: a player can go unreachable
//     -> reachable during the wait, which is exactly what the notify affordance
//     is for.
//
// NOT a presence check. "Can we get them back", never "are they here right
// now" -- see the vocabulary note in internal/store/push_reachability.go.
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

// PushReachable is the boolean form, for callers that genuinely only need the
// tier and will not log why.
func (r *Resolver) PushReachable(ctx context.Context, userID uuid.UUID) (bool, error) {
	_, reason, err := r.PushReachability(ctx, userID)
	if err != nil {
		return false, err
	}
	return reason == store.ReachabilityReachable, nil
}

// pushConfigured reports whether this deployment can actually sign and send.
func (r *Resolver) pushConfigured() bool {
	return r.pushSender().PublicKey() != ""
}
