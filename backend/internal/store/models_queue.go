package store

import (
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/prequeue"
)

// UserQueueView is the current queue/match state for a user.
type UserQueueView struct {
	GameID      uuid.UUID
	ModeQueueID uuid.UUID
	InQueue     bool
	Waiting     bool
	Matched     bool
	QueuedCount int
	QueuePath   *string
	SessionID   *uuid.UUID
}

// SwitchedFromQueue records a waiting queue the user left to join another.
type SwitchedFromQueue struct {
	ModeQueueID uuid.UUID
	GameID      uuid.UUID
	GameName    string
}

// SwitchedFromPlayerMessage is shown on joinQueue when the player left another game's wait list.
func SwitchedFromPlayerMessage(gameName string) string {
	return "You left the group for " + gameName + " to look for a group here."
}

// QueueJoinResult is returned when a user joins a game queue.
type QueueJoinResult struct {
	GameID         uuid.UUID
	ModeQueueID    uuid.UUID
	Status         string
	SessionID      *uuid.UUID
	QueuedCount    int
	QueuePath      *string
	NotifyUserIDs  []uuid.UUID
	AlreadyInQueue bool
	SwitchedFrom   *SwitchedFromQueue
}

type QueueEntry struct {
	ID             uuid.UUID
	GameID         uuid.UUID
	ModeQueueID    *uuid.UUID
	UserID         uuid.UUID
	Status         string
	QueuePath      *string
	PartyID        *uuid.UUID
	FormingMatchID *uuid.UUID
	// QueueOptions is what this player picked before queueing, frozen at join.
	QueueOptions []prequeue.Selection
	JoinedAt     time.Time
}
