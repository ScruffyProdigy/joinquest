package store

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID       uuid.UUID
	Email    string
	Username string
	// Nil until the player picks one. Render-time code substitutes a fallback.
	DisplayName  *string
	AvatarURL    *string
	AvatarKey    *string
	AvatarSource *string
	IsGuest      bool
	CreatedAt    time.Time
}

// ChosenDisplayName returns the name the player picked, or "" if they have not
// picked one. Callers that need something to show should fall back themselves.
func (u *User) ChosenDisplayName() string {
	if u == nil || u.DisplayName == nil {
		return ""
	}
	return strings.TrimSpace(*u.DisplayName)
}

// HasChosenAvatar reports whether the player picked a face. A spirit animal
// clears avatar_key and stores a URL instead, so all three fields count.
// Mirrors hasChosenAvatar in frontend/src/lib/viewer.js.
func (u *User) HasChosenAvatar() bool {
	if u == nil {
		return false
	}
	if u.AvatarKey != nil && strings.TrimSpace(*u.AvatarKey) != "" {
		return true
	}
	if u.AvatarURL != nil && strings.TrimSpace(*u.AvatarURL) != "" {
		return true
	}
	return u.AvatarSource != nil && *u.AvatarSource == SourceSpiritAnimal
}

// HasChosenIdentity is the single gate for entering play: a player is known
// once they have both a name and a face. Mirrors needsIdentity in
// frontend/src/lib/viewer.js, inverted.
func (u *User) HasChosenIdentity() bool {
	return u.ChosenDisplayName() != "" && u.HasChosenAvatar()
}

type CreateUserParams struct {
	Email       string
	DisplayName string
}

type MagicLink struct {
	ID             uuid.UUID
	UserID         *uuid.UUID
	Email          string
	TokenHash      string
	FailedAttempts int
	CodeHash       string
	ExpiresAt      time.Time
	UsedAt         *time.Time
	CreatedAt      time.Time
}

type CreateMagicLinkParams struct {
	Email     string
	UserID    *uuid.UUID
	TokenHash string
	CodeHash  string
	ExpiresAt time.Time
}
