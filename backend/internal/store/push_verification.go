package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

/*
Verification: the difference between a browser that says it can be reached and
one that has been.

A saved subscription is a claim. The browser hands over an endpoint and two
keys, and every one of them can be stale, wrong, or bound to a push service
that will silently drop what we send. The only way to know is to send something
and hear it come back.

So opt-in sends a push carrying a single-use token, the service worker acks it,
and the row is stamped. JQ-216 reads that stamp at the moment a queued player's
socket drops -- when the client is by definition gone and cannot be asked
anything.
*/

// ErrPushVerificationNotFound means the token was unknown, already redeemed, or
// past its window. All three are the same answer to the caller: this ack proves
// nothing.
var ErrPushVerificationNotFound = errors.New("store: push verification not found")

// PushVerificationWindow is how long a token stays redeemable.
//
// The ack is a round trip through a push service, not a human decision, so this
// is generous rather than long: it covers a slow service, not a player who
// wandered off. A token that outlives the press it belongs to would let a much
// older push verify a subscription whose keys have since moved on.
const PushVerificationWindow = 10 * time.Minute

// StartPushSubscriptionVerification mints a fresh token for one of a user's
// subscriptions and clears any existing proof.
//
// Clearing matters: from here until the ack lands the subscription is
// unverified, so a verification that never completes cannot leave the player
// looking reachable on the strength of a previous round trip.
//
// User-scoped, so one account cannot start verification on another's endpoint
// and learn whether it exists.
func (s *Store) StartPushSubscriptionVerification(ctx context.Context, userID uuid.UUID, endpoint string) (*PushSubscription, string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, "", ErrInvalidPushSubscription
	}

	token, err := newPushVerificationToken()
	if err != nil {
		return nil, "", err
	}

	query := `
		UPDATE push_subscriptions
		SET verification_token = $3,
		    verification_sent_at = NOW(),
		    verified_at = NULL
		WHERE user_id = $1 AND endpoint = $2 AND expired_at IS NULL
		RETURNING ` + pushSubscriptionColumns

	var out PushSubscription
	err = scanPushSubscription(s.db.QueryRowContext(ctx, query, userID, endpoint, token), &out)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", ErrPushVerificationNotFound
	}
	if err != nil {
		return nil, "", err
	}
	return &out, token, nil
}

// ConfirmPushSubscriptionVerification redeems a token, stamping the
// subscription verified. Returns the owning user.
//
// The token is the credential: only the browser the push reached has it. That
// is why this needs no session -- at ack time the page may already be gone, and
// requiring one would make the proof unobtainable in exactly the case it is
// meant to cover.
//
// Single-use. The token is cleared in the same statement, so a replayed ack
// finds nothing.
func (s *Store) ConfirmPushSubscriptionVerification(ctx context.Context, token string) (uuid.UUID, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return uuid.Nil, ErrPushVerificationNotFound
	}

	const query = `
		UPDATE push_subscriptions
		SET verified_at = NOW(),
		    verification_token = NULL
		WHERE verification_token = $1
		  AND expired_at IS NULL
		  AND verification_sent_at > NOW() - $2::interval
		RETURNING user_id`

	var userID uuid.UUID
	err := s.db.QueryRowContext(ctx, query, token, PushVerificationWindow.String()).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, ErrPushVerificationNotFound
	}
	if err != nil {
		return uuid.Nil, err
	}
	return userID, nil
}

// HasVerifiedPushSubscription reports whether a user has at least one install
// a push has actually reached.
//
// Device state only -- it cannot see whether this deployment has VAPID keys, so
// it is not the full answer. Use graph.PushReachability.
func (s *Store) HasVerifiedPushSubscription(ctx context.Context, userID uuid.UUID) (bool, error) {
	const query = `SELECT EXISTS (
		SELECT 1 FROM push_subscriptions
		WHERE user_id = $1 AND expired_at IS NULL AND verified_at IS NOT NULL
	)`
	var exists bool
	if err := s.db.QueryRowContext(ctx, query, userID).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

// newPushVerificationToken returns an unguessable single-use token. 32 bytes,
// because it stands in for a session on the ack path.
func newPushVerificationToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
