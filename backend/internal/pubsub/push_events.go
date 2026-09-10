package pubsub

import (
	"context"
	"encoding/json"
)

// Push delivery outcomes, published so a seat hold can react to a failed send
// instead of waiting out its ceiling.
//
// An event rather than a return value because a push service only reveals a
// dead subscription on an actual send, after the hold has already begun.

// PushDeliveryStatus is the outcome of trying to reach one user.
type PushDeliveryStatus string

const (
	// PushDelivered -- at least one install accepted the push.
	PushDelivered PushDeliveryStatus = "DELIVERED"
	// PushUndeliverable -- every install was permanently rejected and marked
	// expired. The user is now known unreachable.
	PushUndeliverable PushDeliveryStatus = "UNDELIVERABLE"
	// PushFailed -- transient failure (5xx, timeout). Nothing was delivered,
	// but subscriptions are kept: this says nothing about reachability.
	PushFailed PushDeliveryStatus = "FAILED"
	// PushSkipped -- nothing attempted: no live subscription, no VAPID keys, or
	// no TTL on the notification.
	PushSkipped PushDeliveryStatus = "SKIPPED"
)

// PushDeliveryEvent reports the result of a push attempt for one user.
type PushDeliveryEvent struct {
	UserID string             `json:"userId"`
	Status PushDeliveryStatus `json:"status"`
	// HoldID ties the outcome to the hold it belongs to, so a late event is
	// discardable. Not a session id: the hold runs before the match is
	// announced, so no session exists yet.
	HoldID string `json:"holdId,omitempty"`
	// Attempted and Delivered count installs, so a partial success is visible.
	Attempted int `json:"attempted"`
	Delivered int `json:"delivered"`
	// Expired counts installs marked dead by this attempt.
	Expired int `json:"expired,omitempty"`
}

// UserPushChannel returns the channel for a user's push delivery outcomes.
func UserPushChannel(userID string) string {
	return "lobby:user:" + userID + ":push"
}

// MarshalPushDeliveryEvent encodes a delivery outcome.
func MarshalPushDeliveryEvent(event PushDeliveryEvent) ([]byte, error) {
	return json.Marshal(event)
}

// UnmarshalPushDeliveryEvent decodes a delivery outcome.
func UnmarshalPushDeliveryEvent(payload []byte) (PushDeliveryEvent, error) {
	var event PushDeliveryEvent
	err := json.Unmarshal(payload, &event)
	return event, err
}

// PublishPushDelivery publishes a delivery outcome for one user.
func PublishPushDelivery(ctx context.Context, broker Broker, event PushDeliveryEvent) error {
	if broker == nil {
		return nil
	}
	payload, err := MarshalPushDeliveryEvent(event)
	if err != nil {
		return err
	}
	channel := UserPushChannel(event.UserID)
	DebugLog(
		"publish push outcome user=%s channel=%s status=%s attempted=%d delivered=%d expired=%d",
		event.UserID, channel, event.Status, event.Attempted, event.Delivered, event.Expired,
	)
	return broker.Publish(ctx, channel, payload)
}
