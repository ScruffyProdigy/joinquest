package graph

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/graph/model"
)

// pushCapabilityFor builds the snapshot the frontend gates its notify control
// on. The public key is reported only when this deployment can actually send,
// so the browser cannot subscribe to an endpoint nothing will push to.
//
// `reachable` counts verified subscriptions only -- see
// internal/store/push_verification.go for why a registration on its own is not
// an answer.
func (r *Resolver) pushCapabilityFor(ctx context.Context, userID uuid.UUID) (*model.PushCapability, error) {
	st, err := r.requireStore()
	if err != nil {
		return nil, err
	}

	reach, err := st.GetPushReachability(ctx, userID)
	if err != nil {
		return nil, err
	}

	out := &model.PushCapability{
		SubscriptionCount: len(reach.Live),
		VerifiedCount:     len(reach.Verified),
	}
	if key := strings.TrimSpace(r.pushSender().PublicKey()); key != "" {
		out.PublicKey = &key
		// Three halves, really: a way to send, a stored subscription, and a
		// push that has actually come back from it. The third is what stops
		// the frontend offering to hold a place on an untested endpoint.
		out.Reachable = reach.Reachable(true)
	}
	return out, nil
}
