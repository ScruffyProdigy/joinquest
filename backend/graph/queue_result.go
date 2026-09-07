package graph

import (
	"strings"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/graph/model"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

func joinSwitchMessage(from *store.SwitchedFromQueue) *string {
	if from == nil || from.GameName == "" {
		return nil
	}
	msg := store.SwitchedFromPlayerMessage(from.GameName)
	return &msg
}

func joinResultQueuePath(path *string) *string {
	if path == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*path)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// joinResultFromQueueJoin is the joinQueue mutation response for waiters: always "in queue"
// from the client's perspective. Match details are delivered via queueUpdated / myQueueStatus.
func joinResultFromQueueJoin(result *store.QueueJoinResult) *model.JoinResult {
	if result == nil {
		return &model.JoinResult{Queued: false}
	}

	count := result.QueuedCount
	return &model.JoinResult{
		Queued:      true,
		QueuedCount: &count,
		QueuePath:   joinResultQueuePath(result.QueuePath),
		Message:     joinSwitchMessage(result.SwitchedFrom),
	}
}

// joinResultAfterJoin is the joinQueue response when matchmaking still runs inline (tests).
// Production joinQueue uses joinResultFromQueueJoin; matches arrive via the forming worker.
func joinResultAfterJoin(result *store.QueueJoinResult, userID uuid.UUID, launchURLs map[uuid.UUID]string) *model.JoinResult {
	if result == nil {
		return &model.JoinResult{Queued: false}
	}

	if result.Status == store.QueueStatusMatched && result.SessionID != nil {
		sessionID := result.SessionID.String()
		resp := &model.JoinResult{
			Queued:    false,
			SessionID: &sessionID,
			QueuePath: joinResultQueuePath(result.QueuePath),
			Message:   joinSwitchMessage(result.SwitchedFrom),
		}
		if launchURLs != nil {
			if url, ok := launchURLs[userID]; ok && url != "" {
				resp.JoinURL = &url
			}
		}
		return resp
	}

	return joinResultFromQueueJoin(result)
}

func joinResultFromQueueView(view *store.UserQueueView, launchURL string) *model.JoinResult {
	if view == nil || !view.InQueue {
		return &model.JoinResult{Queued: false}
	}

	if view.Waiting {
		count := view.QueuedCount
		return &model.JoinResult{
			Queued:      true,
			QueuedCount: &count,
			QueuePath:   joinResultQueuePath(view.QueuePath),
		}
	}

	if view.Matched && view.SessionID != nil {
		sessionID := view.SessionID.String()
		result := &model.JoinResult{
			Queued:    false,
			SessionID: &sessionID,
		}
		if launchURL != "" {
			result.JoinURL = &launchURL
		}
		return result
	}

	return &model.JoinResult{Queued: false}
}
