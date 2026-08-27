package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

var ErrIdentityAlreadyLinked = errors.New("store: identity is already linked to another account")
var ErrIdentityLinkBlocked = errors.New("store: identity link blocked by database lock")

type UserIdentity struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Provider   string
	Subject    string
	Email      *string
	VerifiedAt time.Time
	CreatedAt  time.Time
}

const userIdentityColumns = `id, user_id, provider, subject, email, verified_at, created_at`

func scanUserIdentity(row interface{ Scan(dest ...any) error }) (*UserIdentity, error) {
	var item UserIdentity
	var email sql.NullString
	if err := row.Scan(
		&item.ID,
		&item.UserID,
		&item.Provider,
		&item.Subject,
		&email,
		&item.VerifiedAt,
		&item.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if email.Valid {
		item.Email = &email.String
	}
	return &item, nil
}

func (s *Store) ListUserIdentities(ctx context.Context, userID uuid.UUID) ([]UserIdentity, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+userIdentityColumns+`
		FROM user_identities
		WHERE user_id = $1
		ORDER BY created_at ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []UserIdentity
	for rows.Next() {
		var item UserIdentity
		var email sql.NullString
		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.Provider,
			&item.Subject,
			&email,
			&item.VerifiedAt,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		if email.Valid {
			item.Email = &email.String
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetUserIdentityByProviderSubject(ctx context.Context, provider, subject string) (*UserIdentity, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	subject = strings.TrimSpace(subject)
	if provider == "" || subject == "" {
		return nil, ErrNotFound
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT `+userIdentityColumns+`
		FROM user_identities
		WHERE provider = $1 AND subject = $2
	`, provider, subject)
	return scanUserIdentity(row)
}

func (s *Store) CreateUserIdentity(ctx context.Context, userID uuid.UUID, provider, subject string, email *string) (*UserIdentity, error) {
	return s.linkOAuthIdentity(ctx, userID, provider, subject, email, false)
}

// LinkOAuthIdentity links an OAuth provider to a user with short lock/statement timeouts.
func (s *Store) LinkOAuthIdentity(ctx context.Context, userID uuid.UUID, provider, subject string, email *string) (*UserIdentity, error) {
	return s.linkOAuthIdentity(ctx, userID, provider, subject, email, true)
}

func (s *Store) linkOAuthIdentity(ctx context.Context, userID uuid.UUID, provider, subject string, email *string, enforceTimeouts bool) (*UserIdentity, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	subject = strings.TrimSpace(subject)
	if provider == "" || subject == "" {
		return nil, fmt.Errorf("store: provider and subject are required")
	}

	var emailValue sql.NullString
	if email != nil {
		normalized := strings.ToLower(strings.TrimSpace(*email))
		if normalized != "" {
			emailValue = sql.NullString{String: normalized, Valid: true}
		}
	}

	run := func(exec interface {
		ExecContext(context.Context, string, ...any) (sql.Result, error)
		QueryRowContext(context.Context, string, ...any) *sql.Row
	}) (*UserIdentity, error) {
		row := exec.QueryRowContext(ctx, `
			INSERT INTO user_identities (user_id, provider, subject, email, verified_at)
			VALUES ($1, $2, $3, $4, NOW())
			RETURNING `+userIdentityColumns+`
		`, userID, provider, subject, emailValue)
		item, err := scanUserIdentity(row)
		if err != nil {
			if isIdentityUniqueViolation(err) {
				return nil, ErrIdentityAlreadyLinked
			}
			if enforceTimeouts && isLockOrTimeout(err) {
				return nil, ErrIdentityLinkBlocked
			}
			return nil, err
		}

		if _, err := exec.ExecContext(ctx, `
			UPDATE users SET is_guest = false, updated_at = NOW() WHERE id = $1
		`, userID); err != nil {
			if enforceTimeouts && isLockOrTimeout(err) {
				return nil, ErrIdentityLinkBlocked
			}
			return nil, err
		}
		return item, nil
	}

	if !enforceTimeouts {
		return run(s.db)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, "SET LOCAL lock_timeout = '2s'"); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, "SET LOCAL statement_timeout = '5s'"); err != nil {
		return nil, err
	}

	item, err := run(tx)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		if isLockOrTimeout(err) {
			return nil, ErrIdentityLinkBlocked
		}
		return nil, err
	}
	return item, nil
}

func isLockOrTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		switch pqErr.Code {
		case "55P03", "57014": // lock_not_available, query_canceled
			return true
		}
	}
	return false
}

func (s *Store) CompleteOAuthSignUp(ctx context.Context, provider, subject, displayName string, email *string) (*User, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	subject = strings.TrimSpace(subject)
	// Providers do not always give us a name; leave it null rather than invent one.
	var name *string
	if trimmed := strings.TrimSpace(displayName); trimmed != "" {
		name = &trimmed
	}
	if provider == "" || subject == "" {
		return nil, fmt.Errorf("store: provider and subject are required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, "SET LOCAL lock_timeout = '3s'"); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, "SET LOCAL statement_timeout = '8s'"); err != nil {
		return nil, err
	}

	username := "oauth_" + strings.ReplaceAll(uuid.NewString(), "-", "")

	var emailValue sql.NullString
	if email != nil {
		normalized := strings.ToLower(strings.TrimSpace(*email))
		if normalized != "" {
			emailValue = sql.NullString{String: normalized, Valid: true}
		}
	}

	row := tx.QueryRowContext(ctx, `
		INSERT INTO users (username, display_name, is_guest)
		VALUES ($1, $2, false)
		RETURNING `+userColumns+`
	`, username, name)
	user, err := scanUser(row)
	if err != nil {
		if isLockOrTimeout(err) {
			return nil, ErrIdentityLinkBlocked
		}
		return nil, err
	}

	identityRow := tx.QueryRowContext(ctx, `
		INSERT INTO user_identities (user_id, provider, subject, email, verified_at)
		VALUES ($1, $2, $3, $4, NOW())
		RETURNING `+userIdentityColumns+`
	`, user.ID, provider, subject, emailValue)
	if _, err := scanUserIdentity(identityRow); err != nil {
		if isIdentityUniqueViolation(err) {
			return nil, ErrIdentityAlreadyLinked
		}
		if isLockOrTimeout(err) {
			return nil, ErrIdentityLinkBlocked
		}
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		if isLockOrTimeout(err) {
			return nil, ErrIdentityLinkBlocked
		}
		return nil, err
	}
	return user, nil
}

func (s *Store) GetUserIdentityByID(ctx context.Context, id uuid.UUID) (*UserIdentity, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+userIdentityColumns+`
		FROM user_identities
		WHERE id = $1
	`, id)
	return scanUserIdentity(row)
}

func (s *Store) RemoveUserIdentity(ctx context.Context, userID, identityID uuid.UUID) error {
	emails, err := s.ListUserEmails(ctx, userID)
	if err != nil {
		return err
	}
	identities, err := s.ListUserIdentities(ctx, userID)
	if err != nil {
		return err
	}
	if len(emails)+len(identities) <= 1 {
		return ErrLastSignInMethod
	}

	result, err := s.db.ExecContext(ctx, `
		DELETE FROM user_identities WHERE id = $1 AND user_id = $2
	`, identityID, userID)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result, ErrNotFound)
}
