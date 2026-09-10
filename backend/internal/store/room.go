package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	RoomStatusOpen   = "open"
	RoomStatusClosed = "closed"

	roomInviteCodeLen   = 6
	maxRoomMessageLen   = 2000
	defaultMessageLimit = 50
	maxMessageLimit     = 100
)

// inviteCodeAlphabet avoids ambiguous characters (0/O, 1/I/L).
const inviteCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// Room is a social chat room.
type Room struct {
	ID         uuid.UUID
	InviteCode string
	HostUserID uuid.UUID
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// RoomMessage is one chat message in a room.
type RoomMessage struct {
	ID        uuid.UUID
	RoomID    uuid.UUID
	UserID    uuid.UUID
	Body      string
	CreatedAt time.Time
}

const roomColumns = `id, invite_code, host_user_id, status, created_at, updated_at`

func scanRoom(row interface{ Scan(dest ...any) error }) (*Room, error) {
	var r Room
	if err := row.Scan(&r.ID, &r.InviteCode, &r.HostUserID, &r.Status, &r.CreatedAt, &r.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &r, nil
}

func scanRoomMessage(row interface{ Scan(dest ...any) error }) (*RoomMessage, error) {
	var m RoomMessage
	if err := row.Scan(&m.ID, &m.RoomID, &m.UserID, &m.Body, &m.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &m, nil
}

func normalizeInviteCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func generateInviteCode() (string, error) {
	buf := make([]byte, roomInviteCodeLen)
	max := byte(len(inviteCodeAlphabet))
	random := make([]byte, roomInviteCodeLen)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("store: generate invite code: %w", err)
	}
	for i := range buf {
		buf[i] = inviteCodeAlphabet[int(random[i])%int(max)]
	}
	return string(buf), nil
}

func (s *Store) newUniqueInviteCode(ctx context.Context, q sqlQueryRowContext) (string, error) {
	for i := 0; i < 10; i++ {
		code, err := generateInviteCode()
		if err != nil {
			return "", err
		}
		var exists bool
		if err := q.QueryRowContext(ctx, `
			SELECT EXISTS(SELECT 1 FROM rooms WHERE UPPER(invite_code) = $1 AND status = $2)
		`, code, RoomStatusOpen).Scan(&exists); err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}
	return "", fmt.Errorf("store: failed to generate unique invite code")
}

func (s *Store) leaveRoomTx(ctx context.Context, tx *sql.Tx, userID uuid.UUID) (*uuid.UUID, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT room_id FROM room_members WHERE user_id = $1
	`, userID)
	var roomID uuid.UUID
	if err := row.Scan(&roomID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM room_members WHERE user_id = $1`, userID); err != nil {
		return nil, err
	}

	// Leaving the room gives up the seat held in it. A seat is only claimable by a
	// member (sitAtTableTx's isRoomMemberTx), so a seat outliving its membership is a
	// state no rule allows: it blocks its seat_key against the friends still here, and
	// it reads as live play in roomHasLivePlayClause, which would hold the room open on
	// behalf of somebody who has left.
	if _, _, err := s.leaveTableSeatTx(ctx, tx, userID); err != nil {
		return nil, err
	}

	var remaining int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM room_members WHERE room_id = $1
	`, roomID).Scan(&remaining); err != nil {
		return nil, err
	}

	if remaining == 0 {
		// An empty roster is not always a finished room: players who are IN a game
		// have their sockets on the game rather than the lobby, and a started table
		// with an active session is the room they come back to. closeRoomIfEmptyTx
		// makes that distinction; when it declines, the room stays open with nobody
		// in it until the play behind it ends.
		if _, err := s.closeRoomIfEmptyTx(ctx, tx, roomID); err != nil {
			return nil, err
		}
		// No members left, so there is no host to reassign either way.
		return &roomID, nil
	}

	var hostID uuid.UUID
	if err := tx.QueryRowContext(ctx, `SELECT host_user_id FROM rooms WHERE id = $1`, roomID).Scan(&hostID); err != nil {
		return nil, err
	}
	if hostID != userID {
		if _, err := tx.ExecContext(ctx, `UPDATE rooms SET updated_at = NOW() WHERE id = $1`, roomID); err != nil {
			return nil, err
		}
		return &roomID, nil
	}

	row = tx.QueryRowContext(ctx, `
		SELECT user_id FROM room_members
		WHERE room_id = $1
		ORDER BY joined_at ASC
		LIMIT 1
	`, roomID)
	var newHost uuid.UUID
	if err := row.Scan(&newHost); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE rooms SET host_user_id = $2, updated_at = NOW() WHERE id = $1
	`, roomID, newHost); err != nil {
		return nil, err
	}
	return &roomID, nil
}

func (s *Store) addRoomMemberTx(ctx context.Context, tx *sql.Tx, roomID, userID uuid.UUID) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO room_members (room_id, user_id) VALUES ($1, $2)
	`, roomID, userID)
	return err
}

// ensureRoomMemberTx puts the user in the room, leaving whichever room they are in now.
// room_members is unique per user, so a blind insert fails for someone who is already
// here — which a regrouping room-table group always is.
func (s *Store) ensureRoomMemberTx(ctx context.Context, tx *sql.Tx, roomID, userID uuid.UUID) error {
	var currentRoomID uuid.UUID
	err := tx.QueryRowContext(ctx, `
		SELECT room_id FROM room_members WHERE user_id = $1
	`, userID).Scan(&currentRoomID)
	switch {
	case err == nil && currentRoomID == roomID:
		return nil
	case err != nil && !errors.Is(err, sql.ErrNoRows):
		return err
	}

	if _, err := s.leaveRoomTx(ctx, tx, userID); err != nil {
		return err
	}
	return s.addRoomMemberTx(ctx, tx, roomID, userID)
}

// CreateRoom opens a new chat room and adds the host as the sole member.
func (s *Store) CreateRoom(ctx context.Context, hostUserID uuid.UUID) (*Room, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	room, err := s.createRoomTx(ctx, tx, hostUserID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return room, nil
}

func (s *Store) createRoomTx(ctx context.Context, tx *sql.Tx, hostUserID uuid.UUID) (*Room, error) {
	if _, err := s.leaveRoomTx(ctx, tx, hostUserID); err != nil {
		return nil, err
	}

	code, err := s.newUniqueInviteCode(ctx, tx)
	if err != nil {
		return nil, err
	}

	row := tx.QueryRowContext(ctx, `
		INSERT INTO rooms (invite_code, host_user_id, status)
		VALUES ($1, $2, $3)
		RETURNING `+roomColumns+`
	`, code, hostUserID, RoomStatusOpen)
	room, err := scanRoom(row)
	if err != nil {
		return nil, err
	}

	if err := s.addRoomMemberTx(ctx, tx, room.ID, hostUserID); err != nil {
		return nil, err
	}

	return room, nil
}

// JoinRoom adds the user to an open room by invite code, leaving any prior room.
func (s *Store) JoinRoom(ctx context.Context, userID uuid.UUID, inviteCode string) (*Room, error) {
	code := normalizeInviteCode(inviteCode)
	if code == "" {
		return nil, fmt.Errorf("store: invite code is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
		SELECT `+roomColumns+`
		FROM rooms
		WHERE UPPER(invite_code) = $1 AND status = $2
	`, code, RoomStatusOpen)
	room, err := scanRoom(row)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	var existingRoomID uuid.UUID
	err = tx.QueryRowContext(ctx, `SELECT room_id FROM room_members WHERE user_id = $1`, userID).Scan(&existingRoomID)
	if err == nil && existingRoomID == room.ID {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return room, nil
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	if _, err := s.leaveRoomTx(ctx, tx, userID); err != nil {
		return nil, err
	}

	if err := s.addRoomMemberTx(ctx, tx, room.ID, userID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE rooms SET updated_at = NOW() WHERE id = $1`, room.ID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return room, nil
}

// LeaveRoom removes the user from their current room.
func (s *Store) LeaveRoom(ctx context.Context, userID uuid.UUID) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	roomID, err := s.leaveRoomTx(ctx, tx, userID)
	if err != nil {
		return false, err
	}
	if roomID == nil {
		return false, nil
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// GetRoomByID loads a room by id.
func (s *Store) GetRoomByID(ctx context.Context, roomID uuid.UUID) (*Room, error) {
	return s.getRoomByIDTx(ctx, s.db, roomID)
}

func (s *Store) getRoomByIDTx(ctx context.Context, q sqlQueryRowContext, roomID uuid.UUID) (*Room, error) {
	row := q.QueryRowContext(ctx, `
		SELECT `+roomColumns+` FROM rooms WHERE id = $1
	`, roomID)
	return scanRoom(row)
}

// GetRoomByInviteCode loads an open room by invite code.
func (s *Store) GetRoomByInviteCode(ctx context.Context, inviteCode string) (*Room, error) {
	code := normalizeInviteCode(inviteCode)
	if code == "" {
		return nil, ErrNotFound
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT `+roomColumns+`
		FROM rooms
		WHERE UPPER(invite_code) = $1 AND status = $2
	`, code, RoomStatusOpen)
	return scanRoom(row)
}

// GetUserRoom returns the room the user is currently in, if any.
func (s *Store) GetUserRoom(ctx context.Context, userID uuid.UUID) (*Room, error) {
	return s.getUserRoomTx(ctx, s.db, userID)
}

func (s *Store) getUserRoomTx(ctx context.Context, q sqlQueryRowContext, userID uuid.UUID) (*Room, error) {
	row := q.QueryRowContext(ctx, `
		SELECT r.id, r.invite_code, r.host_user_id, r.status, r.created_at, r.updated_at
		FROM rooms r
		INNER JOIN room_members rm ON rm.room_id = r.id
		WHERE rm.user_id = $1 AND r.status = $2
	`, userID, RoomStatusOpen)
	return scanRoom(row)
}

// IsRoomMember reports whether the user belongs to the room.
func (s *Store) IsRoomMember(ctx context.Context, roomID, userID uuid.UUID) (bool, error) {
	return s.isRoomMemberTx(ctx, s.db, roomID, userID)
}

func (s *Store) isRoomMemberTx(ctx context.Context, q sqlQueryRowContext, roomID, userID uuid.UUID) (bool, error) {
	var exists bool
	err := q.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM room_members rm
			INNER JOIN rooms r ON r.id = rm.room_id
			WHERE rm.room_id = $1 AND rm.user_id = $2 AND r.status = $3
		)
	`, roomID, userID, RoomStatusOpen).Scan(&exists)
	return exists, err
}

// ListRoomMemberUsers returns members ordered by join time.
func (s *Store) ListRoomMemberUsers(ctx context.Context, roomID uuid.UUID) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+strings.ReplaceAll(userColumns, "id,", "u.id,")+`
		FROM users u
		INNER JOIN room_members rm ON rm.user_id = u.id
		WHERE rm.room_id = $1 AND u.is_active = true
		ORDER BY rm.joined_at ASC
	`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *u)
	}
	return users, rows.Err()
}

// ListRoomMessages returns recent messages oldest-first for display.
func (s *Store) ListRoomMessages(ctx context.Context, roomID uuid.UUID, limit int, before *uuid.UUID) ([]RoomMessage, error) {
	if limit <= 0 {
		limit = defaultMessageLimit
	}
	if limit > maxMessageLimit {
		limit = maxMessageLimit
	}

	var rows *sql.Rows
	var err error
	if before != nil {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, room_id, user_id, body, created_at
			FROM room_messages
			WHERE room_id = $1
			  AND created_at < (SELECT created_at FROM room_messages WHERE id = $2)
			ORDER BY created_at DESC
			LIMIT $3
		`, roomID, *before, limit)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, room_id, user_id, body, created_at
			FROM room_messages
			WHERE room_id = $1
			ORDER BY created_at DESC
			LIMIT $2
		`, roomID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var desc []RoomMessage
	for rows.Next() {
		msg, err := scanRoomMessage(rows)
		if err != nil {
			return nil, err
		}
		desc = append(desc, *msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Reverse to oldest-first for chat UI.
	for i, j := 0, len(desc)-1; i < j; i, j = i+1, j-1 {
		desc[i], desc[j] = desc[j], desc[i]
	}
	return desc, nil
}

// SendRoomMessage appends a chat message; caller must verify membership.
func (s *Store) SendRoomMessage(ctx context.Context, roomID, userID uuid.UUID, body string) (*RoomMessage, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, fmt.Errorf("store: message body is required")
	}
	if len(body) > maxRoomMessageLen {
		return nil, fmt.Errorf("store: message body too long")
	}

	member, err := s.IsRoomMember(ctx, roomID, userID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, ErrNotFound
	}

	row := s.db.QueryRowContext(ctx, `
		INSERT INTO room_messages (room_id, user_id, body)
		VALUES ($1, $2, $3)
		RETURNING id, room_id, user_id, body, created_at
	`, roomID, userID, body)
	msg, err := scanRoomMessage(row)
	if err != nil {
		return nil, err
	}

	if _, err := s.db.ExecContext(ctx, `UPDATE rooms SET updated_at = NOW() WHERE id = $1`, roomID); err != nil {
		return nil, err
	}
	return msg, nil
}

// GetRoomMessageByID loads a single message.
func (s *Store) GetRoomMessageByID(ctx context.Context, messageID uuid.UUID) (*RoomMessage, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, room_id, user_id, body, created_at
		FROM room_messages WHERE id = $1
	`, messageID)
	return scanRoomMessage(row)
}
