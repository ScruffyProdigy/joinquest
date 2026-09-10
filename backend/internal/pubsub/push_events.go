package pubsub

import (
	"context"
	"encoding/json"
)

/*
Push delivery outcomes (JQ-198), published so the seat-hold window (JQ-199) can
react to a failed send instead of waiting out a ceiling for a player who will
never be told.

Why this is an event and not a return value: push services only report a dead
subscription (HTTP 410 Gone) on an actual send. So reachability is a prediction
until the moment we try, and JQ-199's tier is necessarily chosen before the
truth is known. The hold starts optimistically on a stored subscription, and
this event is how it learns it was wrong -- promptly, rather than by expiring a
2 minute ceiling that was never going to pay off.
*/

// PushDeliveryStatus is the outcome of trying to reach one user.
type PushDeliveryStatus string

const (
	// PushDelivered -- at least one of the user's installs accepted the push.
	// The optimistic tier was right.
	PushDelivered PushDeliveryStatus = "DELIVERED"
	// PushUndeliverable -- every install was permanently rejected, and they
	// have been marked expired. The user is now known unreachable, and a hold
	// taken on the belief that they were reachable should collapse to its
	// floor.
	PushUndeliverable PushDeliveryStatus = "UNDELIVERABLE"
	// PushFailed -- the send failed for a transient reason (push service 5xx,
	// timeout). Subscriptions are retained, but nothing was delivered.
	//
	// Deliberately distinct from UNDELIVERABLE: a transient failure says
	// nothing about whether the player can be reached, only that this attempt
	// did not land. Collapsing the two would evict players over a push-service
	// hiccup.
	PushFailed PushDeliveryStatus = "FAILED"
	// PushSkipped -- nothing was attempted, because the user had no live
	// subscription or the deployment cannot send. The hold should never have
	// been in the optimistic tier.
	PushSkipped PushDeliveryStatus = "SKIPPED"
)

// PushDeliveryEvent reports the result of a push attempt for one user.
//
// Published on the user's existing queue channel rather than a new one: JQ-199
// and JQ-216 already subscribe there, and a second channel would mean a second
// subscription to keep alive for the same subject.
type PushDeliveryEvent struct {
	UserID string             `json:"userId"`
	Status PushDeliveryStatus `json:"status"`
	// SessionID ties the outcome to the match whose seat is being held, so a
	// late event for a previous match is discardable.
	SessionID string `json:"sessionId,omitempty"`
	// Attempted and Delivered count installs, so a partial success (phone
	// dead, desktop fine) is visible rather than flattened.
	Attempted int `json:"attempted"`
	Delivered int `json:"delivered"`
	// Expired counts installs marked dead by this attempt. Non-zero means the
	// stored reachability was stale.
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
//
// Best-effort by contract: the caller is inside a match-formation path, and a
// broker hiccup must never fail a match that formed correctly.
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
