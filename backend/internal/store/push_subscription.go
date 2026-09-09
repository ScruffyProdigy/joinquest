package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrInvalidPushSubscription is returned when the browser hands us a
// subscription missing the endpoint or either key -- all three are required to
// encrypt a payload, so a partial one is not storable.
var ErrInvalidPushSubscription = errors.New("store: invalid push subscription")

// PushSubscription is one browser install that has granted notification
// permission and can be reached by Web Push.
type PushSubscription struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Endpoint   string
	P256dh     string
	Auth       string
	UserAgent  string
	CreatedAt  time.Time
	LastUsedAt *time.Time
}

// SavePushSubscriptionParams is the browser's PushSubscription, flattened.
type SavePushSubscriptionParams struct {
	UserID    uuid.UUID
	Endpoint  string
	P256dh    string
	Auth      string
	UserAgent string
}

// SavePushSubscription records a browser install as reachable for a user.
//
// Conflict is on endpoint rather than (user_id, endpoint) deliberately: the
// endpoint identifies the browser, not the account. When a shared or
// re-signed-in browser subscribes again, the row must move to the new user
// rather than leaving the previous owner able to receive that device's
// notifications.
func (s *Store) SavePushSubscription(ctx context.Context, params SavePushSubscriptionParams) (*PushSubscription, error) {
	endpoint := strings.TrimSpace(params.Endpoint)
	p256dh := strings.TrimSpace(params.P256dh)
	auth := strings.TrimSpace(params.Auth)
	if endpoint == "" || p256dh == "" || auth == "" {
		return nil, ErrInvalidPushSubscription
	}

	const query = `
		INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth, user_agent)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (endpoint) DO UPDATE
		SET user_id = EXCLUDED.user_id,
		    p256dh = EXCLUDED.p256dh,
		    auth = EXCLUDED.auth,
		    user_agent = EXCLUDED.user_agent
		RETURNING id, user_id, endpoint, p256dh, auth, user_agent, created_at, last_used_at`

	var out PushSubscription
	err := s.db.QueryRowContext(ctx, query,
		params.UserID, endpoint, p256dh, auth, strings.TrimSpace(params.UserAgent),
	).Scan(
		&out.ID, &out.UserID, &out.Endpoint, &out.P256dh, &out.Auth,
		&out.UserAgent, &out.CreatedAt, &out.LastUsedAt,
	)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ListPushSubscriptions returns every install currently reachable for a user.
func (s *Store) ListPushSubscriptions(ctx context.Context, userID uuid.UUID) ([]PushSubscription, error) {
	const query = `
		SELECT id, user_id, endpoint, p256dh, auth, user_agent, created_at, last_used_at
		FROM push_subscriptions
		WHERE user_id = $1
		ORDER BY created_at`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PushSubscription
	for rows.Next() {
		var sub PushSubscription
		if err := rows.Scan(
			&sub.ID, &sub.UserID, &sub.Endpoint, &sub.P256dh, &sub.Auth,
			&sub.UserAgent, &sub.CreatedAt, &sub.LastUsedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}

// HasPushSubscription reports whether a user has at least one reachable install.
//
// This is the capability check JQ-198's acceptance criteria gate on: whether
// staying queued while away from the waiting page can actually be honoured.
// It is deliberately a stored-subscription check rather than a
// permission-was-granted-once check -- a granted permission on a platform that
// cannot deliver, or a subscription the push service has since expired, is not
// reachability.
func (s *Store) HasPushSubscription(ctx context.Context, userID uuid.UUID) (bool, error) {
	const query = `SELECT EXISTS (SELECT 1 FROM push_subscriptions WHERE user_id = $1)`
	var exists bool
	if err := s.db.QueryRowContext(ctx, query, userID).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

// DeletePushSubscription removes a single install, by endpoint.
//
// Scoped to the user so one account cannot unsubscribe another's device by
// guessing an endpoint.
func (s *Store) DeletePushSubscription(ctx context.Context, userID uuid.UUID, endpoint string) error {
	const query = `DELETE FROM push_subscriptions WHERE user_id = $1 AND endpoint = $2`
	_, err := s.db.ExecContext(ctx, query, userID, strings.TrimSpace(endpoint))
	return err
}

// DeleteExpiredPushSubscription removes an install the push service has
// rejected as gone (HTTP 404/410), regardless of owner.
//
// Not user-scoped, because the rejection comes from the push service against an
// endpoint and the send path should not have to trust its own idea of who owns
// it. Keeping a dead endpoint would make HasPushSubscription lie, and the
// ticket is explicit that a hold taken on false reachability is worse than none.
func (s *Store) DeleteExpiredPushSubscription(ctx context.Context, endpoint string) error {
	const query = `DELETE FROM push_subscriptions WHERE endpoint = $1`
	_, err := s.db.ExecContext(ctx, query, strings.TrimSpace(endpoint))
	return err
}

// DeletePushSubscriptionsForUser drops every install for a user. Called on
// logout: the next person to use this browser must not inherit the last one's
// match notifications.
func (s *Store) DeletePushSubscriptionsForUser(ctx context.Context, userID uuid.UUID) error {
	const query = `DELETE FROM push_subscriptions WHERE user_id = $1`
	_, err := s.db.ExecContext(ctx, query, userID)
	return err
}

// TouchPushSubscription records a successful send, which is what separates a
// subscription that still works from one that merely exists.
func (s *Store) TouchPushSubscription(ctx context.Context, endpoint string) error {
	const query = `UPDATE push_subscriptions SET last_used_at = NOW() WHERE endpoint = $1`
	_, err := s.db.ExecContext(ctx, query, strings.TrimSpace(endpoint))
	return err
}
