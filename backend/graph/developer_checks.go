package graph

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/graph/model"
	"github.com/scruffyprodigy/joinquest/internal/auth"
	"github.com/scruffyprodigy/joinquest/internal/integrationchecks"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

func (r *Resolver) checkRunner() *integrationchecks.Runner {
	return integrationchecks.NewRunner(r.manifestFetcher())
}

func (r *Resolver) persistGameChecks(ctx context.Context, game *store.Game) ([]*model.GameIntegrationCheck, error) {
	st, err := r.requireStore()
	if err != nil {
		return nil, err
	}
	runner := r.checkRunner()
	all := runner.RunManifestChecks(ctx, game)

	modes, err := st.ListGameModesByGameID(ctx, game.ID)
	if err != nil {
		return nil, err
	}
	seatsByMode := make(map[uuid.UUID][]store.GameModeSeat, len(modes))
	for _, mode := range modes {
		seats, listErr := st.ListGameModeSeats(ctx, mode.ID)
		if listErr != nil {
			return nil, listErr
		}
		seatsByMode[mode.ID] = seats
	}

	lobby, err := lobbyProvisionInfo(game)
	if err != nil {
		return nil, err
	}
	token := lobby.ServiceToken
	provisionOutcome := runner.RunProvisionChecks(ctx, game, modes, seatsByMode, r.gameProvisioner(), integrationchecks.ProvisionConfig{
		LobbyID:      auth.LobbyIssuer(),
		Lobby:        lobby,
		ServiceToken: token,
	})
	all = append(all, provisionOutcome.Results...)

	authService, err := r.requireAuth()
	if err != nil {
		return nil, err
	}
	if provisionOutcome.MatchID != "" && provisionOutcome.SeatKey != "" {
		all = append(all, runner.RunJWTChecks(ctx, game, authService.Signer(), provisionOutcome.MatchID, provisionOutcome.SeatKey, provisionOutcome.SecondSeatKey)...)
	} else {
		all = append(all, integrationchecks.JWTSkippedNoProvision()...)
	}

	return r.storeCheckResults(ctx, game.ID, all)
}

func (r *Resolver) storeCheckResults(ctx context.Context, gameID uuid.UUID, checks []integrationchecks.Result) ([]*model.GameIntegrationCheck, error) {
	st, err := r.requireStore()
	if err != nil {
		return nil, err
	}
	out := make([]*model.GameIntegrationCheck, 0, len(checks))
	for _, check := range checks {
		var message *string
		if check.Message != "" {
			msg := check.Message
			message = &msg
		}
		stored, err := st.UpsertIntegrationCheck(ctx, gameID, check.CheckID, check.Status, message, check.Detail)
		if err != nil {
			return nil, err
		}
		out = append(out, ToGraphQLIntegrationCheck(stored))
	}
	return out, nil
}

func (r *Resolver) ownedGame(ctx context.Context, gameID string) (*store.Game, error) {
	userID, err := requireAuthUserID(ctx)
	if err != nil {
		return nil, err
	}
	st, err := r.requireStore()
	if err != nil {
		return nil, err
	}
	id, err := parseUUID(gameID, "game id")
	if err != nil {
		return nil, err
	}
	game, err := st.GetOwnedGame(ctx, id, userID)
	if err != nil {
		if err == store.ErrNotFound {
			return nil, fmt.Errorf("game not found")
		}
		return nil, err
	}
	return game, nil
}
