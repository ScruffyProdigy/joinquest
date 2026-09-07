package graph

import (
	"testing"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

func TestJoinResultIncludesSwitchMessage(t *testing.T) {
	name := "Rock Paper Scissors"
	result := joinResultFromQueueJoin(&store.QueueJoinResult{
		Status:      store.QueueStatusWaiting,
		QueuedCount: 1,
		SwitchedFrom: &store.SwitchedFromQueue{
			GameName: name,
		},
	})
	if result.Message == nil || *result.Message != "You left the group for Rock Paper Scissors to look for a group here." {
		t.Fatalf("message = %+v", result.Message)
	}
}

func TestJoinResultFromQueueJoinAlwaysQueued(t *testing.T) {
	sessionID := uuid.New()
	waiting := joinResultFromQueueJoin(&store.QueueJoinResult{
		Status:      store.QueueStatusWaiting,
		QueuedCount: 1,
	})
	if !waiting.Queued || waiting.JoinURL != nil || waiting.SessionID != nil {
		t.Fatalf("expected waiting-only join result, got %+v", waiting)
	}
	if waiting.QueuedCount == nil || *waiting.QueuedCount != 1 {
		t.Fatalf("expected queuedCount 1, got %+v", waiting.QueuedCount)
	}

	userID := uuid.New()
	urls := map[uuid.UUID]string{userID: "http://localhost:5174/?match=" + sessionID.String() + "&token=t"}
	matched := joinResultAfterJoin(&store.QueueJoinResult{
		Status:      store.QueueStatusMatched,
		SessionID:   &sessionID,
		QueuedCount: 0,
	}, userID, urls)
	if matched.Queued || matched.JoinURL == nil || matched.SessionID == nil {
		t.Fatalf("expected match fields for joiner, got %+v", matched)
	}
}
