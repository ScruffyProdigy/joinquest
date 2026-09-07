package graph

import (
	"context"

	"github.com/scruffyprodigy/joinquest/internal/pubsub"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

func (r *Resolver) publishKickedWaiters(ctx context.Context, kicked []store.KickedWaiter) error {
	if r.PubSub == nil || len(kicked) == 0 {
		return nil
	}
	for _, k := range kicked {
		event := pubsub.QueueEvent{
			GameID:  k.GameID.String(),
			QueueID: k.ModeQueueID.String(),
			Status:  pubsub.QueueStatusLeft,
			Message: k.Message,
		}
		if err := pubsub.PublishQueueEvent(ctx, r.PubSub, k.UserID.String(), event); err != nil {
			return err
		}
	}
	return nil
}
