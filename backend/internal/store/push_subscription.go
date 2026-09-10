package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrInvalidPushSubscription means the endpoint or a key was missing. All
// three are required to encrypt a payload.
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
	// ExpiredAt is set when the push service permanently rejected this
	// endpoint. Expired rows are kept so GetPushReachability can tell
	// "expired" from "never subscribed".
	ExpiredAt *time.Time
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
// The endpoint identifies the browser, not the account, so re-subscribing moves
// the row to the new user rather than leaving the old owner subscribed.
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
		    user_agent = EXCLUDED.user_agent,
		    -- A re-subscribed endpoint is reachable again.
		    expired_at = NULL
		RETURNING id, user_id, endpoint, p256dh, auth, user_agent, created_at, last_used_at, expired_at`

	var out PushSubscription
	err := s.db.QueryRowContext(ctx, query,
		params.UserID, endpoint, p256dh, auth, strings.TrimSpace(params.UserAgent),
	).Scan(
		&out.ID, &out.UserID, &out.Endpoint, &out.P256dh, &out.Auth,
		&out.UserAgent, &out.CreatedAt, &out.LastUsedAt, &out.ExpiredAt,
	)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ListPushSubscriptions returns the installs still deliverable for a user.
// Expired rows are excluded.
func (s *Store) ListPushSubscriptions(ctx context.Context, userID uuid.UUID) ([]PushSubscription, error) {
	const query = `
		SELECT id, user_id, endpoint, p256dh, auth, user_agent, created_at, last_used_at, expired_at
		FROM push_subscriptions
		WHERE user_id = $1 AND expired_at IS NULL
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
			&sub.UserAgent, &sub.CreatedAt, &sub.LastUsedAt, &sub.ExpiredAt,
		); err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}

// HasPushSubscription reports whether a user has at least one live install.
//
// Device state only. It does not know whether this deployment has VAPID keys,
// so it is not the full reachability answer -- use graph.PushReachability.
func (s *Store) HasPushSubscription(ctx context.Context, userID uuid.UUID) (bool, error) {
	const query = `SELECT EXISTS (
		SELECT 1 FROM push_subscriptions WHERE user_id = $1 AND expired_at IS NULL
	)`
	var exists bool
	if err := s.db.QueryRowContext(ctx, query, userID).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

// DeletePushSubscription removes one install. User-scoped, so one account
// cannot unsubscribe another's device by guessing an endpoint.
func (s *Store) DeletePushSubscription(ctx context.Context, userID uuid.UUID, endpoint string) error {
	const query = `DELETE FROM push_subscriptions WHERE user_id = $1 AND endpoint = $2`
	_, err := s.db.ExecContext(ctx, query, userID, strings.TrimSpace(endpoint))
	return err
}

// MarkPushSubscriptionExpired records that the push service permanently
// rejected an endpoint (HTTP 404/410). Not user-scoped: the rejection names an
// endpoint, not an owner.
//
// Marked, not deleted, so the row stops counting as reachable while still
// distinguishing "expired" from "never subscribed".
func (s *Store) MarkPushSubscriptionExpired(ctx context.Context, endpoint string) error {
	const query = `
		UPDATE push_subscriptions
		SET expired_at = NOW()
		WHERE endpoint = $1 AND expired_at IS NULL`
	_, err := s.db.ExecContext(ctx, query, strings.TrimSpace(endpoint))
	return err
}

// DeletePushSubscriptionsForUser drops every install for a user, on logout, so
// the next person to sign in on this browser does not inherit its notifications.
func (s *Store) DeletePushSubscriptionsForUser(ctx context.Context, userID uuid.UUID) error {
	const query = `DELETE FROM push_subscriptions WHERE user_id = $1`
	_, err := s.db.ExecContext(ctx, query, userID)
	return err
}

// TouchPushSubscription records a successful send.
func (s *Store) TouchPushSubscription(ctx context.Context, endpoint string) error {
	const query = `UPDATE push_subscriptions SET last_used_at = NOW() WHERE endpoint = $1`
	_, err := s.db.ExecContext(ctx, query, strings.TrimSpace(endpoint))
	return err
}
