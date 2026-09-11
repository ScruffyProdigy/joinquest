package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/scruffyprodigy/joinquest/internal/lfg/partytree"
)

// StartTableBackfill enqueues seated table players and leaves matchmaking to the forming worker.
func (s *Store) StartTableBackfill(ctx context.Context, tableID, kingUserID, modeQueueID uuid.UUID) (*QueueJoinResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	table, game, mode, modeSeats, err := s.loadTableContext(ctx, tx, tableID)
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
	// The king's, like starting early, and for the same reason (JQ-137). Filling the
	// remaining seats with strangers ends the wait for anyone still on their way just as
	// surely as starting short-handed does — it sells their seat rather than playing
	// without them — so it is the same decision taken on everybody's behalf, and it keeps
	// the same owner.
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
		//
		// They are re-checked on the way in rather than trusted because they were
		// stored once already: a mode whose declaration changed while the table sat
		// forming would otherwise carry a stale selection into matchmaking and only
		// fail at provision, with strangers already matched to it (JQ-211).
		if err := ensureSelectionsSatisfyMode(mode, seat.QueueOptions); err != nil {
			return nil, err
		}
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

// CancelTableBackfillResult reports what a cancel took back out of the queue.
type CancelTableBackfillResult struct {
	Cancelled     bool
	GameID        uuid.UUID
	ModeQueueID   uuid.UUID
	NotifyUserIDs []uuid.UUID
	QueuedCount   int
}

// CancelTableBackfill takes the whole group back out of the queue and leaves the table
// exactly as it stood before the request.
//
// This cannot be LeaveModeQueue per player. That cancels the party but leaves everyone
// else's waiting rows behind as solo entries (see cancelPartyTx), so one member backing
// out would silently convert their friends into strangers looking for a game on their
// own. A request made for the whole table is withdrawn for the whole table.
//
// The king's, like the request itself (JQ-137). Withdrawing it puts everyone back to
// waiting, which is as much a decision for the group as making it was. A second cancel
// finds nothing waiting and reports false rather than failing.
func (s *Store) CancelTableBackfill(ctx context.Context, tableID, actorUserID uuid.UUID) (*CancelTableBackfillResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	table, game, _, _, err := s.loadTableContext(ctx, tx, tableID)
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
	king := tableKingUserID(seated)
	if king == nil || *king != actorUserID {
		return nil, fmt.Errorf("store: only the king can cancel backfill")
	}

	userIDs := make([]uuid.UUID, len(seated))
	for i, seat := range seated {
		userIDs[i] = seat.UserID
	}

	partyIDs, modeQueueID, err := waitingPartiesForUsersTx(ctx, tx, userIDs)
	if err != nil {
		return nil, err
	}
	if modeQueueID == uuid.Nil {
		// Nothing of this table's is waiting: already fired, already cancelled, or
		// never started. Not an error — the caller asked for a state that holds.
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return &CancelTableBackfillResult{GameID: game.ID}, nil
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE game_queues
		SET status = 'cancelled'
		WHERE user_id = ANY($1::uuid[]) AND status = 'waiting'
	`, pq.Array(queueIDStrings(userIDs))); err != nil {
		return nil, err
	}
	// The seats this table holds on the forming map go back to the pool, so the
	// players already matched around it are re-formed on the next reconcile rather
	// than left waiting on a group that has gone.
	if _, err := tx.ExecContext(ctx, `
		UPDATE forming_match_assignments
		SET user_id = NULL, party_id = NULL, source = 'solo', table_id = NULL
		WHERE table_id = $1
	`, tableID); err != nil {
		return nil, err
	}
	if len(partyIDs) > 0 {
		if _, err := tx.ExecContext(ctx, `
			UPDATE parties SET status = $2 WHERE id = ANY($1::uuid[]) AND status IN ($3, $4)
		`, pq.Array(queueIDStrings(partyIDs)), PartyStatusCancelled, PartyStatusWaiting, PartyStatusPlaced); err != nil {
			return nil, err
		}
	}

	waiting, err := listWaitingModeQueueEntriesTx(ctx, tx, modeQueueID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &CancelTableBackfillResult{
		Cancelled:     true,
		GameID:        game.ID,
		ModeQueueID:   modeQueueID,
		NotifyUserIDs: userIDs,
		QueuedCount:   len(waiting),
	}, nil
}

// waitingPartiesForUsersTx returns the parties these users are waiting in, and the queue
// they are waiting on. A table's players are enqueued together, so one queue is expected.
func waitingPartiesForUsersTx(ctx context.Context, tx *sql.Tx, userIDs []uuid.UUID) ([]uuid.UUID, uuid.UUID, error) {
	if len(userIDs) == 0 {
		return nil, uuid.Nil, nil
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT DISTINCT party_id, mode_queue_id
		FROM game_queues
		WHERE user_id = ANY($1::uuid[]) AND status = 'waiting' AND mode_queue_id IS NOT NULL
	`, pq.Array(queueIDStrings(userIDs)))
	if err != nil {
		return nil, uuid.Nil, err
	}
	defer rows.Close()

	var partyIDs []uuid.UUID
	var modeQueueID uuid.UUID
	for rows.Next() {
		var partyID sql.NullString
		var queueID uuid.UUID
		if err := rows.Scan(&partyID, &queueID); err != nil {
			return nil, uuid.Nil, err
		}
		modeQueueID = queueID
		if partyID.Valid {
			id, err := uuid.Parse(partyID.String)
			if err != nil {
				return nil, uuid.Nil, err
			}
			partyIDs = append(partyIDs, id)
		}
	}
	return partyIDs, modeQueueID, rows.Err()
}
