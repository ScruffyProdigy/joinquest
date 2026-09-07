package pubsub

import (
	"context"
	"encoding/json"
)

const (
	MatchEventPlayerFinished = "player_finished"
	MatchEventResult         = "result"
	MatchEventRegroup        = "regroup"
)

// MatchEvent is published on a match channel when a result or regroup state changes.
type MatchEvent struct {
	Type    string `json:"type"`
	MatchID string `json:"matchId"`
}

// MatchChannel returns the Redis channel for a match's result updates.
func MatchChannel(sessionID string) string {
	return "lobby:match:" + sessionID
}

func MarshalMatchEvent(event MatchEvent) ([]byte, error) {
	return json.Marshal(event)
}

func UnmarshalMatchEvent(payload []byte) (MatchEvent, error) {
	var event MatchEvent
	err := json.Unmarshal(payload, &event)
	return event, err
}

// PublishMatchEvent marshals and publishes a match event.
func PublishMatchEvent(ctx context.Context, broker Broker, sessionID string, event MatchEvent) error {
	if broker == nil {
		return nil
	}
	event.MatchID = sessionID
	payload, err := MarshalMatchEvent(event)
	if err != nil {
		return err
	}
	return broker.Publish(ctx, MatchChannel(sessionID), payload)
}
