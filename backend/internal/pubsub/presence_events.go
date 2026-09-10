package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
)

type PresenceStatus string

const (
	// PresenceStatusConnected is the 0 -> 1 edge: the player came back. Consumers
	// use it to restore state for a returning player without polling.
	PresenceStatusConnected PresenceStatus = "CONNECTED"
	// PresenceStatusDisconnected is the 1 -> 0 edge: the player's last socket closed
	// and any grace window is now running.
	PresenceStatusDisconnected PresenceStatus = "DISCONNECTED"
)

// PresenceEvent is published on per-user channels when a player's presence changes.
//
// Only edges are published. A second tab opening, or one of three sockets closing,
// is not a presence change — the player's socket count moved but the player did not.
//
// This carries "disconnected" (socket gone) and never "away" (socket alive, attention
// gone), which is a different signal with a different owner.
type PresenceEvent struct {
	Status PresenceStatus `json:"status"`
	// DisconnectedAt is RFC3339 and set only on DISCONNECTED. Consumers get the raw
	// timestamp rather than a staleness boolean, so each applies its own window:
	// ctx.Done() is a fast positive signal and a slow negative one, and a consumer
	// reasoning about that needs the time, not somebody else's verdict on it.
	DisconnectedAt string `json:"disconnectedAt,omitempty"`
}

// UserPresenceChannel returns the channel for a user's presence edges.
func UserPresenceChannel(userID string) string {
	return "lobby:user:" + userID + ":presence"
}

func MarshalPresenceEvent(event PresenceEvent) ([]byte, error) {
	return json.Marshal(event)
}

func UnmarshalPresenceEvent(payload []byte) (PresenceEvent, error) {
	var event PresenceEvent
	err := json.Unmarshal(payload, &event)
	return event, err
}

// PublishPresenceEvent marshals and publishes a presence edge to the user's channel.
func PublishPresenceEvent(ctx context.Context, broker Broker, userID string, event PresenceEvent) error {
	if broker == nil {
		return fmt.Errorf("pubsub: broker is required")
	}
	payload, err := MarshalPresenceEvent(event)
	if err != nil {
		return err
	}
	channel := UserPresenceChannel(userID)
	DebugLog(
		"publish presence user=%s channel=%s status=%s disconnectedAt=%s",
		userID,
		channel,
		event.Status,
		event.DisconnectedAt,
	)
	return broker.Publish(ctx, channel, payload)
}
