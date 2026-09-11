package graph

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/graph/model"
	"github.com/scruffyprodigy/joinquest/internal/pubsub"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

// sessionStatusCompleted is the game_sessions.status value CompleteSession writes. A
// match result is only "final" once the session reaches it; before that the same query
// serves a partial, still-playing view.
const sessionStatusCompleted = "completed"

// publishMatchEvent notifies matchResultUpdated subscribers that a match's result or
// regroup state changed. It is a no-op when pub/sub is not configured.
func (r *Resolver) publishMatchEvent(ctx context.Context, sessionID uuid.UUID, eventType string) error {
	if r.PubSub == nil {
		return nil
	}
	return pubsub.PublishMatchEvent(ctx, r.PubSub, sessionID.String(), pubsub.MatchEvent{Type: eventType})
}

// matchResultLoader is the per-caller state loadMatchResultModel is allowed to keep. A
// query resolver makes a fresh one and it does nothing; a live subscription keeps one for
// the life of the connection, where it is the difference between paying for the whole
// fan-out on every push and paying only for what the push can actually have changed
// (JQ-177).
//
// It is used from one goroutine at a time — the subscription's own — so it needs no lock.
type matchResultLoader struct {
	// wantsMode is read from the selection set rather than assumed. mode was added to
	// MatchResult after the screen was built and was loaded unconditionally, so a client
	// that never selects it — anything but the results screen — paid a catalog query per
	// push for a field it would not read.
	wantsMode bool
	// game is the catalog game the session was played in. game_sessions.game_id never
	// changes, so within one subscription this is a constant.
	game *store.Game
}

// newMatchResultLoader reads the client's selection set once, at the resolver boundary, so
// the decision is made where the field context still exists rather than inside the loader.
func newMatchResultLoader(ctx context.Context) *matchResultLoader {
	return &matchResultLoader{wantsMode: selectsMatchResultField(ctx, "mode")}
}

// selectsMatchResultField reports whether the MatchResult selection set below the current
// field includes name. It answers true when there is no field context to read — a direct
// caller such as a test gets the whole model rather than a silently thinner one.
func selectsMatchResultField(ctx context.Context, name string) bool {
	// GetOperationContext panics rather than returning nil, so both halves are checked
	// through the nil-safe accessors.
	if graphql.GetFieldContext(ctx) == nil || !graphql.HasOperationContext(ctx) {
		return true
	}
	for _, field := range graphql.CollectFieldsCtx(ctx, []string{"MatchResult"}) {
		if field.Name == name {
			return true
		}
	}
	return false
}

func (l *matchResultLoader) loadGame(ctx context.Context, st *store.Store, gameID uuid.UUID) (*store.Game, error) {
	if l.game != nil && l.game.ID == gameID {
		return l.game, nil
	}
	game, err := st.GetGameByID(ctx, gameID)
	if err != nil {
		return nil, err
	}
	l.game = game
	return game, nil
}

// loadMatchResultModel maps a match's stored outcome onto the GraphQL model. It is
// deliberately free of resolver state and does no authorization of its own — every caller
// gates on requireMatchParticipant first.
//
// viewerID is the caller, and it is only ever used for the fields that are answers to
// "how does *this* player get back in" — today that is groupPlay. The standings do not
// vary by viewer.
//
// loader may be nil, which means "load everything, keep nothing".
func loadMatchResultModel(ctx context.Context, st *store.Store, sessionID, viewerID uuid.UUID, loader *matchResultLoader) (*model.MatchResult, error) {
	if loader == nil {
		loader = &matchResultLoader{wantsMode: true}
	}
	result, err := st.GetMatchResult(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	roster, err := st.GetRegroupRoster(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	parties, err := st.GetArrivalParties(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	game, err := loader.loadGame(ctx, st, result.GameID)
	if err != nil {
		return nil, err
	}
	inviteCode, err := regroupInviteCode(ctx, st, sessionID, viewerID)
	if err != nil {
		return nil, err
	}

	var mode *model.GameMode
	if loader.wantsMode {
		if mode, err = matchResultMode(ctx, st, result.ModeID); err != nil {
			return nil, err
		}
	}

	// The return context is the lobby's isGroupPlay: "room" means this player reached the
	// match from a table they were sitting at with a group, anything else means they came
	// through the catalog queue on their own (JQ-232). It is stamped when the session
	// starts and never rewritten, which is what makes it usable here — room_tables.
	// session_id is cleared by the post-match reset, and game_sessions.regroup_table_id is
	// stamped by whichever player claims first, so neither can answer this per player.
	returnCtx, err := st.GetParticipantReturnContext(ctx, sessionID, viewerID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	groupPlay := returnCtx.Kind == store.ReturnKindRoom

	out := &model.MatchResult{
		MatchID:           sessionID.String(),
		Game:              ToGraphQLGame(game),
		Mode:              mode,
		Status:            toGraphQLMatchResultStatus(result.Status),
		Reported:          result.Status != nil,
		Complete:          result.SessionStatus == sessionStatusCompleted,
		EndedAt:           result.EndedAt,
		Participants:      make([]*model.MatchParticipantResult, 0, len(result.Participants)),
		RegroupInviteCode: inviteCode,
		GroupPlay:         groupPlay,
	}

	for i := range result.Participants {
		p := result.Participants[i]
		// A player who left is dropped rather than rendered. Nothing writes left_at today;
		// skipping the row keeps the rest of the standings readable if something ever does,
		// where a half-built entry would null the whole non-null payload and the frontend
		// would read that as "no result" and redirect the player off the screen.
		if p.LeftAt != nil {
			continue
		}
		entry := &model.MatchParticipantResult{
			User:       ToGraphQLPublicPlayer(&p.User),
			Finished:   p.FinishedAt != nil,
			FinishedAt: p.FinishedAt,
			// Stays nil when the game never reported this player: that is what
			// separates "the game says they are done" from "they walked back into
			// /return on their own".
			Reason:    toGraphQLPlayerFinishReason(p.Reason),
			Placement: p.Placement,
			Winner:    p.IsWinner,
			Regroup:   toGraphQLRegroupState(roster[p.UserID]),
			// Note this is false for the viewer's own row when they arrived alone:
			// SameArrivalParty refuses to match two empty parties, so "I came alone"
			// never reads as a group of one (JQ-291).
			ArrivalParty: store.SameArrivalParty(parties[viewerID], parties[p.UserID]),
		}
		if role := strings.TrimSpace(p.Role); role != "" {
			entry.Role = &role
		}
		out.Participants = append(out.Participants, entry)
	}
	return out, nil
}

// loadRegroupRosterEntries builds the identity-and-intent view of a finished match's
// roster: who played, in which seat, and whether they are coming back. It is what a
// regroup table exposes, and it deliberately does not go through loadMatchResultModel —
// the standings that model carries are exactly what must not reach a surface joinable by
// players who never played the match (JQ-174). Building a separate value means a field
// added to MatchParticipantResult later cannot arrive here by accident.
//
// The result is ordered by seat, then by user id. GetMatchResult returns participants
// ordered by placement, and passing that order through would hand a non-participant the
// finishing order of a match they never played — a shorter field list would not help.
//
// roomID scopes it to one arrival party: the people who queued from this table's room, and
// nobody else from the match (JQ-291). Scoping by the room rather than by the viewer is what
// makes the field answerable for a backfill joiner who never played — they have no arrival
// party of their own, and the ghosted pending seats they are shown are the party whose room
// they just walked into.
func loadRegroupRosterEntries(ctx context.Context, st *store.Store, sessionID, roomID uuid.UUID) ([]*model.RegroupRosterEntry, error) {
	result, err := st.GetMatchResult(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	roster, err := st.GetRegroupRoster(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	parties, err := st.GetArrivalParties(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	party := store.ArrivalPartyOfRoom(roomID)

	out := make([]*model.RegroupRosterEntry, 0, len(result.Participants))
	for i := range result.Participants {
		p := result.Participants[i]
		// Same skip as loadMatchResultModel: a participant who left is not on the roster,
		// and a nil user would null the whole non-null list.
		if p.LeftAt != nil {
			continue
		}
		// A table built for a player who arrived alone matches nobody, so its roster is
		// empty rather than listing the strangers they were matched with.
		if !store.SameArrivalParty(party, parties[p.UserID]) {
			continue
		}
		entry := &model.RegroupRosterEntry{
			User:    ToGraphQLPublicPlayer(&p.User),
			Regroup: toGraphQLRegroupState(roster[p.UserID]),
		}
		if role := strings.TrimSpace(p.Role); role != "" {
			entry.Role = &role
		}
		out = append(out, entry)
	}

	role := func(e *model.RegroupRosterEntry) string {
		if e.Role == nil {
			return ""
		}
		return *e.Role
	}
	sort.SliceStable(out, func(i, j int) bool {
		if ri, rj := role(out[i]), role(out[j]); ri != rj {
			return ri < rj
		}
		return out[i].User.ID < out[j].User.ID
	})
	return out, nil
}

// matchResultMode resolves the catalog mode the session was played in. Both "the session
// never named a mode" and "the mode it named has since been deleted" resolve to nil: the
// schema field is nullable precisely because sessions outlive modes (JQ-134), and a
// missing mode must not fail the whole results screen.
func matchResultMode(ctx context.Context, st *store.Store, modeID *uuid.UUID) (*model.GameMode, error) {
	if modeID == nil {
		return nil, nil
	}
	mode, err := st.GetGameModeByID(ctx, *modeID)
	if err != nil {
		// GetGameModeByID returns raw sql.ErrNoRows, not store.ErrNotFound, so both have to
		// be caught here or the degradation this comment promises would in fact error.
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return ToGraphQLGameMode(mode), nil
}

// regroupInviteCode resolves the room code of the table the viewer's own arrival party
// converged on, or nil when that party has claimed no table yet.
//
// Viewer-scoped since a match has one regroup table per arrival party (JQ-291). It is the
// code the results screen renders as a link, so answering it session-wide would invite the
// group that lost the 3v3 into the winners' room.
func regroupInviteCode(ctx context.Context, st *store.Store, sessionID, viewerID uuid.UUID) (*string, error) {
	tableID, err := st.GetRegroupTableIDForUser(ctx, sessionID, viewerID)
	if err != nil {
		return nil, err
	}
	if tableID == nil {
		return nil, nil
	}
	table, err := st.GetRoomTableByID(ctx, *tableID)
	if err != nil {
		return nil, err
	}
	room, err := st.GetRoomByID(ctx, table.RoomID)
	if err != nil {
		return nil, err
	}
	code := room.InviteCode
	return &code, nil
}

func toGraphQLRegroupState(state store.RegroupState) model.RegroupState {
	switch state {
	case store.RegroupIn:
		return model.RegroupStateIn
	case store.RegroupOut:
		return model.RegroupStateOut
	default:
		// Missing from the roster map reads as PENDING: nobody has answered.
		return model.RegroupStatePending
	}
}

// toGraphQLMatchResultStatus maps the stored status string back onto the enum the game
// reported it as. An unrecognized value reads as no status rather than an enum the
// schema cannot serialize.
func toGraphQLMatchResultStatus(raw *string) *model.MatchResultStatus {
	if raw == nil {
		return nil
	}
	status := model.MatchResultStatus(strings.ToUpper(strings.TrimSpace(*raw)))
	if !status.IsValid() {
		return nil
	}
	return &status
}

func toGraphQLPlayerFinishReason(raw *string) *model.PlayerFinishReason {
	if raw == nil {
		return nil
	}
	reason := model.PlayerFinishReason(strings.ToUpper(strings.TrimSpace(*raw)))
	if !reason.IsValid() {
		return nil
	}
	return &reason
}
