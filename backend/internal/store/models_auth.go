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
