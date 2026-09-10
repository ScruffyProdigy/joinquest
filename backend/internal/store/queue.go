package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/google/uuid"
)

const queueColumns = `id, game_id, user_id, status, joined_at, mode_queue_id, queue_path, party_id, forming_match_id, queue_options`

// queueColumnsGQ is queueColumns bound to the `gq` alias, for the queries that join
// another table and would otherwise be ambiguous on shared column names (user_id).
//
// Derived from queueColumns rather than written out again: the two lists must stay in
// the same order forever, because scanQueueEntry reads them positionally, and a second
// hand-maintained copy is exactly how that goes wrong silently.
var queueColumnsGQ = "gq." + strings.ReplaceAll(queueColumns, ", ", ", gq.")

func scanQueueEntry(row interface{ Scan(dest ...any) error }) (*QueueEntry, error) {
	var entry QueueEntry
	var modeQueueID, queuePath, partyID, formingMatchID sql.NullString
	var queueOptions []byte
	if err := row.Scan(
		&entry.ID, &entry.GameID, &entry.UserID, &entry.Status, &entry.JoinedAt,
		&modeQueueID, &queuePath, &partyID, &formingMatchID, &queueOptions,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	selections, err := decodeQueueOptions(queueOptions)
	if err != nil {
		return nil, err
	}
	entry.QueueOptions = selections
	if modeQueueID.Valid {
		id, err := uuid.Parse(modeQueueID.String)
		if err != nil {
			return nil, err
		}
		entry.ModeQueueID = &id
	}
	if queuePath.Valid {
		entry.QueuePath = &queuePath.String
	}
	if partyID.Valid {
		id, err := uuid.Parse(partyID.String)
		if err != nil {
			return nil, err
		}
		entry.PartyID = &id
	}
	if formingMatchID.Valid {
		id, err := uuid.Parse(formingMatchID.String)
		if err != nil {
			return nil, err
		}
		entry.FormingMatchID = &id
	}
	return &entry, nil
}

func scanQueueEntryRow(rows *sql.Rows) (*QueueEntry, error) {
	var entry QueueEntry
	var modeQueueID, queuePath, partyID, formingMatchID sql.NullString
	var queueOptions []byte
	if err := rows.Scan(
		&entry.ID, &entry.GameID, &entry.UserID, &entry.Status, &entry.JoinedAt,
		&modeQueueID, &queuePath, &partyID, &formingMatchID, &queueOptions,
	); err != nil {
		return nil, err
	}
	selections, err := decodeQueueOptions(queueOptions)
	if err != nil {
		return nil, err
	}
	entry.QueueOptions = selections
	if modeQueueID.Valid {
		id, err := uuid.Parse(modeQueueID.String)
		if err != nil {
			return nil, err
		}
		entry.ModeQueueID = &id
	}
	if queuePath.Valid {
		entry.QueuePath = &queuePath.String
	}
	if partyID.Valid {
		id, err := uuid.Parse(partyID.String)
		if err != nil {
			return nil, err
		}
		entry.PartyID = &id
	}
	if formingMatchID.Valid {
		id, err := uuid.Parse(formingMatchID.String)
		if err != nil {
			return nil, err
		}
		entry.FormingMatchID = &id
	}
	return &entry, nil
}

func (s *Store) GetWaitingModeQueueEntry(ctx context.Context, modeQueueID, userID uuid.UUID) (*QueueEntry, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+queueColumns+`
		FROM game_queues
		WHERE mode_queue_id = $1 AND user_id = $2 AND status = 'waiting'
		ORDER BY joined_at DESC
		LIMIT 1
	`, modeQueueID, userID)
	return scanQueueEntry(row)
}
