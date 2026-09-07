package graph

import (
	"context"
	"fmt"
	"strings"

	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/scruffyprodigy/joinquest/internal/auth"
)

// WebsocketInitFunc authenticates GraphQL websocket connections using the HTTP
// upgrade request context (session cookie) or connection_init Authorization.
func WebsocketInitFunc(signer *auth.Signer, apiKeys auth.DeveloperAPIKeyVerifier) transport.WebsocketInitFunc {
	return func(ctx context.Context, initPayload transport.InitPayload) (context.Context, *transport.InitPayload, error) {
		if _, ok := auth.UserIDFromContext(ctx); ok {
			return ctx, nil, nil
		}

		token := strings.TrimSpace(initPayload.Authorization())
		if token == "" {
			return ctx, nil, fmt.Errorf("authentication required")
		}
		if strings.HasPrefix(strings.ToLower(token), "bearer ") {
			token = strings.TrimSpace(token[7:])
		}

		if auth.MatchesDeveloperAPIKey(token) && apiKeys != nil {
			if userID, err := apiKeys.VerifyDeveloperAPIKey(ctx, token); err == nil {
				ctx = auth.WithUserID(ctx, userID.String())
				return ctx, nil, nil
			}
		}

		userID, err := signer.VerifyUserToken(token)
		if err != nil {
			return ctx, nil, fmt.Errorf("authentication required")
		}

		ctx = auth.WithUserID(ctx, userID.String())
		return auth.WithSessionToken(ctx, token), nil, nil
	}
}
