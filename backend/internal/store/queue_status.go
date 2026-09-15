package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// GetUserModeQueueView returns whether the user is waiting, matched, or not in a mode queue.
func (s *Store) GetUserModeQueueView(ctx context.Context, modeQueueID, userID uuid.UUID) (*UserQueueView, error) {
	modeQueue, err := s.GetModeQueueByID(ctx, modeQueueID)
	if err != nil {
		return nil, err
	}
	mode, err := getGameModeByID(ctx, s.db, modeQueue.ModeID)
	if err != nil {
		return nil, err
	}

	if err := expireStaleMatchedModeQueue(ctx, s.db, modeQueueID, userID); err != nil {
		return nil, err
	}

	entry, err := s.GetWaitingModeQueueEntry(ctx, modeQueueID, userID)
	if err == nil {
		count, err := s.CountWaitingInModeQueue(ctx, modeQueueID)
		if err != nil {
			return nil, err
		}
		return &UserQueueView{
			GameID:      mode.GameID,
			ModeQueueID: modeQueueID,
			InQueue:     true,
			Waiting:     true,
			QueuedCount: count,
			QueuePath:   entry.QueuePath,
		}, nil
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	session, err := s.GetMatchedSessionForUserAndModeQueue(ctx, modeQueueID, userID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if session != nil {
		return &UserQueueView{
			GameID:      mode.GameID,
			ModeQueueID: modeQueueID,
			InQueue:     true,
			Matched:     true,
			SessionID:   &session.ID,
		}, nil
	}

	return &UserQueueView{
		GameID:      mode.GameID,
		ModeQueueID: modeQueueID,
	}, nil
}
