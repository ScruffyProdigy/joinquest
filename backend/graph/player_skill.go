package graph

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/graph/model"
	"github.com/scruffyprodigy/joinquest/internal/auth"
	"github.com/scruffyprodigy/joinquest/internal/rating"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

// serviceScopedGameID returns the catalog game a request's service token speaks
// for, and whether the request has one at all.
//
// Both halves are load-bearing. GameServiceFromContext is what separates a game
// server from a browser: nothing a player's own session can set it, which is
// what keeps skill off the client-visible path. The scoped game id is what
// keeps a game inside its own catalog entry — Lobby's legacy global service
// token authenticates without naming a game, and "some game server" is not an
// answer to whose rating this is, so it reads as no access rather than as
// access to every game's.
//
// This deliberately does not honour requireGameServiceAuth's open-when-unset
// development bypass. That bypass lets a local stack run with no token
// configured; here it would mean an unauthenticated request to a dev lobby
// reads skill, and a field whose entire justification is "only a game server
// sees this" should not have a configuration in which that is untrue.
func serviceScopedGameID(ctx context.Context) (uuid.UUID, bool) {
	if !auth.GameServiceFromContext(ctx) {
		return uuid.Nil, false
	}
	raw, ok := auth.GameServiceGameIDFromContext(ctx)
	if !ok {
		return uuid.Nil, false
	}
	gameID, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, false
	}
	return gameID, true
}

// playerSkills reads skill for a whole roster in one game and mode, keyed by
// user id. Every requested user gets an entry: an unrated player reports the
// prior rather than being left out, so a caller never has to special-case a
// player it is meeting for the first time.
//
// The mode is not verified here. Both callers already hold a mode from the
// catalog — the resolver checks the key it was handed, and provision reads the
// mode it is provisioning.
func playerSkills(ctx context.Context, st *store.Store, gameID uuid.UUID, modeKey string, userIDs []uuid.UUID) (map[uuid.UUID]rating.Skill, error) {
	rated, err := st.GetPlayerRatings(ctx, gameID, modeKey, userIDs)
	if err != nil {
		return nil, err
	}

	out := make(map[uuid.UUID]rating.Skill, len(userIDs))
	for _, id := range userIDs {
		if v, ok := rated[id]; ok {
			out[id] = rating.SkillOf(v.Mu, v.Sigma)
			continue
		}
		out[id] = rating.UnratedSkill()
	}
	return out, nil
}

// toGraphQLPlayerSkill maps the service-facing skill shape onto the GraphQL
// type. Kept in one place so the provision payload and the GraphQL field can
// never drift into quoting different numbers for the same rating.
func toGraphQLPlayerSkill(s rating.Skill) *model.PlayerSkill {
	return &model.PlayerSkill{
		Rating:      s.Rating,
		Uncertainty: s.Uncertainty,
	}
}

// resolvePlayerSkill backs PublicPlayer.skill.
//
// Every refusal here is a nil, not an error. A player reaching this field is
// not doing anything wrong — PublicPlayer is the same type behind Table.user
// and MatchSeat.user, so an ordinary lobby query can select it — and an error
// would fail their whole request over a field they are simply not entitled to.
// A game server that gets nil has asked about a mode it does not declare, or is
// not presenting a per-game service token; both are the caller's own state to
// fix, not a fact about the player.
func resolvePlayerSkill(ctx context.Context, r *Resolver, playerID, modeKey string) (*model.PlayerSkill, error) {
	gameID, ok := serviceScopedGameID(ctx)
	if !ok {
		return nil, nil
	}

	modeKey = strings.TrimSpace(modeKey)
	if modeKey == "" {
		return nil, nil
	}

	st, err := r.requireStore()
	if err != nil {
		return nil, err
	}

	userID, err := parseUUID(playerID, "player id")
	if err != nil {
		return nil, err
	}

	// Asked before the rating lookup so a typo'd mode key answers null instead
	// of a prior that looks like a real reading of a real mode.
	exists, err := st.GameModeExists(ctx, gameID, modeKey)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}

	skills, err := playerSkills(ctx, st, gameID, modeKey, []uuid.UUID{userID})
	if err != nil {
		return nil, err
	}
	return toGraphQLPlayerSkill(skills[userID]), nil
}
