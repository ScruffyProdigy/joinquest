package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/lfg/partytree"
)

// StartTableBackfill enqueues seated table players and leaves matchmaking to the forming worker.
func (s *Store) StartTableBackfill(ctx context.Context, tableID, kingUserID, modeQueueID uuid.UUID) (*QueueJoinResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	table, game, _, modeSeats, err := s.loadTableContext(ctx, tx, tableID)
	if err != nil {
		return nil, err
	}
	if table.Status != TableStatusForming {
		return nil, fmt.Errorf("store: table is not forming")
	}
	seated, err := s.listTableSeatsTx(ctx, tx, tableID)
	if err != nil {
		return nil, err
	}
	if len(seated) == 0 {
		return nil, fmt.Errorf("store: table has no seated players")
	}
	king := tableKingUserID(seated)
	if king == nil || *king != kingUserID {
		return nil, fmt.Errorf("store: only the king can start backfill")
	}
	if active, err := s.TableBackfillActive(ctx, tableID); err != nil {
		return nil, err
	} else if active {
		return nil, fmt.Errorf("store: table backfill already active")
	}

	queues, err := s.listActiveModeQueuesTx(ctx, tx, table.ModeID)
	if err != nil {
		return nil, err
	}
	var modeQueue *ModeQueue
	for i := range queues {
		if queues[i].ID == modeQueueID {
			modeQueue = &queues[i]
			break
		}
	}
	if modeQueue == nil {
		return nil, fmt.Errorf("store: queue not found for this table mode")
	}

	seatRoles := map[string]string{}
	for _, seat := range modeSeats {
		path := ""
		if seat.QueuePath != nil {
			path = *seat.QueuePath
		}
		seatRoles[seat.SeatKey] = path
	}

	pinned := make([]partytree.PinnedSeat, len(seated))
	members := make([]JoinPartyMemberInput, len(seated))
	for i, seat := range seated {
		pinned[i] = partytree.PinnedSeat{UserID: seat.UserID.String(), SeatKey: seat.SeatKey}
		path := seatRoles[seat.SeatKey]
		// Each seated player's own picks ride into the queue with them, so a
		// table that backfills does not lose what everyone already chose.
		members[i] = JoinPartyMemberInput{UserID: seat.UserID, QueuePath: path, QueueOptions: seat.QueueOptions}
	}
	tree := partytree.BuildFromPinnedSeats(pinned, seatRoles)
	tree.TableID = tableID.String()

	party, err := s.CreatePartyFromTreeTx(ctx, tx, modeQueue.ID, kingUserID, tree, members)
	if err != nil {
		return nil, err
	}

	for _, member := range members {
		if _, err := enqueueModeQueueTx(ctx, tx, game.ID, modeQueue.ID, member.UserID, member.QueuePath, member.QueueOptions, &party.ID); err != nil {
			return nil, err
		}
	}

	waiting, err := listWaitingModeQueueEntriesTx(ctx, tx, modeQueue.ID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	notifyIDs := make([]uuid.UUID, len(members))
	for i, m := range members {
		notifyIDs[i] = m.UserID
	}
	return &QueueJoinResult{
		GameID:        game.ID,
		ModeQueueID:   modeQueue.ID,
		Status:        QueueStatusWaiting,
		QueuedCount:   len(waiting),
		NotifyUserIDs: notifyIDs,
	}, nil
}

func (s *Store) listActiveModeQueuesTx(ctx context.Context, tx *sql.Tx, modeID uuid.UUID) ([]ModeQueue, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, mode_id, name, status, created_at
		FROM mode_queues
		WHERE mode_id = $1 AND status = $2
		ORDER BY created_at ASC
	`, modeID, ModeQueueActive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ModeQueue
	for rows.Next() {
		var q ModeQueue
		if err := rows.Scan(&q.ID, &q.ModeID, &q.Name, &q.Status, &q.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}
