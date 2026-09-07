package graph

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/auth"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

// ErrIdentityRequired rejects a caller who has a session but has not finished
// picking an identity. It is deliberately worded apart from "authentication
// required" so the frontend can raise the identity prompt instead of a sign-in
// error — keep it in sync with isIdentityRequiredError in
// frontend/src/lib/graphql.js.
var ErrIdentityRequired = errors.New("identity required")

// requireIdentityUserID resolves the caller and rejects anyone who has not
// chosen both a name and an avatar. Mutations that put a player into play — a
// queue, a room, a seat — use this in place of requireAuthUserID, which a
// nameless, faceless guest satisfies. Being a guest is fine; being unknown is
// not.
func (r *Resolver) requireIdentityUserID(ctx context.Context) (uuid.UUID, error) {
	userID, err := requireAuthUserID(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	st, err := r.requireStore()
	if err != nil {
		return uuid.Nil, err
	}
	user, err := st.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return uuid.Nil, fmt.Errorf("authentication required")
		}
		return uuid.Nil, err
	}
	if !user.HasChosenIdentity() {
		return uuid.Nil, ErrIdentityRequired
	}
	return userID, nil
}

func requireNonGuestAccount(ctx context.Context, authService *auth.Service) (*auth.Service, uuid.UUID, error) {
	user, err := authService.RequireNonGuestUser(ctx)
	if err != nil {
		return nil, uuid.Nil, err
	}
	return authService, user.ID, nil
}

func requireAuthenticatedUserID(ctx context.Context) (uuid.UUID, error) {
	userID, err := requireAuthUserID(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("authentication required")
	}
	return userID, nil
}

func mapAuthProvider(provider string) (string, error) {
	switch provider {
	case "google":
		return "GOOGLE", nil
	case "discord":
		return "DISCORD", nil
	case "apple":
		return "APPLE", nil
	case "facebook":
		return "FACEBOOK", nil
	default:
		return "", fmt.Errorf("unknown auth provider %q", provider)
	}
}
