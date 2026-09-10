package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/prequeue"
	"github.com/scruffyprodigy/joinquest/internal/seattemplate"
)

// ModeOffersPreMatchChoice reports whether a mode asks a player to choose something
// before play: more than one seat class, or any pre-queue option group.
//
// It is the whole trigger for the rejoin seating rules (JQ-232). The principle is not
// seats specifically — it is whether a choice exists that a group would plausibly
// re-make between rounds. Roles rotate (who plays spymaster), and characters change
// constantly, so a mode with pre-queue options counts exactly as a mode with several
// seat types does. Where no choice exists at all, making people re-click an identical
// seat is friction with no upside.
//
// "More than one seat type" means more than one seat *class*, not more than one seat
// key: seat classes are `seattemplate.Leaf.NamePath`, which is precisely the "these two
// seats are interchangeable" fact the rating layer already keys on (see
// rating.ModeShape.SeatClasses). A mode with five identical player seats is one class,
// so it counts as single and its players are re-seated automatically.
//
// A template that will not expand, or a pre-queue block that will not parse, reads as
// "there is a choice". That is the safe direction: it declines to seat anyone
// automatically rather than guessing a seat for a mode nothing can describe.
func ModeOffersPreMatchChoice(mode *GameMode) bool {
	if mode == nil {
		return false
	}
	decl, err := prequeue.Parse(mode.PreQueue)
	if err != nil || decl != nil {
		return true
	}
	leaves, err := seattemplate.Expand(mode.SeatTemplate)
	if err != nil {
		return true
	}
	classes := make(map[string]struct{}, len(leaves))
	for _, leaf := range leaves {
		classes[leaf.NamePath] = struct{}{}
		if len(classes) > 1 {
			return true
		}
	}
	return false
}

// participantSeating is what a player had last round: the seat they held and the
// pre-queue options they picked. It is what a solo rejoin replays.
type participantSeating struct {
	SeatKey string
	Options []prequeue.Selection
	// GroupPlay is true when this player reached the match through a room table
	// rather than the catalog queue. It is read from the return context, which is
	// stamped per participant when the session starts and never rewritten — unlike
	// room_tables.session_id, which resetRoomTableAfterSessionTx clears, and unlike
	// game_sessions.regroup_table_id, which the first claimant stamps for everyone.
	GroupPlay bool
}

func loadParticipantSeatingTx(ctx context.Context, tx *sql.Tx, sessionID, userID uuid.UUID) (*participantSeating, error) {
	var (
		seatKey      sql.NullString
		queueOptions []byte
		returnCtx    []byte
	)
	err := tx.QueryRowContext(ctx, `
		SELECT role, queue_options, return_context
		FROM game_session_participants
		WHERE session_id = $1 AND user_id = $2
	`, sessionID, userID).Scan(&seatKey, &queueOptions, &returnCtx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	options, err := decodeQueueOptions(queueOptions)
	if err != nil {
		return nil, err
	}
	rc, err := decodeReturnContext(returnCtx)
	if err != nil {
		return nil, err
	}
	return &participantSeating{
		SeatKey:   seatKey.String,
		Options:   options,
		GroupPlay: rc.Kind == ReturnKindRoom,
	}, nil
}
