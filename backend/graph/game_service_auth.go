package graph

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/auth"
)

func requireGameServiceAuth(ctx context.Context) error {
	if auth.GameServiceFromContext(ctx) {
		return nil
	}
	if !auth.GameServiceAuthEnabled() {
		return nil
	}
	return fmt.Errorf("game service authentication required")
}

func requireGameServiceForSessionGame(ctx context.Context, sessionGameID uuid.UUID) error {
	if err := requireGameServiceAuth(ctx); err != nil {
		return err
	}
	scopedID, ok := auth.GameServiceGameIDFromContext(ctx)
	if !ok {
		return nil
	}
	if scopedID != sessionGameID.String() {
		return fmt.Errorf("game service token is not authorized for this match")
	}
	return nil
}
