package graph

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/graph/model"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

// requireMatchParticipant is the authorization gate on everything a match knows about
// itself: the standings, the roster, and the regroup offer are for the people who played
// that match and for nobody else. Match ids travel in a player-editable return URL, so a
// signed-in stranger can guess or be handed one; without this gate they would read another
// group's names and outcome.
func requireMatchParticipant(ctx context.Context, st *store.Store, sessionID, userID uuid.UUID) error {
	// Deliberately "is an active participant": ParticipantIsActive filters
	// left_at IS NULL, which is the codebase's prevailing participant predicate. Nothing
	// writes a non-null left_at today, so this is identical to "played in this match".
	// If left_at ever starts being written, revisit this together with GetMatchResult,
	// which deliberately includes players who have left — otherwise a player who left
	// would still appear on everyone else's roster while being told they did not play.
	if err := st.ParticipantIsActive(ctx, sessionID, userID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return errors.New("you did not play in this match")
		}
		return err
	}
	return nil
}

func loadAuthorizedGameSession(ctx context.Context, st *store.Store, matchID string) (uuid.UUID, *store.Session, error) {
	sessionID, err := parseUUID(matchID, "match id")
	if err != nil {
		return uuid.Nil, nil, err
	}
	session, err := st.GetSessionByID(ctx, sessionID)
	if err != nil {
		return uuid.Nil, nil, err
	}
	if err := requireGameServiceForSessionGame(ctx, session.GameID); err != nil {
		return uuid.Nil, nil, err
	}
	return sessionID, session, nil
}

func returnDestinationFromContext(ctx store.ReturnContext) *model.ReturnDestination {
	defaultDest := &model.ReturnDestination{Path: "/", Kind: store.ReturnKindCatalogLFG}
	if err := ctx.Validate(); err != nil {
		return defaultDest
	}
	return &model.ReturnDestination{Path: ctx.Path, Kind: ctx.Kind}
}

// resolveReturnDestination computes where an already-authorized participant should land,
// given a resolved session. Shared by the returnDestination query and declinePlayAgain, which
// routes a decliner through the same "where do I go now" logic instead of duplicating it.
func resolveReturnDestination(ctx context.Context, st *store.Store, sessionID, userID uuid.UUID) (*model.ReturnDestination, error) {
	defaultDest := &model.ReturnDestination{Path: "/", Kind: store.ReturnKindCatalogLFG}

	if err := st.AcknowledgePlayerReturn(ctx, sessionID, userID, time.Now()); err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}

	if err := st.ParticipantIsActive(ctx, sessionID, userID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return defaultDest, nil
		}
		return nil, err
	}

	ctxData, err := st.GetParticipantReturnContext(ctx, sessionID, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return defaultDest, nil
		}
		return nil, err
	}
	return returnDestinationFromContext(ctxData), nil
}

// regroupClientError translates the store's regroup sentinel errors into messages a client can
// act on, instead of letting an internal "store: ..." string (or an opaque 500) reach the
// player. The three cases are deliberately distinguishable from each other and from "you did
// not play in this match": the client falls back to the game detail page on ErrNoRegroupMode,
// and can tell "too early" (ErrSessionNotFinished) apart from "not your match" (ErrNotFound).
func regroupClientError(err error) error {
	switch {
	case errors.Is(err, store.ErrNoRegroupMode):
		return errors.New("this match no longer has a mode to build a table from")
	case errors.Is(err, store.ErrSessionNotFinished):
		return errors.New("this match hasn't finished yet")
	case errors.Is(err, store.ErrTableFull):
		return errors.New("the table is full")
	case errors.Is(err, store.ErrNotFound):
		return errors.New("you did not play in this match")
	default:
		return err
	}
}
