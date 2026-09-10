package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/lfg"
)

// FormingReconcileResult is returned when the forming worker evaluates a mode queue.
type FormingReconcileResult struct {
	GameID        uuid.UUID
	ModeQueueID   uuid.UUID
	Fired         bool
	SessionID     *uuid.UUID
	NotifyUserIDs []uuid.UUID
	TableIDs      []uuid.UUID
	QueuedCount   int
}

// ListModeQueuesNeedingReconcile returns mode queues with waiting players or an open forming match.
func (s *Store) ListModeQueuesNeedingReconcile(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT mode_queue_id
		FROM game_queues
		WHERE status = 'waiting' AND mode_queue_id IS NOT NULL
		UNION
		SELECT mode_queue_id
		FROM forming_matches
		WHERE status = $1
	`, FormingMatchStatusFilling)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ReconcileFormingModeQueue syncs waiting parties onto the forming map and fires when ready.
func (s *Store) ReconcileFormingModeQueue(ctx context.Context, modeQueueID uuid.UUID) (*FormingReconcileResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, modeQueueID.String()); err != nil {
		return nil, err
	}

	waiting, err := listWaitingModeQueueEntriesTx(ctx, tx, modeQueueID)
	if err != nil {
		return nil, err
	}
	if len(waiting) == 0 {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return &FormingReconcileResult{ModeQueueID: modeQueueID, QueuedCount: 0}, nil
	}

	joinCtx, err := s.loadModeQueueJoinContext(ctx, tx, modeQueueID)
	if err != nil {
		return nil, err
	}

	matchSeats, err := matchSeatsFromTemplate(joinCtx.Seats, joinCtx.Mode.SeatTemplate)
	if err != nil {
		return nil, err
	}
	if len(matchSeats) == 0 {
		matchSeats = joinCtx.Seats
	}

	fm, err := s.GetOrCreateFillingFormingMatchTx(ctx, tx, joinCtx, matchSeats)
	if err != nil {
		return nil, err
	}

	if err := s.syncWaitingPartiesOnFormingTx(ctx, tx, fm, modeQueueID); err != nil {
		return nil, err
	}

	gaps, err := s.FormingPathGapsTx(ctx, tx, fm)
	if err != nil {
		return nil, err
	}

	if lfg.ReadyToFire(gaps) {
		// The map being full is now the first of two terms, not the whole fire
		// condition. The second asks whether this is a lobby worth firing or
		// whether waiting could still buy a better one -- and answers "fire" on
		// its own whenever waiting cannot pay, which is every thin queue and
		// every mode that has opted out.
		decision, err := s.skillFireDecisionTx(ctx, tx, joinCtx, fm, waiting, time.Now())
		if err != nil {
			return nil, err
		}
		if !decision.Fire {
			// Spend the choice the deferral bought. Holding accumulates
			// candidates in the waiting pool, but placement is greedy and
			// nothing moves a player already seated -- so without this the
			// budget expires on the identical lobby and the wait bought
			// nothing.
			improved, err := s.improveDeferredLobbyTx(ctx, tx, joinCtx, fm)
			if err != nil {
				return nil, err
			}
			if improved {
				// Ask again rather than waiting a tick. The deferral is an
				// upper bound, never a delay spent for its own sake: a lobby
				// that has just become good enough should fire now.
				decision, err = s.skillFireDecisionTx(ctx, tx, joinCtx, fm, waiting, time.Now())
				if err != nil {
					return nil, err
				}
			}
		}
		if !decision.Fire {
			// Not an error and not a stall. The map keeps its players, nobody
			// has been told a match formed, and the next tick asks again with a
			// smaller budget -- so this can only repeat until the budget runs
			// out, at which point the same call fires regardless of dispersion.
			// Fall through to the not-ready path, which commits any swap above
			// and reports the queue as still forming.
			gaps = nil
		}
	}

	if lfg.ReadyToFire(gaps) {
		fired, err := s.fireFormingMatchTx(ctx, tx, joinCtx, fm)
		if err != nil {
			return nil, err
		}
		// A nil result with no error means the fire declined: an assigned player had
		// no waiting row left, so their seat was vacated instead. Fall through to the
		// not-yet-ready path, which commits that vacate and reports the queue as
		// still forming — the next reconcile refills the seat from the waiting pool.
		if fired != nil {
			if err := tx.Commit(); err != nil {
				return nil, err
			}
			return &FormingReconcileResult{
				GameID:        joinCtx.Game.ID,
				ModeQueueID:   modeQueueID,
				Fired:         true,
				SessionID:     &fired.session.ID,
				NotifyUserIDs: fired.notifyIDs,
				TableIDs:      fired.tableIDs,
			}, nil
		}
	}

	waiting, err = listWaitingModeQueueEntriesTx(ctx, tx, modeQueueID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &FormingReconcileResult{
		GameID:      joinCtx.Game.ID,
		ModeQueueID: modeQueueID,
		QueuedCount: len(waiting),
	}, nil
}
