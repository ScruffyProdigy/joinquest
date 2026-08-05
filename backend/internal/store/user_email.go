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

var (
	ErrEmailAlreadyLinked = errors.New("store: email is already linked to another account")
	ErrLastSignInMethod   = errors.New("store: cannot remove the last sign-in method")
	ErrEmailNotLinked     = errors.New("store: email is not linked to this account")
)

type UserEmail struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Email      string
	IsPrimary  bool
	VerifiedAt time.Time
	CreatedAt  time.Time
}

func scanUserEmail(row interface{ Scan(dest ...any) error }) (*UserEmail, error) {
	var item UserEmail
	if err := row.Scan(
		&item.ID,
		&item.UserID,
		&item.Email,
		&item.IsPrimary,
		&item.VerifiedAt,
		&item.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &item, nil
}

const userEmailColumns = `id, user_id, email, is_primary, verified_at, created_at`

func (s *Store) ListUserEmails(ctx context.Context, userID uuid.UUID) ([]UserEmail, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+userEmailColumns+`
		FROM user_emails
		WHERE user_id = $1
		ORDER BY is_primary DESC, created_at ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []UserEmail
	for rows.Next() {
		var item UserEmail
		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.Email,
			&item.IsPrimary,
			&item.VerifiedAt,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetUserEmailByAddress(ctx context.Context, email string) (*UserEmail, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+userEmailColumns+`
		FROM user_emails
		WHERE email = $1
	`, strings.ToLower(strings.TrimSpace(email)))
	return scanUserEmail(row)
}

func (s *Store) GetUserEmailByID(ctx context.Context, id uuid.UUID) (*UserEmail, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+userEmailColumns+`
		FROM user_emails
		WHERE id = $1
	`, id)
	return scanUserEmail(row)
}

func (s *Store) AddVerifiedUserEmail(ctx context.Context, userID uuid.UUID, email string, makePrimary bool) (*UserEmail, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return nil, fmt.Errorf("store: email is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	existing, err := scanUserEmail(tx.QueryRowContext(ctx, `
		SELECT `+userEmailColumns+`
		FROM user_emails
		WHERE email = $1
	`, normalized))
	if err == nil {
		if existing.UserID != userID {
			return nil, ErrEmailAlreadyLinked
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return existing, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	var hasPrimary bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM user_emails WHERE user_id = $1 AND is_primary = true
		)
	`, userID).Scan(&hasPrimary); err != nil {
		return nil, err
	}
	primary := makePrimary || !hasPrimary

	if primary {
		if _, err := tx.ExecContext(ctx, `
			UPDATE user_emails SET is_primary = false WHERE user_id = $1
		`, userID); err != nil {
			return nil, err
		}
	}

	row := tx.QueryRowContext(ctx, `
		INSERT INTO user_emails (user_id, email, is_primary, verified_at)
		VALUES ($1, $2, $3, NOW())
		RETURNING `+userEmailColumns+`
	`, userID, normalized, primary)
	item, err := scanUserEmail(row)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrEmailAlreadyLinked
		}
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET email = CASE WHEN $2 THEN $3 ELSE email END,
		    is_guest = false,
		    updated_at = NOW()
		WHERE id = $1
	`, userID, primary, normalized); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Store) SetPrimaryUserEmail(ctx context.Context, userID, emailID uuid.UUID) (*UserEmail, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	item, err := scanUserEmail(tx.QueryRowContext(ctx, `
		SELECT `+userEmailColumns+`
		FROM user_emails
		WHERE id = $1 AND user_id = $2
	`, emailID, userID))
	if err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE user_emails SET is_primary = false WHERE user_id = $1
	`, userID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE user_emails SET is_primary = true WHERE id = $1
	`, emailID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE users SET email = $2, updated_at = NOW() WHERE id = $1
	`, userID, item.Email); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	item.IsPrimary = true
	return item, nil
}

func (s *Store) RemoveUserEmail(ctx context.Context, userID, emailID uuid.UUID) error {
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

	var target *UserEmail
	for i := range emails {
		if emails[i].ID == emailID {
			target = &emails[i]
			break
		}
	}
	if target == nil {
		return ErrEmailNotLinked
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM user_emails WHERE id = $1 AND user_id = $2`, emailID, userID); err != nil {
		return err
	}

	if target.IsPrimary {
		var nextEmail sql.NullString
		if err := tx.QueryRowContext(ctx, `
			SELECT email FROM user_emails WHERE user_id = $1 ORDER BY created_at ASC LIMIT 1
		`, userID).Scan(&nextEmail); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if nextEmail.Valid {
			if _, err := tx.ExecContext(ctx, `
				UPDATE user_emails SET is_primary = true
				WHERE user_id = $1 AND email = $2
			`, userID, nextEmail.String); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE users SET email = $2, updated_at = NOW() WHERE id = $1
			`, userID, nextEmail.String); err != nil {
				return err
			}
		} else {
			if _, err := tx.ExecContext(ctx, `
				UPDATE users SET email = NULL, updated_at = NOW() WHERE id = $1
			`, userID); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func (s *Store) CountUserSignInMethods(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT (
			SELECT COUNT(*)::int FROM user_emails WHERE user_id = $1
		) + (
			SELECT COUNT(*)::int FROM user_identities WHERE user_id = $1
		)
	`, userID).Scan(&count)
	return count, err
}

func (s *Store) ResolveUserIDByEmail(ctx context.Context, email string) (uuid.UUID, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	item, err := s.GetUserEmailByAddress(ctx, normalized)
	if err == nil {
		return item.UserID, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return uuid.Nil, err
	}

	var userID uuid.UUID
	err = s.db.QueryRowContext(ctx, `
		SELECT id
		FROM users
		WHERE email = $1 AND is_active = true
	`, normalized).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, ErrNotFound
		}
		return uuid.Nil, err
	}
	return userID, nil
}

func isEmailUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505" && pqErr.Constraint == "user_emails_email_unique"
}
