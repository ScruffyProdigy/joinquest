package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/lfg"
	"github.com/scruffyprodigy/joinquest/internal/seattemplate"
)

func scanFormingMatch(row interface{ Scan(dest ...any) error }) (*FormingMatch, error) {
	var fm FormingMatch
	var firedAt sql.NullTime
	if err := row.Scan(
		&fm.ID, &fm.ModeQueueID, &fm.ModeID, &fm.GameID, &fm.Status, &fm.TargetSpec,
		&fm.CreatedAt, &firedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if firedAt.Valid {
		fm.FiredAt = &firedAt.Time
	}
	return &fm, nil
}

func scanFormingAssignment(row interface{ Scan(dest ...any) error }) (*FormingAssignment, error) {
	var a FormingAssignment
	var userID, partyID, affinity, tableID sql.NullString
	if err := row.Scan(
		&a.ID, &a.FormingMatchID, &a.SeatKey, &userID, &partyID,
		&a.QueuePath, &affinity, &a.Source, &tableID, &a.AssignedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if userID.Valid {
		id, err := uuid.Parse(userID.String)
		if err != nil {
			return nil, err
		}
		a.UserID = &id
	}
	if partyID.Valid {
		id, err := uuid.Parse(partyID.String)
		if err != nil {
			return nil, err
		}
		a.PartyID = &id
	}
	if affinity.Valid {
		a.AffinityKey = &affinity.String
	}
	if tableID.Valid {
		id, err := uuid.Parse(tableID.String)
		if err != nil {
			return nil, err
		}
		a.TableID = &id
	}
	return &a, nil
}

// GetOrCreateFillingFormingMatchTx returns the active forming match for a queue, creating one if needed.
func (s *Store) GetOrCreateFillingFormingMatchTx(
	ctx context.Context,
	tx *sql.Tx,
	joinCtx *ModeQueueJoinContext,
	matchSeats []GameModeSeat,
) (*FormingMatch, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, mode_queue_id, mode_id, game_id, status, target_spec, created_at, fired_at
		FROM forming_matches
		WHERE mode_queue_id = $1 AND status = $2
	`, joinCtx.ModeQueue.ID, FormingMatchStatusFilling)
	if fm, err := scanFormingMatch(row); err == nil {
		return fm, nil
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	specs, err := seattemplate.PathSpecs(joinCtx.Mode.SeatTemplate)
	if err != nil {
		return nil, err
	}
	specJSON, err := json.Marshal(specs)
	if err != nil {
		return nil, err
	}

	row = tx.QueryRowContext(ctx, `
		INSERT INTO forming_matches (mode_queue_id, mode_id, game_id, status, target_spec)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, mode_queue_id, mode_id, game_id, status, target_spec, created_at, fired_at
	`, joinCtx.ModeQueue.ID, joinCtx.Mode.ID, joinCtx.Game.ID, FormingMatchStatusFilling, specJSON)
	fm, err := scanFormingMatch(row)
	if err != nil {
		return nil, err
	}
	if err := seedFormingAssignmentsFromSeatsTx(ctx, tx, fm.ID, matchSeats); err != nil {
		return nil, err
	}
	return fm, nil
}

func seedFormingAssignmentsFromSeatsTx(ctx context.Context, tx *sql.Tx, formingMatchID uuid.UUID, seats []GameModeSeat) error {
	for _, seat := range seats {
		queuePath := seatQueuePathValue(seat)
		affinity := optionalString(seat.AffinityKey)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO forming_match_assignments (
				forming_match_id, seat_key, queue_path, affinity_key, source
			) VALUES ($1, $2, $3, $4, 'solo')
			ON CONFLICT (forming_match_id, seat_key) DO NOTHING
		`, formingMatchID, seat.SeatKey, queuePathColumnValue(queuePath), affinity); err != nil {
			return err
		}
	}
	return nil
}

// ListFormingAssignmentsTx returns all seat cells for a forming match.
func (s *Store) ListFormingAssignmentsTx(ctx context.Context, tx *sql.Tx, formingMatchID uuid.UUID) ([]FormingAssignment, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, forming_match_id, seat_key, user_id, party_id,
		       queue_path, affinity_key, source, table_id, assigned_at
		FROM forming_match_assignments
		WHERE forming_match_id = $1
		ORDER BY seat_key ASC
	`, formingMatchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []FormingAssignment
	for rows.Next() {
		var a FormingAssignment
		var userID, partyID, affinity, tableID sql.NullString
		if err := rows.Scan(
			&a.ID, &a.FormingMatchID, &a.SeatKey, &userID, &partyID,
			&a.QueuePath, &affinity, &a.Source, &tableID, &a.AssignedAt,
		); err != nil {
			return nil, err
		}
		if userID.Valid {
			id, err := uuid.Parse(userID.String)
			if err != nil {
				return nil, err
			}
			a.UserID = &id
		}
		if partyID.Valid {
			id, err := uuid.Parse(partyID.String)
			if err != nil {
				return nil, err
			}
			a.PartyID = &id
		}
		if affinity.Valid {
			a.AffinityKey = &affinity.String
		}
		if tableID.Valid {
			id, err := uuid.Parse(tableID.String)
			if err != nil {
				return nil, err
			}
			a.TableID = &id
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// FormingPathGapsTx computes queue-path gaps for a forming match.
func (s *Store) FormingPathGapsTx(ctx context.Context, tx *sql.Tx, fm *FormingMatch) ([]lfg.PathGap, error) {
	var specs []seattemplate.PathSpec
	if len(fm.TargetSpec) > 0 {
		if err := json.Unmarshal(fm.TargetSpec, &specs); err != nil {
			return nil, fmt.Errorf("store: decode forming target_spec: %w", err)
		}
	}
	assignments, err := s.ListFormingAssignmentsTx(ctx, tx, fm.ID)
	if err != nil {
		return nil, err
	}
	return lfg.ComputePathGaps(specs, formingAssignmentsToLFG(assignments)), nil
}

// MarkFormingMatchFiredTx sets forming match status to fired.
func (s *Store) MarkFormingMatchFiredTx(ctx context.Context, tx *sql.Tx, formingMatchID uuid.UUID, at time.Time) error {
	result, err := tx.ExecContext(ctx, `
		UPDATE forming_matches
		SET status = $2, fired_at = $3
		WHERE id = $1 AND status = $4
	`, formingMatchID, FormingMatchStatusFired, at, FormingMatchStatusFilling)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result, ErrNotFound)
}

func formingAssignmentsToLFG(assignments []FormingAssignment) []lfg.Assignment {
	out := make([]lfg.Assignment, len(assignments))
	for i, a := range assignments {
		userID := ""
		if a.UserID != nil {
			userID = a.UserID.String()
		}
		aff := ""
		if a.AffinityKey != nil {
			aff = *a.AffinityKey
		}
		out[i] = lfg.Assignment{
			SeatKey:     a.SeatKey,
			UserID:      userID,
			QueuePath:   a.QueuePath,
			AffinityKey: aff,
		}
	}
	return out
}
