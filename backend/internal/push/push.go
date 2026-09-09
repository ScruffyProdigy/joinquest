// Package push delivers Web Push notifications to players who have left the
// page while queued.
//
// The ticket (JQ-198) is explicit that a notification is only worth sending if
// the seat is still there when the player comes back, so nothing here decides
// *whether* to notify -- that is the seat-hold window's call (JQ-199). This
// package answers only "can we reach this install, and did the push service
// accept it".
package push

import (
	"context"
	"errors"
	"time"
)

// Notification is a match-ready ping. Kept deliberately small: the payload
// crosses an untrusted push service, so it carries no game state, only enough
// to render the notification and route the tap.
type Notification struct {
	// Title and Body are what the OS renders.
	Title string
	Body  string
	// URL is where a tap lands the player. Per the acceptance criteria this
	// must reach the launch step with their seat intact, never a fresh queue.
	URL string
	// Tag collapses repeats: a second notification for the same match replaces
	// the first rather than stacking. Without it a player who is pushed, then
	// re-pushed on a retry, gets a pile of identical alerts.
	Tag string
	// TTL is how long the push service should keep trying. There is no point
	// outliving the seat-hold window -- a notification delivered after the seat
	// is gone is the exact failure the ticket calls worse than sending nothing.
	TTL time.Duration
}

// Subscription is one browser install's push endpoint and keys.
type Subscription struct {
	Endpoint string
	P256dh   string
	Auth     string
}

// ErrSubscriptionGone reports that the push service has permanently rejected
// this endpoint (HTTP 404 or 410). The caller must delete the stored
// subscription: keeping it would make the reachability check lie, and the whole
// seat-hold tiering keys off that check being honest.
var ErrSubscriptionGone = errors.New("push: subscription gone")

// Sender delivers a notification to one browser install.
type Sender interface {
	Send(ctx context.Context, sub Subscription, note Notification) error
	// PublicKey is the VAPID application server key the browser needs at
	// subscribe time. Empty when push is not configured, which is the signal
	// the frontend uses to hide the affordance entirely rather than offering a
	// control that cannot work.
	PublicKey() string
}
