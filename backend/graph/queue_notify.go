package graph

import (
	"context"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/graph/model"
	"github.com/scruffyprodigy/joinquest/internal/pubsub"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

func (r *Resolver) publishQueueResult(ctx context.Context, result *store.QueueJoinResult, launchURLs map[uuid.UUID]string) error {
	if r.PubSub == nil || result == nil {
		return nil
	}

	seen := make(map[uuid.UUID]struct{}, len(result.NotifyUserIDs))
	for _, userID := range result.NotifyUserIDs {
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}

		event := pubsub.QueueEvent{
			GameID:      result.GameID.String(),
			QueuedCount: result.QueuedCount,
		}
		if result.ModeQueueID != uuid.Nil {
			event.QueueID = result.ModeQueueID.String()
		}

		switch result.Status {
		case store.QueueStatusMatched:
			event.Status = pubsub.QueueStatusMatched
			if result.SessionID != nil {
				event.SessionID = result.SessionID.String()
				if launchURLs != nil {
					event.JoinURL = launchURLs[userID]
				}
			}
		default:
			event.Status = pubsub.QueueStatusWaiting
		}

		if err := pubsub.PublishQueueEvent(ctx, r.PubSub, userID.String(), event); err != nil {
			return err
		}
	}
	return nil
}

func (r *Resolver) publishQueueLeft(ctx context.Context, gameID, modeQueueID, userID uuid.UUID, queuedCount int, message string) error {
	if r.PubSub == nil {
		return nil
	}
	event := pubsub.QueueEvent{
		GameID:      gameID.String(),
		Status:      pubsub.QueueStatusLeft,
		QueuedCount: queuedCount,
		Message:     message,
	}
	if modeQueueID != uuid.Nil {
		event.QueueID = modeQueueID.String()
	}
	return pubsub.PublishQueueEvent(ctx, r.PubSub, userID.String(), event)
}

// publishQueueSwitchFrom notifies the player and remaining waiters after leaving a queue to join another.
func (r *Resolver) publishQueueSwitchFrom(ctx context.Context, st *store.Store, from *store.SwitchedFromQueue, userID uuid.UUID) error {
	if r.PubSub == nil || from == nil {
		return nil
	}
	count, err := st.CountWaitingInModeQueue(ctx, from.ModeQueueID)
	if err != nil {
		return err
	}
	leftMsg := "You left this queue to join another game."
	if err := r.publishQueueLeft(ctx, from.GameID, from.ModeQueueID, userID, count, leftMsg); err != nil {
		return err
	}
	waiters, err := st.ListWaitingUserIDsInModeQueue(ctx, from.ModeQueueID)
	if err != nil {
		return err
	}
	for _, waiterID := range waiters {
		event := pubsub.QueueEvent{
			GameID:      from.GameID.String(),
			QueueID:     from.ModeQueueID.String(),
			Status:      pubsub.QueueStatusWaiting,
			QueuedCount: count,
		}
		if err := pubsub.PublishQueueEvent(ctx, r.PubSub, waiterID.String(), event); err != nil {
			return err
		}
	}
	return nil
}

func queueUpdateFromView(view *store.UserQueueView, launchURL string) *model.QueueUpdate {
	if view == nil || !view.InQueue {
		return nil
	}

	gameIDStr := view.GameID.String()
	queueIDStr := ""
	if view.ModeQueueID != uuid.Nil {
		queueIDStr = view.ModeQueueID.String()
	}
	update := &model.QueueUpdate{
		GameID:      gameIDStr,
		QueueID:     queueIDStr,
		QueuedCount: view.QueuedCount,
		FormingGaps: []*model.QueuePathGap{},
	}

	if view.Matched && view.SessionID != nil {
		update.Status = model.QueueStatusMatched
		update.QueuedCount = 0
		sessionID := view.SessionID.String()
		update.SessionID = &sessionID
		if launchURL != "" {
			update.JoinURL = &launchURL
		}
		return update
	}

	if view.Waiting {
		update.Status = model.QueueStatusWaiting
		return update
	}

	return nil
}

func toGraphQLQueueUpdate(event pubsub.QueueEvent) *model.QueueUpdate {
	update := &model.QueueUpdate{
		GameID:      event.GameID,
		QueueID:     event.QueueID,
		QueuedCount: event.QueuedCount,
		FormingGaps: []*model.QueuePathGap{},
	}
	switch event.Status {
	case pubsub.QueueStatusMatched:
		update.Status = model.QueueStatusMatched
	case pubsub.QueueStatusLeft:
		update.Status = model.QueueStatusLeft
	default:
		update.Status = model.QueueStatusWaiting
	}
	if event.SessionID != "" {
		update.SessionID = &event.SessionID
	}
	if event.JoinURL != "" {
		update.JoinURL = &event.JoinURL
	}
	if event.Message != "" {
		update.Message = &event.Message
	}
	return update
}

func (r *Resolver) enrichQueueUpdateGaps(ctx context.Context, update *model.QueueUpdate) error {
	if update == nil {
		return nil
	}
	if update.Status != model.QueueStatusWaiting || update.QueueID == "" {
		if update.FormingGaps == nil {
			update.FormingGaps = []*model.QueuePathGap{}
		}
		return nil
	}
	modeQueueID, err := parseUUID(update.QueueID, "queue id")
	if err != nil {
		return err
	}
	gaps, err := r.formingGapsForModeQueue(ctx, modeQueueID)
	if err != nil {
		return err
	}
	update.FormingGaps = gaps
	return nil
}
