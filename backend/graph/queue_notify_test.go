package graph

import (
	"testing"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/graph/model"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

func TestQueueUpdateFromViewMatched(t *testing.T) {
	sessionID := uuid.New()
	gameID := uuid.MustParse("a1000000-0000-4000-8000-000000000001")
	queueID := uuid.New()

	launch := "http://localhost:5174/?match=" + sessionID.String() + "&token=eyJ.test"
	update := queueUpdateFromView(&store.UserQueueView{
		GameID:      gameID,
		ModeQueueID: queueID,
		InQueue:     true,
		Matched:     true,
		SessionID:   &sessionID,
	}, launch)

	if update == nil || update.Status != model.QueueStatusMatched {
		t.Fatalf("expected matched update, got %+v", update)
	}
	if update.JoinURL == nil || *update.JoinURL == "" {
		t.Fatalf("expected join URL on matched snapshot")
	}
}

func TestQueueUpdateFromViewWaiting(t *testing.T) {
	gameID := uuid.MustParse("a1000000-0000-4000-8000-000000000001")
	queueID := uuid.New()
	update := queueUpdateFromView(&store.UserQueueView{
		GameID:      gameID,
		ModeQueueID: queueID,
		InQueue:     true,
		Waiting:     true,
		QueuedCount: 2,
	}, "")

	if update == nil || update.Status != model.QueueStatusWaiting || update.QueuedCount != 2 {
		t.Fatalf("expected waiting update with count 2, got %+v", update)
	}
}
