package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/playhub/internal/seattemplate"
)

const (
	TableStatusForming   = "forming"
	TableStatusStarted   = "started"
	TableStatusDiscarded = "discarded"

	staleEmptyTableAge = 60 * time.Second
)

// RoomTable is a forming or started game table inside a room.
type RoomTable struct {
	ID        uuid.UUID
	RoomID    uuid.UUID
	GameID    uuid.UUID
	ModeID    uuid.UUID
	Status    string
	SessionID *uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableSeat is one player seated at a specific seat key.
type TableSeat struct {
	ID       uuid.UUID
	TableID  uuid.UUID
	UserID   uuid.UUID
	SeatKey  string
	SeatedAt time.Time
}

// UserTableSeatView is the caller's active table seat for the intent banner.
type UserTableSeatView struct {
	TableID    uuid.UUID
	RoomID     uuid.UUID
	InviteCode string
	GameID     uuid.UUID
	GameName   string
	ModeID     uuid.UUID
	ModeName   string
	SeatKey    string
	SessionID  *uuid.UUID
}

// StartTableResult is returned when a table starts a session.
type StartTableResult struct {
	GameID        uuid.UUID
	SessionID     uuid.UUID
	NotifyUserIDs []uuid.UUID
}

const roomTableColumns = `id, room_id, game_id, mode_id, status, session_id, created_at, updated_at`

func scanRoomTable(row interface{ Scan(dest ...any) error }) (*RoomTable, error) {
	var t RoomTable
	var sessionID sql.NullString
	if err := row.Scan(
		&t.ID, &t.RoomID, &t.GameID, &t.ModeID, &t.Status, &sessionID,
		&t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if sessionID.Valid {
		id, err := uuid.Parse(sessionID.String)
		if err != nil {
			return nil, err
		}
		t.SessionID = &id
	}
	return &t, nil
}

func scanTableSeat(row interface{ Scan(dest ...any) error }) (*TableSeat, error) {
	var s TableSeat
	if err := row.Scan(&s.ID, &s.TableID, &s.UserID, &s.SeatKey, &s.SeatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &s, nil
}

func seatByKey(seats []GameModeSeat, seatKey string) (*GameModeSeat, error) {
	for i := range seats {
		if seats[i].SeatKey == seatKey {
			return &seats[i], nil
		}
	}
	return nil, fmt.Errorf("store: invalid seat key %q", seatKey)
}

func isPooledSeatGroup(modeSeats []GameModeSeat, queuePath string) bool {
	return queuePath != "" || allSeatsShareQueuePath(modeSeats, queuePath)
}

// resolvePooledSeatKey picks the next open seat in a pooled fifo group. The requested
// key identifies which group to join; stale clients may send an already-taken seat.
func resolvePooledSeatKey(modeSeats []GameModeSeat, seated []TableSeat, requestedKey string) (string, error) {
	target, err := seatByKey(modeSeats, requestedKey)
	if err != nil {
		return "", err
	}
	path := seatQueuePathValue(*target)
	if !isPooledSeatGroup(modeSeats, path) {
		return requestedKey, nil
	}
	next := firstOpenSeatKeyInPath(modeSeats, seated, path)
	if next == "" {
		return "", fmt.Errorf("store: no open seats in this group")
	}
	return next, nil
}

func allSeatsShareQueuePath(modeSeats []GameModeSeat, queuePath string) bool {
	if len(modeSeats) <= 1 {
		return false
	}
	for _, seat := range modeSeats {
		if seatQueuePathValue(seat) != queuePath {
			return false
		}
	}
	return true
}

func firstOpenSeatKeyInPath(modeSeats []GameModeSeat, seated []TableSeat, queuePath string) string {
	occupied := make(map[string]struct{}, len(seated))
	for _, s := range seated {
		occupied[s.SeatKey] = struct{}{}
	}
	for _, seat := range modeSeats {
		if seatQueuePathValue(seat) != queuePath {
			continue
		}
		if _, taken := occupied[seat.SeatKey]; !taken {
			return seat.SeatKey
		}
	}
	return ""
}

func countSeatedByPath(seated []TableSeat, seats []GameModeSeat, queuePath string) int {
	byKey := make(map[string]string, len(seats))
	for _, seat := range seats {
		byKey[seat.SeatKey] = seatQueuePathValue(seat)
	}
	count := 0
	for _, s := range seated {
		if byKey[s.SeatKey] == queuePath {
			count++
		}
	}
	return count
}

func tableKingUserID(seated []TableSeat) *uuid.UUID {
	if len(seated) == 0 {
		return nil
	}
	king := seated[0].UserID
	earliest := seated[0].SeatedAt
	for _, s := range seated[1:] {
		if s.SeatedAt.Before(earliest) {
			earliest = s.SeatedAt
			king = s.UserID
		}
	}
	return &king
}

func minRequiredForPath(spec seattemplate.PathSpec) int {
	if spec.Min > 0 {
		return spec.Min
	}
	return spec.PlayersToStart()
}

func tableCanStart(mode *GameMode, modeSeats []GameModeSeat, seated []TableSeat, template json.RawMessage) (bool, error) {
	count := len(seated)
	if count < mode.MinPlayers || count > mode.MaxPlayers {
		return false, nil
	}
	specs, err := seattemplate.PathSpecs(template)
	if err != nil {
		return false, err
	}
	if len(specs) == 0 {
		return count >= mode.MinPlayers, nil
	}
	for _, spec := range specs {
		if countSeatedByPath(seated, modeSeats, spec.QueuePath) < minRequiredForPath(spec) {
			return false, nil
		}
	}
	return true, nil
}

func tableCanDiscard(table *RoomTable, seatedCount int, callerID uuid.UUID, kingID *uuid.UUID) bool {
	if table.Status != TableStatusForming || seatedCount > 0 {
		return false
	}
	if kingID != nil && *kingID == callerID {
		return true
	}
	return time.Since(table.CreatedAt) >= staleEmptyTableAge
}

func teamPrefixFromSeatKey(seatKey string) string {
	parts := strings.Split(seatKey, "-")
	if len(parts) >= 2 && strings.EqualFold(parts[0], "Team") {
		return parts[0] + "-" + parts[1]
	}
	return ""
}

func leaveUserWaitingQueuesTx(ctx context.Context, tx *sql.Tx, userID uuid.UUID) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE game_queues SET status = 'cancelled'
		WHERE user_id = $1 AND status = 'waiting'
	`, userID)
	return err
}

func ensureNotQueueMatchedTx(ctx context.Context, tx *sql.Tx, userID uuid.UUID) error {
	var exists bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM game_queues WHERE user_id = $1 AND status = 'matched')
	`, userID).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return ErrAlreadyMatched
	}
	return nil
}

// leaveTableSeatTx removes the user from any table seat. Returns affected table and room ids.
func (s *Store) leaveTableSeatTx(ctx context.Context, tx *sql.Tx, userID uuid.UUID) (*uuid.UUID, *uuid.UUID, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT ts.table_id, rt.room_id
		FROM table_seats ts
		INNER JOIN room_tables rt ON rt.id = ts.table_id
		WHERE ts.user_id = $1 AND rt.status = $2
	`, userID, TableStatusForming)
	var tableID, roomID uuid.UUID
	if err := row.Scan(&tableID, &roomID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM table_seats WHERE user_id = $1`, userID); err != nil {
		return nil, nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE room_tables SET updated_at = NOW() WHERE id = $1
	`, tableID); err != nil {
		return nil, nil, err
	}
	return &tableID, &roomID, nil
}

func (s *Store) sweepStaleEmptyTablesTx(ctx context.Context, tx *sql.Tx, roomID uuid.UUID) error {
	_, err := tx.ExecContext(ctx, `
		DELETE FROM room_tables rt
		WHERE rt.room_id = $1
		  AND rt.status = $2
		  AND rt.created_at < NOW() - ($3 * INTERVAL '1 second')
		  AND NOT EXISTS (SELECT 1 FROM table_seats ts WHERE ts.table_id = rt.id)
	`, roomID, TableStatusForming, int(staleEmptyTableAge.Seconds()))
	return err
}

func (s *Store) listTableSeatsTx(ctx context.Context, tx *sql.Tx, tableID uuid.UUID) ([]TableSeat, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, table_id, user_id, seat_key, seated_at
		FROM table_seats
		WHERE table_id = $1
		ORDER BY seated_at ASC
	`, tableID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var seated []TableSeat
	for rows.Next() {
		seat, err := scanTableSeat(rows)
		if err != nil {
			return nil, err
		}
		seated = append(seated, *seat)
	}
	return seated, rows.Err()
}

func (s *Store) loadTableContext(ctx context.Context, q sqlQueryRowContext, tableID uuid.UUID) (*RoomTable, *Game, *GameMode, []GameModeSeat, error) {
	row := q.QueryRowContext(ctx, `
		SELECT `+roomTableColumns+`
		FROM room_tables
		WHERE id = $1
	`, tableID)
	table, err := scanRoomTable(row)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	game, err := getGameForMatchmaking(ctx, q, table.GameID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	mode, err := getGameModeByID(ctx, q, table.ModeID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if mode.Status != ModeStatusActive {
		return nil, nil, nil, nil, fmt.Errorf("store: game mode is not active")
	}
	seats, err := listGameModeSeats(ctx, q, mode.ID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if len(seats) == 0 {
		return nil, nil, nil, nil, fmt.Errorf("store: mode has no seats configured")
	}
	return table, game, mode, seats, nil
}

func (s *Store) createTableTx(ctx context.Context, tx *sql.Tx, roomID, gameID, modeID uuid.UUID) (*RoomTable, error) {
	if err := s.sweepStaleEmptyTablesTx(ctx, tx, roomID); err != nil {
		return nil, err
	}
	mode, err := getGameModeByID(ctx, tx, modeID)
	if err != nil {
		return nil, err
	}
	if mode.GameID != gameID {
		return nil, fmt.Errorf("store: mode does not belong to game")
	}
	if mode.Status != ModeStatusActive {
		return nil, fmt.Errorf("store: game mode is not active")
	}
	if _, err := getGameForMatchmaking(ctx, tx, gameID); err != nil {
		return nil, err
	}

	row := tx.QueryRowContext(ctx, `
		INSERT INTO room_tables (room_id, game_id, mode_id, status)
		VALUES ($1, $2, $3, $4)
		RETURNING `+roomTableColumns+`
	`, roomID, gameID, modeID, TableStatusForming)
	table, err := scanRoomTable(row)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE rooms SET updated_at = NOW() WHERE id = $1`, roomID); err != nil {
		return nil, err
	}
	return table, nil
}

// CreateTable adds a forming table to a room the user belongs to.
func (s *Store) CreateTable(ctx context.Context, roomID, gameID, modeID, userID uuid.UUID) (*RoomTable, error) {
	member, err := s.IsRoomMember(ctx, roomID, userID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, ErrNotFound
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if err := ensureNotInActiveGameTx(ctx, tx, userID); err != nil {
		return nil, err
	}

	table, err := s.createTableTx(ctx, tx, roomID, gameID, modeID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return table, nil
}

// CreatePrivateTable ensures the user has a room, then creates a forming table.
func (s *Store) CreatePrivateTable(ctx context.Context, userID, gameID, modeID uuid.UUID) (*RoomTable, error) {
	room, err := s.GetUserRoom(ctx, userID)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
		room, err = s.CreateRoom(ctx, userID)
		if err != nil {
			return nil, err
		}
	}
	return s.CreateTable(ctx, room.ID, gameID, modeID, userID)
}

// ListRoomTables returns forming tables in a room.
func (s *Store) ListRoomTables(ctx context.Context, roomID uuid.UUID) ([]RoomTable, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+roomTableColumns+`
		FROM room_tables
		WHERE room_id = $1 AND status = $2
		ORDER BY created_at ASC
	`, roomID, TableStatusForming)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []RoomTable
	for rows.Next() {
		table, err := scanRoomTable(rows)
		if err != nil {
			return nil, err
		}
		tables = append(tables, *table)
	}
	return tables, rows.Err()
}

// GetRoomTableByID loads a table by id.
func (s *Store) GetRoomTableByID(ctx context.Context, tableID uuid.UUID) (*RoomTable, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+roomTableColumns+` FROM room_tables WHERE id = $1
	`, tableID)
	return scanRoomTable(row)
}

// ListTableSeats returns seated players ordered by seated_at.
func (s *Store) ListTableSeats(ctx context.Context, tableID uuid.UUID) ([]TableSeat, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, table_id, user_id, seat_key, seated_at
		FROM table_seats
		WHERE table_id = $1
		ORDER BY seated_at ASC
	`, tableID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var seated []TableSeat
	for rows.Next() {
		seat, err := scanTableSeat(rows)
		if err != nil {
			return nil, err
		}
		seated = append(seated, *seat)
	}
	return seated, rows.Err()
}

// GetUserTableSeat returns the user's active forming table seat, if any.
func (s *Store) GetUserTableSeat(ctx context.Context, userID uuid.UUID) (*UserTableSeatView, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT rt.id, rt.room_id, r.invite_code, g.id, g.name, gm.id, gm.display_name, ts.seat_key
		FROM table_seats ts
		INNER JOIN room_tables rt ON rt.id = ts.table_id AND rt.status = $2
		INNER JOIN rooms r ON r.id = rt.room_id
		INNER JOIN games g ON g.id = rt.game_id
		INNER JOIN game_modes gm ON gm.id = rt.mode_id
		WHERE ts.user_id = $1
	`, userID, TableStatusForming)
	var view UserTableSeatView
	if err := row.Scan(
		&view.TableID, &view.RoomID, &view.InviteCode,
		&view.GameID, &view.GameName, &view.ModeID, &view.ModeName, &view.SeatKey,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &view, nil
}

// GetUserStartedTableSession returns the user's active launch from a started room table.
func (s *Store) GetUserStartedTableSession(ctx context.Context, userID uuid.UUID) (*UserTableSeatView, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT rt.id, rt.room_id, r.invite_code, g.id, g.name, gm.id, gm.display_name, COALESCE(NULLIF(gsp.role, ''), 'player'), gs.id
		FROM game_session_participants gsp
		INNER JOIN game_sessions gs ON gs.id = gsp.session_id AND gs.status = 'active'
		INNER JOIN room_tables rt ON rt.session_id = gs.id AND rt.status = $2
		INNER JOIN rooms r ON r.id = rt.room_id
		INNER JOIN games g ON g.id = rt.game_id
		INNER JOIN game_modes gm ON gm.id = rt.mode_id
		WHERE gsp.user_id = $1
		ORDER BY rt.updated_at DESC
		LIMIT 1
	`, userID, TableStatusStarted)
	var view UserTableSeatView
	var sessionID uuid.UUID
	if err := row.Scan(
		&view.TableID, &view.RoomID, &view.InviteCode,
		&view.GameID, &view.GameName, &view.ModeID, &view.ModeName, &view.SeatKey, &sessionID,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	view.SessionID = &sessionID
	return &view, nil
}

// SitAtTable seats the user at a seat key, leaving queue and other table seats first.
// For pooled fifo groups (shared queue path or flat duel seats), the server assigns
// the next open seat in that group so stale clients do not need the exact seat number.
func (s *Store) SitAtTable(ctx context.Context, tableID, userID uuid.UUID, seatKey string) (*RoomTable, error) {
	seatKey = strings.TrimSpace(seatKey)
	if seatKey == "" {
		return nil, fmt.Errorf("store: seat key is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if err := ensureNotQueueMatchedTx(ctx, tx, userID); err != nil {
		return nil, err
	}
	if err := ensureNotInActiveGameTx(ctx, tx, userID); err != nil {
		return nil, err
	}
	if err := leaveUserWaitingQueuesTx(ctx, tx, userID); err != nil {
		return nil, err
	}
	if _, _, err := s.leaveTableSeatTx(ctx, tx, userID); err != nil {
		return nil, err
	}

	table, _, mode, modeSeats, err := s.loadTableContext(ctx, tx, tableID)
	if err != nil {
		return nil, err
	}
	if table.Status != TableStatusForming {
		return nil, fmt.Errorf("store: table is not accepting seats")
	}
	member, err := s.IsRoomMember(ctx, table.RoomID, userID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, ErrNotFound
	}

	target, err := seatByKey(modeSeats, seatKey)
	if err != nil {
		return nil, err
	}

	seated, err := s.listTableSeatsTx(ctx, tx, tableID)
	if err != nil {
		return nil, err
	}
	if len(seated) >= mode.MaxPlayers {
		return nil, fmt.Errorf("store: table is full")
	}

	seatKey, err = resolvePooledSeatKey(modeSeats, seated, seatKey)
	if err != nil {
		return nil, err
	}
	target, err = seatByKey(modeSeats, seatKey)
	if err != nil {
		return nil, err
	}

	path := seatQueuePathValue(*target)
	if path != "" {
		specs, err := seattemplate.PathSpecs(mode.SeatTemplate)
		if err != nil {
			return nil, err
		}
		for _, spec := range specs {
			if spec.QueuePath != path {
				continue
			}
			if countSeatedByPath(seated, modeSeats, path)+1 > spec.Max {
				return nil, fmt.Errorf("store: role is full")
			}
		}
	}

	if !isPooledSeatGroup(modeSeats, path) {
		var occupied bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS(SELECT 1 FROM table_seats WHERE table_id = $1 AND seat_key = $2)
		`, tableID, seatKey).Scan(&occupied); err != nil {
			return nil, err
		}
		if occupied {
			return nil, fmt.Errorf("store: seat is already taken")
		}
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO table_seats (table_id, user_id, seat_key)
		VALUES ($1, $2, $3)
	`, tableID, userID, seatKey); err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("store: already seated at a table")
		}
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE room_tables SET updated_at = NOW() WHERE id = $1
	`, tableID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return table, nil
}

// LeaveTable removes the user from a table.
func (s *Store) LeaveTable(ctx context.Context, tableID, userID uuid.UUID) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		DELETE FROM table_seats
		WHERE table_id = $1 AND user_id = $2
	`, tableID, userID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if affected == 0 {
		return false, nil
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE room_tables SET updated_at = NOW() WHERE id = $1
	`, tableID); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// DiscardTable removes an empty forming table.
func (s *Store) DiscardTable(ctx context.Context, tableID, userID uuid.UUID) (bool, error) {
	table, err := s.GetRoomTableByID(ctx, tableID)
	if err != nil {
		return false, err
	}
	if table.Status != TableStatusForming {
		return false, nil
	}
	member, err := s.IsRoomMember(ctx, table.RoomID, userID)
	if err != nil {
		return false, err
	}
	if !member {
		return false, ErrNotFound
	}

	seated, err := s.ListTableSeats(ctx, tableID)
	if err != nil {
		return false, err
	}
	if !tableCanDiscard(table, len(seated), userID, tableKingUserID(seated)) {
		return false, fmt.Errorf("store: table cannot be discarded")
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE room_tables SET status = $2, updated_at = NOW() WHERE id = $1 AND status = $3
	`, tableID, TableStatusDiscarded, TableStatusForming)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// StartTable provisions a session from seated players; caller must be king.
func (s *Store) StartTable(ctx context.Context, tableID, userID uuid.UUID) (*StartTableResult, error) {
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
	member, err := s.IsRoomMember(ctx, table.RoomID, userID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, ErrNotFound
	}

	seated, err := s.listTableSeatsTx(ctx, tx, tableID)
	if err != nil {
		return nil, err
	}
	king := tableKingUserID(seated)
	if king == nil || *king != userID {
		return nil, fmt.Errorf("store: only the king can start the table")
	}
	canStart, err := tableCanStart(mode, modeSeats, seated, mode.SeatTemplate)
	if err != nil {
		return nil, err
	}
	if !canStart {
		return nil, fmt.Errorf("store: table cannot start yet")
	}

	row := tx.QueryRowContext(ctx, `
		INSERT INTO game_sessions (game_id, status, mode_id)
		VALUES ($1, 'active', $2)
		RETURNING id, game_id, mode_id, mode_queue_id, status, started_at, ended_at
	`, game.ID, mode.ID)
	session, err := scanSessionRow(row)
	if err != nil {
		return nil, err
	}

	room, err := s.GetRoomByID(ctx, table.RoomID)
	if err != nil {
		return nil, err
	}
	returnCtx := RoomTableReturnContext(room.InviteCode, game.ID, table.RoomID, table.ID)

	notifyIDs := make([]uuid.UUID, 0, len(seated))
	for _, seat := range seated {
		if err := addSessionParticipantTx(ctx, tx, session.ID, seat.UserID, seat.SeatKey, returnCtx); err != nil {
			return nil, err
		}
		notifyIDs = append(notifyIDs, seat.UserID)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM table_seats WHERE table_id = $1`, tableID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE room_tables
		SET status = $2, session_id = $3, updated_at = NOW()
		WHERE id = $1
	`, tableID, TableStatusStarted, session.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &StartTableResult{
		GameID:        game.ID,
		SessionID:     session.ID,
		NotifyUserIDs: notifyIDs,
	}, nil
}

// TableKingUserID returns the king for a table based on current seats.
func (s *Store) TableKingUserID(ctx context.Context, tableID uuid.UUID) (*uuid.UUID, error) {
	seated, err := s.ListTableSeats(ctx, tableID)
	if err != nil {
		return nil, err
	}
	return tableKingUserID(seated), nil
}

// TableCanStart reports whether the table meets start requirements.
func (s *Store) TableCanStart(ctx context.Context, tableID uuid.UUID) (bool, error) {
	table, err := s.GetRoomTableByID(ctx, tableID)
	if err != nil {
		return false, err
	}
	if table.Status != TableStatusForming {
		return false, nil
	}
	mode, err := s.GetGameModeByID(ctx, table.ModeID)
	if err != nil {
		return false, err
	}
	modeSeats, err := s.ListGameModeSeats(ctx, table.ModeID)
	if err != nil {
		return false, err
	}
	seated, err := s.ListTableSeats(ctx, tableID)
	if err != nil {
		return false, err
	}
	return tableCanStart(mode, modeSeats, seated, mode.SeatTemplate)
}

// TableCanDiscard reports whether the table can be discarded by the caller.
func (s *Store) TableCanDiscard(ctx context.Context, tableID, userID uuid.UUID) (bool, error) {
	table, err := s.GetRoomTableByID(ctx, tableID)
	if err != nil {
		return false, err
	}
	seated, err := s.ListTableSeats(ctx, tableID)
	if err != nil {
		return false, err
	}
	return tableCanDiscard(table, len(seated), userID, tableKingUserID(seated)), nil
}
