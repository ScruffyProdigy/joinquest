package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/lfg"
	"github.com/scruffyprodigy/joinquest/internal/seattemplate"
)

// GetFillingFormingMatchByModeQueueID returns the active forming match for a mode queue, if any.
func (s *Store) GetFillingFormingMatchByModeQueueID(ctx context.Context, modeQueueID uuid.UUID) (*FormingMatch, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, mode_queue_id, mode_id, game_id, status, target_spec, created_at, fired_at
		FROM forming_matches
		WHERE mode_queue_id = $1 AND status = $2
	`, modeQueueID, FormingMatchStatusFilling)
	fm, err := scanFormingMatch(row)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return fm, nil
}

// FormingPathGaps loads gap counts for a forming match.
func (s *Store) FormingPathGaps(ctx context.Context, fm *FormingMatch) ([]lfg.PathGap, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	gaps, err := s.FormingPathGapsTx(ctx, tx, fm)
	if err != nil {
		return nil, err
	}
	return gaps, tx.Commit()
}

// TableBackfillActive is true when a filling forming match has assignments tied to this table.
func (s *Store) TableBackfillActive(ctx context.Context, tableID uuid.UUID) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM forming_match_assignments fma
			INNER JOIN forming_matches fm ON fm.id = fma.forming_match_id
			WHERE fma.table_id = $1
			  AND fma.source = 'table'
			  AND fm.status = $2
		)
	`, tableID, FormingMatchStatusFilling).Scan(&exists)
	return exists, err
}

// TableFormingGaps returns role gaps for a table. When backfill is active, gaps come from the
// forming match; otherwise they are computed from currently seated players vs path specs.
func (s *Store) TableFormingGaps(ctx context.Context, tableID uuid.UUID) ([]lfg.PathGap, bool, error) {
	active, err := s.TableBackfillActive(ctx, tableID)
	if err != nil {
		return nil, false, err
	}
	if active {
		fmID, err := s.formingMatchIDForTableBackfill(ctx, tableID)
		if err != nil {
			return nil, false, err
		}
		if fmID == uuid.Nil {
			return nil, false, nil
		}
		fm, err := s.getFormingMatchByID(ctx, fmID)
		if err != nil {
			return nil, false, err
		}
		gaps, err := s.FormingPathGaps(ctx, fm)
		return gaps, true, err
	}
	gaps, err := s.tableSeatedPathGaps(ctx, tableID)
	return gaps, false, err
}

func (s *Store) formingMatchIDForTableBackfill(ctx context.Context, tableID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.db.QueryRowContext(ctx, `
		SELECT fma.forming_match_id
		FROM forming_match_assignments fma
		INNER JOIN forming_matches fm ON fm.id = fma.forming_match_id
		WHERE fma.table_id = $1
		  AND fma.source = 'table'
		  AND fm.status = $2
		LIMIT 1
	`, tableID, FormingMatchStatusFilling).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, nil
		}
		return uuid.Nil, err
	}
	return id, nil
}

func (s *Store) getFormingMatchByID(ctx context.Context, id uuid.UUID) (*FormingMatch, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, mode_queue_id, mode_id, game_id, status, target_spec, created_at, fired_at
		FROM forming_matches
		WHERE id = $1
	`, id)
	return scanFormingMatch(row)
}

func (s *Store) tableSeatedPathGaps(ctx context.Context, tableID uuid.UUID) ([]lfg.PathGap, error) {
	table, err := s.GetRoomTableByID(ctx, tableID)
	if err != nil {
		return nil, err
	}
	mode, err := s.GetGameModeByID(ctx, table.ModeID)
	if err != nil {
		return nil, err
	}
	specs, err := seattemplate.PathSpecs(mode.SeatTemplate)
	if err != nil {
		return nil, err
	}
	if len(specs) == 0 {
		return nil, nil
	}
	modeSeats, err := s.ListGameModeSeats(ctx, table.ModeID)
	if err != nil {
		return nil, err
	}
	seated, err := s.ListTableSeats(ctx, tableID)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]uuid.UUID, len(seated))
	for _, seat := range seated {
		byKey[seat.SeatKey] = seat.UserID
	}
	assignments := make([]lfg.Assignment, len(modeSeats))
	for i, seat := range modeSeats {
		userID := ""
		if uid, ok := byKey[seat.SeatKey]; ok {
			userID = uid.String()
		}
		path := ""
		if seat.QueuePath != nil {
			path = *seat.QueuePath
		}
		aff := ""
		if seat.AffinityKey != nil {
			aff = *seat.AffinityKey
		}
		assignments[i] = lfg.Assignment{
			SeatKey:     seat.SeatKey,
			UserID:      userID,
			QueuePath:   path,
			AffinityKey: aff,
		}
	}
	return lfg.ComputePathGaps(specs, assignments), nil
}
