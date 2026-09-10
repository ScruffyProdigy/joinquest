package graph

import (
	"context"
	"errors"
	"fmt"
	"log"

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

	// Both publishes are best-effort: the join — and the switch that preceded it —
	// are committed by the time either runs, so failing the mutation here would
	// report an error for a queue entry that exists. Log and let the player have
	// their join; the reconcile loop re-publishes the count shortly after.
	if result.SwitchedFrom != nil {
		if err := r.publishQueueSwitchFrom(ctx, st, result.SwitchedFrom, userID); err != nil {
			log.Printf("join queue: publish switch-from for %s: %v", userID, err)
		}
	}

	if err := r.publishQueueResult(ctx, result, nil); err != nil {
		log.Printf("join queue: publish result for queue %s: %v", result.ModeQueueID, err)
	}

	r.scheduleFormingReconcile(result.ModeQueueID)

	return joinResultFromQueueJoin(result), nil
}
