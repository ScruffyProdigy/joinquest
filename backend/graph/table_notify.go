package graph

import (
	"context"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/pubsub"
)

func (r *Resolver) publishTableSeatStarted(ctx context.Context, tableID uuid.UUID, userIDs []uuid.UUID, launchURLs map[uuid.UUID]string) error {
	if r.PubSub == nil || tableID == uuid.Nil {
		return nil
	}
	seen := make(map[uuid.UUID]struct{}, len(userIDs))
	for _, userID := range userIDs {
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		event := pubsub.TableSeatEvent{
			Status:  pubsub.TableSeatStatusStarted,
			TableID: tableID.String(),
		}
		if launchURLs != nil {
			event.JoinURL = launchURLs[userID]
		}
		if err := pubsub.PublishTableSeatEvent(ctx, r.PubSub, userID.String(), event); err != nil {
			return err
		}
	}
	return nil
}
