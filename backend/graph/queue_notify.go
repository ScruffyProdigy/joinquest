package graph

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/graph/model"
	"github.com/scruffyprodigy/joinquest/internal/pubsub"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

// publishQueueResult tells every user in result.NotifyUserIDs where the queue
// stands. It is best-effort by design: the queue rows are already committed when
// it runs, so a broker hiccup for one player is that player's problem and must
// not cost the rest of the lobby their notification. Every user is attempted, and
// the failures come back joined together — callers log that error rather than
// aborting, since aborting the mutation cannot un-form the match (JQ-247).
func (r *Resolver) publishQueueResult(ctx context.Context, result *store.QueueJoinResult, launchURLs map[uuid.UUID]string) error {
	if r.PubSub == nil || result == nil {
		return nil
	}

	var errs []error
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
			errs = append(errs, fmt.Errorf("publish queue event to user %s: %w", userID, err))
		}
	}
	return errors.Join(errs...)
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
	// The switch is committed before either publish runs, so a failure telling the
	// switcher they left must not cost the players still waiting their new count.
	leftErr := r.publishQueueLeft(ctx, from.GameID, from.ModeQueueID, userID, count, leftMsg)
	waiters, err := st.ListWaitingUserIDsInModeQueue(ctx, from.ModeQueueID)
	if err != nil {
		return errors.Join(leftErr, err)
	}
	return errors.Join(leftErr, r.publishQueueWaiting(ctx, from.GameID, from.ModeQueueID, waiters, count))
}

// publishQueueWaiting tells each of userIDs the queue is still forming at count.
// Like publishQueueResult it completes the fan-out and joins any failures, so one
// unreachable player does not leave the rest of the queue on a stale count.
func (r *Resolver) publishQueueWaiting(ctx context.Context, gameID, modeQueueID uuid.UUID, userIDs []uuid.UUID, count int) error {
	if r.PubSub == nil {
		return nil
	}
	var errs []error
	for _, userID := range userIDs {
		event := pubsub.QueueEvent{
			GameID:      gameID.String(),
			QueueID:     modeQueueID.String(),
			Status:      pubsub.QueueStatusWaiting,
			QueuedCount: count,
		}
		if err := pubsub.PublishQueueEvent(ctx, r.PubSub, userID.String(), event); err != nil {
			errs = append(errs, fmt.Errorf("publish queue event to user %s: %w", userID, err))
		}
	}
	return errors.Join(errs...)
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
