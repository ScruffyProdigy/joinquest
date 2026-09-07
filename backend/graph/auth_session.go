package graph

import (
	"context"

	"github.com/scruffyprodigy/joinquest/graph/model"
	"github.com/scruffyprodigy/joinquest/internal/auth"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

func finishSignIn(ctx context.Context, authService *auth.Service, user *store.User, sessionToken string) (*model.User, error) {
	if writer, ok := auth.ResponseWriterFromContext(ctx); ok {
		auth.SetSessionCookie(writer, sessionToken, authService.CookieConfig())
	}
	return ToGraphQLUser(user), nil
}
