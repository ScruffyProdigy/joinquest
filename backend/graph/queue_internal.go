package graph

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/graph/model"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

func (r *mutationResolver) joinQueueInternal(ctx context.Context, modeQueueID uuid.UUID, queuePath string, optionsInput []*model.QueueOptionSelectionInput, partyInput *model.PartyNodeInput) (*model.JoinResult, error) {
	st, err := r.requireStore()
	if err != nil {
		return nil, err
	}

	userID, err := r.requireIdentityUserID(ctx)
	if err != nil {
		return nil, err
	}

	party, err := toStorePartyInput(ctx, userID, partyInput)
	if err != nil {
		return nil, err
	}

	game, mode, err := r.modeForQueue(ctx, modeQueueID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("queue not found")
		}
		return nil, err
	}
	options, err := r.resolveSelections(ctx, game, mode, userID.String(), optionsInput)
	if err != nil {
		return nil, err
	}

	result, err := st.JoinModeQueueWithOptions(ctx, modeQueueID, userID, queuePath, options, party)
	if err != nil {
		if errors.Is(err, store.ErrAlreadyMatched) {
			return nil, fmt.Errorf("you already have an active match; finish or leave your current queue first")
		}
		if errors.Is(err, store.ErrActiveGame) {
			return nil, fmt.Errorf("you have a game in progress — use Leave game at the top of the page")
		}
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("queue not found")
		}
		return nil, err
	}

	if result.SwitchedFrom != nil {
		if err := r.publishQueueSwitchFrom(ctx, st, result.SwitchedFrom, userID); err != nil {
			return nil, err
		}
	}

	if err := r.publishQueueResult(ctx, result, nil); err != nil {
		return nil, err
	}

	r.scheduleFormingReconcile(result.ModeQueueID)

	return joinResultFromQueueJoin(result), nil
}
