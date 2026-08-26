package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/scruffyprodigy/playhub/internal/avatars"
)

var ErrInvalidAvatarKey = errors.New("store: invalid avatar key")
var ErrInvalidDisplayName = errors.New("store: invalid display name")

const MaxDisplayNameLen = 100

// NormalizeDisplayName trims and validates a player-visible display name.
func NormalizeDisplayName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || len(name) > MaxDisplayNameLen {
		return "", ErrInvalidDisplayName
	}
	return name, nil
}

func scanUser(row interface{ Scan(dest ...any) error }) (*User, error) {
	var u User
	var email sql.NullString
	if err := row.Scan(
		&u.ID,
		&email,
		&u.Username,
		&u.DisplayName,
		&u.AvatarURL,
		&u.AvatarKey,
		&u.AvatarSource,
		&u.IsGuest,
		&u.DisplayNameChosenAt,
		&u.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if email.Valid {
		u.Email = email.String
	}
	return &u, nil
}

const userColumns = `id, email, username, display_name, avatar_url, avatar_key, avatar_source, is_guest, display_name_chosen_at, created_at`

// DefaultDisplayName builds the initial visible name for a new player. It is a
// placeholder until they pick one, which display_name_chosen_at records.
func DefaultDisplayName(email string) string {
	local := strings.Split(strings.ToLower(strings.TrimSpace(email)), "@")[0]
	if local == "" {
		local = "player"
	}
	return local
}

func (s *Store) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+userColumns+`
		FROM users
		WHERE id = $1 AND is_active = true
	`, id)
	return scanUser(row)
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	userID, err := s.ResolveUserIDByEmail(ctx, normalized)
	if err != nil {
		return nil, err
	}
	return s.GetUserByID(ctx, userID)
}

func (s *Store) CreateUser(ctx context.Context, params CreateUserParams) (*User, error) {
	email := strings.ToLower(strings.TrimSpace(params.Email))

	username, err := s.uniqueUsername(ctx, email)
	if err != nil {
		return nil, err
	}

	// A name the caller supplied is one the player chose; a defaulted one is not,
	// and stays unstamped so the identity prompt still asks for it.
	displayName := strings.TrimSpace(params.DisplayName)
	chosen := displayName != ""
	if !chosen {
		displayName = DefaultDisplayName(email)
	}

	row := s.db.QueryRowContext(ctx, `
		INSERT INTO users (email, username, display_name, is_guest, display_name_chosen_at)
		VALUES ($1, $2, $3, false, CASE WHEN $4 THEN NOW() END)
		RETURNING `+userColumns+`
	`, email, username, displayName, chosen)
	user, err := scanUser(row)
	if err != nil {
		return nil, err
	}
	if _, err := s.AddVerifiedUserEmail(ctx, user.ID, email, true); err != nil && !errors.Is(err, ErrEmailAlreadyLinked) {
		return nil, err
	}
	return s.GetUserByID(ctx, user.ID)
}

func (s *Store) UpdateUserLastLogin(ctx context.Context, id uuid.UUID) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE users
		SET last_login_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result, ErrNotFound)
}

func resolveStarterAvatar(avatarKey, publicOrigin string) (key, source, url string, err error) {
	if entry, ok := avatars.StarterByKey(avatarKey); ok {
		return entry.Key, avatars.SourceStarter, avatars.PublicAssetURL(publicOrigin, entry.File), nil
	}
	if entry, ok := avatars.SigilByKey(avatarKey); ok {
		return entry.Key, avatars.SourceSigil, avatars.PublicAssetURL(publicOrigin, entry.File), nil
	}
	return "", "", "", ErrInvalidAvatarKey
}

// UpdateUserProfile sets the player's display name and optionally a starter avatar.
// When avatarKey is empty, existing avatar fields (including spirit animal) are preserved.
func (s *Store) UpdateUserProfile(ctx context.Context, userID uuid.UUID, displayName, avatarKey, publicOrigin string) (*User, error) {
	name, err := NormalizeDisplayName(displayName)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(avatarKey) == "" {
		row := s.db.QueryRowContext(ctx, `
			UPDATE users
			SET display_name = $2, display_name_chosen_at = NOW(), updated_at = NOW()
			WHERE id = $1 AND is_active = true
			RETURNING `+userColumns+`
		`, userID, name)
		return scanUser(row)
	}

	key, source, url, err := resolveStarterAvatar(avatarKey, publicOrigin)
	if err != nil {
		return nil, err
	}

	row := s.db.QueryRowContext(ctx, `
		UPDATE users
		SET display_name = $2, display_name_chosen_at = NOW(), avatar_key = $3, avatar_source = $4, avatar_url = $5, updated_at = NOW()
		WHERE id = $1 AND is_active = true
		RETURNING `+userColumns+`
	`, userID, name, key, source, url)
	return scanUser(row)
}

func (s *Store) uniqueUsername(ctx context.Context, email string) (string, error) {
	base := strings.Split(email, "@")[0]
	base = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, strings.ToLower(base))
	if base == "" {
		base = "user"
	}
	if len(base) > 40 {
		base = base[:40]
	}

	for i := 0; i < 5; i++ {
		suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
		candidate := fmt.Sprintf("%s_%s", base, suffix)

		var exists bool
		if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`, candidate).Scan(&exists); err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("store: failed to generate unique username for %s", email)
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}

func ensureRowsAffected(result sql.Result, notFound error) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return notFound
	}
	return nil
}
