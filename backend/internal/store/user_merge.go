package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// RandomGuestDisplayName returns guest#NNNNNN with a random 6-digit suffix.
func RandomGuestDisplayName() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("guest#%06d", n.Int64()+100000), nil
}

func (s *Store) CreateGuestUser(ctx context.Context) (*User, error) {
	displayName, err := RandomGuestDisplayName()
	if err != nil {
		return nil, err
	}

	for i := 0; i < 8; i++ {
		username, err := s.uniqueGuestUsername(ctx)
		if err != nil {
			return nil, err
		}
		row := s.db.QueryRowContext(ctx, `
			INSERT INTO users (username, display_name, is_guest)
			VALUES ($1, $2, true)
			RETURNING `+userColumns+`
		`, username, displayName)
		user, err := scanUser(row)
		if err == nil {
			return user, nil
		}
		if isUniqueViolation(err) {
			displayName, err = RandomGuestDisplayName()
			if err != nil {
				return nil, err
			}
			continue
		}
		return nil, err
	}
	return nil, fmt.Errorf("store: failed to create guest user")
}

func (s *Store) uniqueGuestUsername(ctx context.Context) (string, error) {
	for i := 0; i < 5; i++ {
		suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
		candidate := fmt.Sprintf("guest_%s", suffix)
		var exists bool
		if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`, candidate).Scan(&exists); err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("store: failed to generate unique guest username")
}

func mergedUserEmail(userID uuid.UUID) string {
	return fmt.Sprintf("merged+%s@inactive.local", strings.ReplaceAll(userID.String(), "-", ""))
}

// MergeUserInto moves sign-in methods and transferable state from source into target, then deactivates source.
func (s *Store) MergeUserInto(ctx context.Context, sourceID, targetID uuid.UUID) error {
	if sourceID == targetID {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var sourceActive bool
	if err := tx.QueryRowContext(ctx, `
		SELECT is_active FROM users WHERE id = $1
	`, sourceID).Scan(&sourceActive); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if !sourceActive {
		return nil
	}

	sourceEmails, err := listUserEmailsTx(ctx, tx, sourceID)
	if err != nil {
		return err
	}
	targetEmails, err := listUserEmailsTx(ctx, tx, targetID)
	if err != nil {
		return err
	}
	targetEmailSet := make(map[string]struct{}, len(targetEmails))
	targetHasPrimary := false
	for _, item := range targetEmails {
		targetEmailSet[item.Email] = struct{}{}
		if item.IsPrimary {
			targetHasPrimary = true
		}
	}

	for _, item := range sourceEmails {
		if _, exists := targetEmailSet[item.Email]; exists {
			if _, err := tx.ExecContext(ctx, `DELETE FROM user_emails WHERE id = $1`, item.ID); err != nil {
				return err
			}
			continue
		}
		makePrimary := item.IsPrimary && !targetHasPrimary
		if makePrimary {
			if _, err := tx.ExecContext(ctx, `UPDATE user_emails SET is_primary = false WHERE user_id = $1`, targetID); err != nil {
				return err
			}
			targetHasPrimary = true
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE user_emails
			SET user_id = $2, is_primary = $3
			WHERE id = $1
		`, item.ID, targetID, makePrimary); err != nil {
			return err
		}
		if makePrimary {
			if _, err := tx.ExecContext(ctx, `
				UPDATE users SET email = NULL, updated_at = NOW() WHERE id = $1
			`, sourceID); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE users SET email = $2, updated_at = NOW() WHERE id = $1
			`, targetID, item.Email); err != nil {
				return err
			}
		}
	}

	sourceIdentities, err := listUserIdentitiesTx(ctx, tx, sourceID)
	if err != nil {
		return err
	}
	for _, item := range sourceIdentities {
		if _, err := tx.ExecContext(ctx, `
			UPDATE user_identities
			SET user_id = $2
			WHERE id = $1
		`, item.ID, targetID); err != nil {
			if isUniqueViolation(err) {
				if _, err := tx.ExecContext(ctx, `DELETE FROM user_identities WHERE id = $1`, item.ID); err != nil {
					return err
				}
				continue
			}
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE user_inventory
		SET user_id = $2
		WHERE user_id = $1
		  AND NOT EXISTS (
			SELECT 1 FROM user_inventory existing
			WHERE existing.user_id = $2 AND existing.good_id = user_inventory.good_id
		  )
	`, sourceID, targetID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_inventory WHERE user_id = $1`, sourceID); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET is_active = false,
		    merged_into_user_id = $2,
		    email = $3,
		    updated_at = NOW()
		WHERE id = $1
	`, sourceID, targetID, mergedUserEmail(sourceID)); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET is_guest = false,
		    updated_at = NOW()
		WHERE id = $1
	`, targetID); err != nil {
		return err
	}

	return tx.Commit()
}

func listUserEmailsTx(ctx context.Context, tx *sql.Tx, userID uuid.UUID) ([]UserEmail, error) {
	rows, err := tx.QueryContext(ctx, `
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

func listUserIdentitiesTx(ctx context.Context, tx *sql.Tx, userID uuid.UUID) ([]UserIdentity, error) {
	rows, err := tx.QueryContext(ctx, `
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

func isIdentityUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505" && pqErr.Constraint == "user_identities_provider_subject_unique"
}
