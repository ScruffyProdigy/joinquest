// Package push delivers Web Push notifications.
//
// It answers only "did the push service accept this". Deciding whether a player
// is worth notifying belongs to the seat-hold window.
package push

import (
	"context"
	"errors"
	"time"
)

// Notification is a match-ready ping. It carries no game state -- the payload
// crosses an untrusted push service.
type Notification struct {
	// Title and Body are what the OS renders.
	Title string
	Body  string
	// URL is where a tap lands the player: the launch step, not a fresh queue.
	URL string
	// Tag collapses repeats, so a re-push replaces rather than stacks.
	Tag string
	// TTL is how long the push service keeps trying. Must not outlive the seat.
	TTL time.Duration
}

// Subscription is one browser install's push endpoint and keys.
type Subscription struct {
	Endpoint string
	P256dh   string
	Auth     string
}

// ErrSubscriptionGone means the push service permanently rejected this endpoint
// (HTTP 404 or 410). The caller must stop counting it as reachable.
var ErrSubscriptionGone = errors.New("push: subscription gone")

// Sender delivers a notification to one browser install.
type Sender interface {
	Send(ctx context.Context, sub Subscription, note Notification) error
	// PublicKey is the VAPID key the browser needs to subscribe. Empty when
	// push is not configured, which the frontend reads as "hide the control".
	PublicKey() string
}
