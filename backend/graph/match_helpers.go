package graph

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/scruffyprodigy/joinquest/graph/model"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

// metricCompletionPreempted counts completions that found the session already ended.
// A steady trickle is the benign reportPlayerFinished race; a burst on a schedule is a
// cleanup job ending live sessions behind the lobby's back.
//
// It lives here rather than beside its use in match.resolvers.go because gqlgen rewrites
// the resolver files on every `make generate` and moves hand-written top-level
// declarations into a commented-out "one last chance" block at the end of the file. That
// leaves the package uncompilable, which fails generation itself — so the next schema
// change in any domain cannot be generated at all.
const metricCompletionPreempted = "lobby.match.completion_preempted"

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

// regroupGenericMessage is what a player is told about a regroup failure that is not one of
// the three they can act on. Deliberately says nothing: the alternative is the store's own
// "store: ..." text, which is an internal detail and sometimes a database error.
const regroupGenericMessage = "could not start another round"

// regroupClientError translates the store's regroup sentinel errors into a GraphQL error the
// client can branch on, instead of letting an internal "store: ..." string (or an opaque 500)
// reach the player.
//
// The three cases a player should experience differently carry a RegroupErrorCode in the
// error's `code` extension; the message beside it is copy, and rewording it changes nothing
// for the client (JQ-176). The client falls back to the game detail page on NO_REGROUP_MODE,
// and can tell "too early" (SESSION_NOT_FINISHED) apart from a full table (TABLE_FULL).
//
// Everything else — ErrNotFound included — is a failure the player cannot act on, so it gets
// no code and no detail. ErrNotFound in particular is *not* "you did not play in this match"
// here: PlayAgain has already run requireMatchParticipant by this point, so a not-found from
// the claim is some other row missing, and saying "you did not play" would be a lie.
func regroupClientError(err error) error {
	switch {
	case errors.Is(err, store.ErrNoRegroupMode):
		return regroupError(model.RegroupErrorCodeNoRegroupMode, "this match no longer has a mode to build a table from")
	case errors.Is(err, store.ErrSessionNotFinished):
		return regroupError(model.RegroupErrorCodeSessionNotFinished, "this match hasn't finished yet")
	case errors.Is(err, store.ErrTableFull):
		return regroupError(model.RegroupErrorCodeTableFull, "the table is full")
	default:
		log.Printf("playAgain: unclassified regroup failure: %v", err)
		return errors.New(regroupGenericMessage)
	}
}

// regroupError builds the wire form of a coded regroup failure. gqlgen's default error
// presenter returns a *gqlerror.Error unchanged, so the extensions arrive at the client as
// written — see graph/gql_server.go, which registers no presenter of its own.
func regroupError(code model.RegroupErrorCode, message string) *gqlerror.Error {
	return &gqlerror.Error{
		Message:    message,
		Extensions: map[string]any{"code": string(code)},
	}
}
