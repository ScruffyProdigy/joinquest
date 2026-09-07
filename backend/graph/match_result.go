package graph

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/playhub/graph/model"
	"github.com/scruffyprodigy/playhub/internal/pubsub"
	"github.com/scruffyprodigy/playhub/internal/store"
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

// loadMatchResultModel maps a match's stored outcome onto the GraphQL model. It is
// deliberately free of resolver state and does no authorization of its own — every caller
// gates on requireMatchParticipant first.
func loadMatchResultModel(ctx context.Context, st *store.Store, sessionID uuid.UUID) (*model.MatchResult, error) {
	result, err := st.GetMatchResult(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	roster, err := st.GetRegroupRoster(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	game, err := st.GetGameByID(ctx, result.GameID)
	if err != nil {
		return nil, err
	}
	inviteCode, err := regroupInviteCode(ctx, st, sessionID)
	if err != nil {
		return nil, err
	}
	players, err := st.ListSessionParticipants(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]*store.User, len(players))
	for i := range players {
		byID[players[i].ID] = &players[i]
	}

	out := &model.MatchResult{
		MatchID:           sessionID.String(),
		Game:              ToGraphQLGame(game),
		Status:            toGraphQLMatchResultStatus(result.Status),
		Reported:          result.Status != nil,
		Complete:          result.SessionStatus == sessionStatusCompleted,
		EndedAt:           result.EndedAt,
		Participants:      make([]*model.MatchParticipantResult, 0, len(result.Participants)),
		RegroupInviteCode: inviteCode,
	}

	for i := range result.Participants {
		p := result.Participants[i]
		entry := &model.MatchParticipantResult{
			User:       ToGraphQLPublicPlayer(byID[p.UserID]),
			Finished:   p.FinishedAt != nil,
			FinishedAt: p.FinishedAt,
			// Stays nil when the game never reported this player: that is what
			// separates "the game says they are done" from "they walked back into
			// /return on their own".
			Reason:    toGraphQLPlayerFinishReason(p.Reason),
			Placement: p.Placement,
			Winner:    p.IsWinner,
			Regroup:   toGraphQLRegroupState(roster[p.UserID]),
		}
		if role := strings.TrimSpace(p.Role); role != "" {
			entry.Role = &role
		}
		out.Participants = append(out.Participants, entry)
	}
	return out, nil
}

// regroupInviteCode resolves the room code of the table this match converged on, or nil
// when no table has been claimed yet.
func regroupInviteCode(ctx context.Context, st *store.Store, sessionID uuid.UUID) (*string, error) {
	tableID, err := st.GetRegroupTableID(ctx, sessionID)
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
