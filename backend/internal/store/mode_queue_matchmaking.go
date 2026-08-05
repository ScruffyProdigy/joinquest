package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ModeQueueJoinContext is everything needed to match players in a mode queue.
type ModeQueueJoinContext struct {
	Game      *Game
	Mode      *GameMode
	ModeQueue *ModeQueue
	Seats     []GameModeSeat
}

// GetModeQueueByID loads a mode queue and validates it is joinable.
func (s *Store) GetModeQueueByID(ctx context.Context, modeQueueID uuid.UUID) (*ModeQueue, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, mode_id, name, players_to_start, status, is_default, created_at, updated_at
		FROM mode_queues
		WHERE id = $1
	`, modeQueueID)
	q, err := scanModeQueueRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return q, nil
}

// GetDefaultModeQueueForGame returns the default active queue for a catalog game.
func (s *Store) GetDefaultModeQueueForGame(ctx context.Context, gameID uuid.UUID) (*ModeQueue, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT mq.id, mq.mode_id, mq.name, mq.players_to_start, mq.status, mq.is_default, mq.created_at, mq.updated_at
		FROM mode_queues mq
		INNER JOIN game_modes gm ON gm.id = mq.mode_id
		WHERE gm.game_id = $1
		  AND gm.status = 'active'
		  AND mq.status = 'active'
		  AND mq.is_default = true
		ORDER BY gm.mode_key ASC
		LIMIT 1
	`, gameID)
	q, err := scanModeQueueRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return q, nil
}

func (s *Store) loadModeQueueJoinContext(ctx context.Context, q sqlQueryRowContext, modeQueueID uuid.UUID) (*ModeQueueJoinContext, error) {
	modeQueue, err := func() (*ModeQueue, error) {
		row := q.QueryRowContext(ctx, `
			SELECT id, mode_id, name, players_to_start, status, is_default, created_at, updated_at
			FROM mode_queues
			WHERE id = $1
		`, modeQueueID)
		return scanModeQueueRow(row)
	}()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if modeQueue.Status != ModeQueueActive {
		return nil, fmt.Errorf("store: queue is not active")
	}

	mode, err := getGameModeByID(ctx, q, modeQueue.ModeID)
	if err != nil {
		return nil, err
	}
	if mode.Status != ModeStatusActive {
		return nil, fmt.Errorf("store: game mode is not active")
	}

	game, err := getGameForMatchmaking(ctx, q, mode.GameID)
	if err != nil {
		return nil, err
	}

	seats, err := listGameModeSeats(ctx, q, mode.ID)
	if err != nil {
		return nil, err
	}
	if len(seats) == 0 {
		return nil, fmt.Errorf("store: mode %q has no seats configured", mode.ModeKey)
	}

	return &ModeQueueJoinContext{
		Game:      game,
		Mode:      mode,
		ModeQueue: modeQueue,
		Seats:     seats,
	}, nil
}

// GetGameModeByID loads a catalog mode by id.
func (s *Store) GetGameModeByID(ctx context.Context, modeID uuid.UUID) (*GameMode, error) {
	return getGameModeByID(ctx, s.db, modeID)
}

func getGameModeByID(ctx context.Context, q sqlQueryRowContext, modeID uuid.UUID) (*GameMode, error) {
	row := q.QueryRowContext(ctx, `
		SELECT id, game_id, mode_key, display_name, min_players, max_players, seat_template, status, created_at, updated_at
		FROM game_modes
		WHERE id = $1
	`, modeID)
	return scanGameModeRow(row)
}

// JoinModeQueue enqueues the user in a mode queue and starts a session when full.
func (s *Store) JoinModeQueue(ctx context.Context, modeQueueID, userID uuid.UUID, queuePath string, party *JoinPartyInput) (*QueueJoinResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, _, err := s.leaveTableSeatTx(ctx, tx, userID); err != nil {
		return nil, err
	}
	if err := finishReturnedQueueSessionsForRequeueTx(ctx, tx, userID); err != nil {
		return nil, err
	}
	if err := ensureNotInActiveGameTx(ctx, tx, userID); err != nil {
		return nil, err
	}
	if err := s.reconcileStaleMatchedQueuesForUserTx(ctx, tx, userID); err != nil {
		return nil, err
	}

	joinCtx, err := s.loadModeQueueJoinContext(ctx, tx, modeQueueID)
	if err != nil {
		return nil, err
	}
	if err := validateJoinQueuePath(joinCtx.Seats, queuePath); err != nil {
		return nil, err
	}

	return s.joinModeQueueFormingTx(ctx, tx, joinCtx, modeQueueID, userID, queuePath, party)
}

func optionalQueuePathRef(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// LeaveModeQueue removes the user from a mode queue's waiting or matched state.
func (s *Store) LeaveModeQueue(ctx context.Context, modeQueueID, userID uuid.UUID) (int, error) {
	if _, err := s.GetModeQueueByID(ctx, modeQueueID); err != nil {
		return 0, err
	}

	_ = s.leaveModeQueueWaiting(ctx, modeQueueID, userID)
	_ = s.cancelUserMatchedModeQueue(ctx, modeQueueID, userID)
	_ = s.reconcileStaleMatchedQueuesForUser(ctx, userID)
	return s.CountWaitingInModeQueue(ctx, modeQueueID)
}

func (s *Store) leaveModeQueueWaiting(ctx context.Context, modeQueueID, userID uuid.UUID) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE game_queues
		SET status = 'cancelled'
		WHERE mode_queue_id = $1 AND user_id = $2 AND status = 'waiting'
	`, modeQueueID, userID)
	if err != nil {
		return err
	}
	if err := s.reconcileStalePartiesForUser(ctx, userID); err != nil {
		return err
	}
	return ensureRowsAffected(result, ErrNotFound)
}

type enqueueOutcome struct {
	alreadyInQueue bool
	switchedFrom   *SwitchedFromQueue
}

func enqueueModeQueueTx(ctx context.Context, tx *sql.Tx, gameID, modeQueueID, userID uuid.UUID, queuePath string, partyID *uuid.UUID) (enqueueOutcome, error) {
	var out enqueueOutcome
	if _, err := getMatchedModeQueueEntryTx(ctx, tx, modeQueueID, userID); err == nil {
		return out, ErrAlreadyMatched
	} else if !errors.Is(err, ErrNotFound) {
		return out, err
	}

	if other, err := getUserMatchedModeQueueIDTx(ctx, tx, userID, modeQueueID); err != nil {
		return out, err
	} else if other != nil {
		return out, ErrAlreadyMatched
	}

	if prev, err := getUserWaitingQueueSwitchInfoTx(ctx, tx, userID, modeQueueID); err != nil {
		return out, err
	} else if prev != nil {
		out.switchedFrom = prev
	}

	// Legacy rows (mode_queue_id NULL) or waiting rows on another queue.
	if _, err := tx.ExecContext(ctx, `
		UPDATE game_queues
		SET status = 'cancelled'
		WHERE user_id = $1 AND status = 'waiting'
		  AND (mode_queue_id IS NULL OR mode_queue_id <> $2)
	`, userID, modeQueueID); err != nil {
		return out, err
	}

	row := tx.QueryRowContext(ctx, `
		SELECT `+queueColumns+`
		FROM game_queues
		WHERE mode_queue_id = $1 AND user_id = $2 AND status = 'waiting'
		ORDER BY joined_at DESC
		LIMIT 1
	`, modeQueueID, userID)
	if existing, err := scanQueueEntry(row); err == nil {
		if sameQueuePath(existing.QueuePath, queuePath) {
			out.alreadyInQueue = true
			return out, nil
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE game_queues
			SET queue_path = $2, joined_at = NOW(), party_id = $3
			WHERE id = $1 AND status = 'waiting'
		`, existing.ID, nullQueuePathColumn(queuePath), nullUUIDColumn(partyID)); err != nil {
			return out, err
		}
		return out, nil
	} else if !errors.Is(err, ErrNotFound) {
		return out, err
	}

	if _, err := tx.ExecContext(ctx, "SAVEPOINT gq_enqueue"); err != nil {
		return out, err
	}
	row = tx.QueryRowContext(ctx, `
		INSERT INTO game_queues (game_id, user_id, status, mode_queue_id, queue_path, party_id)
		VALUES ($1, $2, 'waiting', $3, $4, $5)
		RETURNING `+queueColumns+`
	`, gameID, userID, modeQueueID, nullQueuePathColumn(queuePath), nullUUIDColumn(partyID))
	if _, err := scanQueueEntry(row); err != nil {
		if isUniqueViolation(err) {
			if _, rbErr := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT gq_enqueue"); rbErr != nil {
				return out, err
			}
			row = tx.QueryRowContext(ctx, `
				SELECT `+queueColumns+`
				FROM game_queues
				WHERE game_id = $1 AND user_id = $2 AND status = 'waiting'
				ORDER BY joined_at DESC
				LIMIT 1
			`, gameID, userID)
			if _, err := scanQueueEntry(row); err != nil {
				return out, err
			}
			out.alreadyInQueue = true
			return out, nil
		}
		return out, err
	}
	if _, err := tx.ExecContext(ctx, "RELEASE SAVEPOINT gq_enqueue"); err != nil {
		return out, err
	}
	return out, nil
}

func getMatchedModeQueueEntryTx(ctx context.Context, tx *sql.Tx, modeQueueID, userID uuid.UUID) (*QueueEntry, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT `+queueColumns+`
		FROM game_queues
		WHERE mode_queue_id = $1 AND user_id = $2 AND status = 'matched'
		ORDER BY matched_at DESC NULLS LAST, joined_at DESC
		LIMIT 1
	`, modeQueueID, userID)
	return scanQueueEntry(row)
}

func listWaitingModeQueueEntriesTx(ctx context.Context, tx *sql.Tx, modeQueueID uuid.UUID) ([]QueueEntry, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT `+queueColumns+`
		FROM game_queues
		WHERE mode_queue_id = $1 AND status = 'waiting'
		ORDER BY joined_at ASC
		FOR UPDATE
	`, modeQueueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []QueueEntry
	for rows.Next() {
		entry, err := scanQueueEntryRow(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, *entry)
	}
	return entries, rows.Err()
}

func createModeQueueSessionTx(ctx context.Context, tx *sql.Tx, gameID, modeID, modeQueueID uuid.UUID) (*Session, error) {
	row := tx.QueryRowContext(ctx, `
		INSERT INTO game_sessions (game_id, status, mode_id, mode_queue_id)
		VALUES ($1, 'active', $2, $3)
		RETURNING id, game_id, mode_id, mode_queue_id, status, started_at, ended_at
	`, gameID, modeID, modeQueueID)
	return scanSessionRow(row)
}

// GetGameModeForSession returns the catalog mode for a session when present.
func (s *Store) GetGameModeForSession(ctx context.Context, sessionID uuid.UUID) (*GameMode, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT gm.id, gm.game_id, gm.mode_key, gm.display_name, gm.min_players, gm.max_players, gm.status, gm.created_at, gm.updated_at
		FROM game_sessions gs
		INNER JOIN game_modes gm ON gm.id = gs.mode_id
		WHERE gs.id = $1
	`, sessionID)
	var mode GameMode
	if err := row.Scan(
		&mode.ID, &mode.GameID, &mode.ModeKey, &mode.DisplayName,
		&mode.MinPlayers, &mode.MaxPlayers, &mode.Status,
		&mode.CreatedAt, &mode.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &mode, nil
}
