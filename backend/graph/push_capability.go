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
func (r *Resolver) pushCapabilityFor(ctx context.Context, userID uuid.UUID) (*model.PushCapability, error) {
	st, err := r.requireStore()
	if err != nil {
		return nil, err
	}

	subs, err := st.ListPushSubscriptions(ctx, userID)
	if err != nil {
		return nil, err
	}

	out := &model.PushCapability{
		SubscriptionCount: len(subs),
	}
	if key := strings.TrimSpace(r.pushSender().PublicKey()); key != "" {
		out.PublicKey = &key
		// Both halves are required: a stored subscription and a way to send.
		out.Reachable = len(subs) > 0
	}
	return out, nil
}
