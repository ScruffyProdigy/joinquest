package graph

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/playhub/internal/auth"
	"github.com/scruffyprodigy/playhub/internal/gameclient"
	"github.com/scruffyprodigy/playhub/internal/gameurl"
	"github.com/scruffyprodigy/playhub/internal/store"
)

func (r *Resolver) gameProvisioner() gameclient.MatchProvisioner {
	if r.GameProvisioner != nil {
		return r.GameProvisioner
	}
	return gameclient.NewClient()
}

func handoffDebugEnabled() bool {
	v := strings.TrimSpace(os.Getenv("LOBBY_HANDOFF_DEBUG"))
	return v == "1" || strings.EqualFold(v, "true")
}

func (r *Resolver) resolvedAPIBaseURL(game *store.Game) string {
	if game != nil && game.APIBaseURL != nil {
		if u := strings.TrimSpace(*game.APIBaseURL); u != "" {
			return strings.TrimRight(u, "/")
		}
	}
	return strings.TrimRight(strings.TrimSpace(os.Getenv("GAME_API_BASE_URL")), "/")
}

func modeKeyFromCatalog(mode *store.GameMode) (string, error) {
	if mode == nil || strings.TrimSpace(mode.ModeKey) == "" {
		return "", fmt.Errorf("catalog mode is required")
	}
	return strings.TrimSpace(mode.ModeKey), nil
}

func assignmentFromParticipants(
	ctx context.Context,
	st *store.Store,
	sessionID uuid.UUID,
	mode *store.GameMode,
	participants []store.SessionParticipant,
) (gameclient.Assignment, error) {
	modeKey, err := modeKeyFromCatalog(mode)
	if err != nil {
		return gameclient.Assignment{}, err
	}
	assignment := gameclient.Assignment{
		ExternalMatchID: sessionID.String(),
		GameMode:        modeKey,
		Seats:           make([]gameclient.AssignmentSeat, 0, len(participants)),
	}
	for _, p := range participants {
		seat := gameclient.AssignmentSeat{
			SeatKey:     p.SeatKey,
			LobbyUserID: p.UserID.String(),
		}
		user, err := st.GetUserByID(ctx, p.UserID)
		if err == nil && user != nil {
			seat.Player = provisionPlayerFromUser(user)
		}
		assignment.Seats = append(assignment.Seats, seat)
	}
	return assignment, nil
}

func provisionPlayerFromUser(user *store.User) *gameclient.ProvisionPlayer {
	if user == nil {
		return nil
	}
	out := &gameclient.ProvisionPlayer{}
	if name := user.ChosenDisplayName(); name != "" {
		out.DisplayName = name
	}
	if url := userAvatarURL(user); url != nil {
		if trimmed := strings.TrimSpace(*url); trimmed != "" {
			out.AvatarURL = trimmed
		}
	}
	if out.DisplayName == "" && out.AvatarURL == "" {
		return nil
	}
	return out
}

func lobbyProvisionInfo(game *store.Game) (gameclient.LobbyInfo, error) {
	info := gameclient.LobbyInfo{
		ReturnURL:  auth.LobbyReturnURL(),
		GraphqlURL: auth.LobbyGraphQLURL(),
	}
	if token, err := auth.FormatGameServiceToken(game.ID); err == nil {
		info.ServiceToken = token
		return info, nil
	}
	if legacy := auth.GameServiceTokenFromEnv(); legacy != "" {
		info.ServiceToken = legacy
		return info, nil
	}
	if auth.IsProductionEnv() {
		return gameclient.LobbyInfo{}, fmt.Errorf("game service token is not configured (set LOBBY_GAME_TOKEN_PEPPER or LOBBY_GAME_SERVICE_TOKEN)")
	}
	return info, nil
}

type provisionOutcome struct {
	result gameclient.ProvisionResult
}

func (r *Resolver) provisionParticipantsOnGame(ctx context.Context, game *store.Game, sessionID uuid.UUID, participants []store.SessionParticipant) (provisionOutcome, error) {
	apiBase := r.resolvedAPIBaseURL(game)
	if apiBase == "" {
		return provisionOutcome{}, fmt.Errorf("game API base URL is not configured (set games.api_base_url or GAME_API_BASE_URL)")
	}
	if err := gameurl.ValidateOutboundURL(ctx, apiBase, auth.IsProductionEnv()); err != nil {
		return provisionOutcome{}, fmt.Errorf("game API base URL: %w", err)
	}
	if len(participants) == 0 {
		return provisionOutcome{}, fmt.Errorf("cannot provision match with no seated players")
	}

	st, err := r.requireStore()
	if err != nil {
		return provisionOutcome{}, err
	}

	mode, err := st.GetGameModeForSession(ctx, sessionID)
	if err != nil {
		return provisionOutcome{}, fmt.Errorf("catalog mode for session: %w", err)
	}
	assignment, err := assignmentFromParticipants(ctx, st, sessionID, mode, participants)
	if err != nil {
		return provisionOutcome{}, err
	}

	gameSlug := ""
	if game != nil && game.Slug != nil {
		gameSlug = strings.TrimSpace(*game.Slug)
	}
	start := time.Now()
	log.Printf("handoff: provision start session=%s game=%s externalMatchId=%s seats=%d api=%s",
		sessionID, gameSlug, sessionID, len(assignment.Seats), apiBase)
	if handoffDebugEnabled() {
		log.Printf("handoff: POST %s/api/v1/matches externalMatchId=%s seats=%d", apiBase, sessionID, len(assignment.Seats))
	}

	lobby, err := lobbyProvisionInfo(game)
	if err != nil {
		return provisionOutcome{}, err
	}
	result, err := r.gameProvisioner().ProvisionMatch(ctx, gameclient.ProvisionRequest{
		APIBaseURL:   apiBase,
		ServiceToken: lobby.ServiceToken,
		LobbyID:      auth.LobbyIssuer(),
		Lobby:        lobby,
		Assignment:   assignment,
	})
	latency := time.Since(start)
	if err != nil {
		if banned, ok := err.(*gameclient.BannedPlayersError); ok {
			log.Printf("handoff: provision banned session=%s game=%s latency_ms=%d banned=%d",
				sessionID, gameSlug, latency.Milliseconds(), len(banned.BannedLobbyUserIDs))
			st, stErr := r.requireStore()
			if stErr != nil {
				return provisionOutcome{}, stErr
			}
			bannedIDs := parseBannedLobbyUserIDs(banned.BannedLobbyUserIDs)
			if rbErr := st.RollbackMatchedSession(ctx, sessionID, bannedIDs); rbErr != nil {
				return provisionOutcome{}, fmt.Errorf("provision banned and rollback failed: %w (rollback: %v)", err, rbErr)
			}
		} else {
			log.Printf("handoff: provision fail session=%s game=%s latency_ms=%d err=%v",
				sessionID, gameSlug, latency.Milliseconds(), err)
		}
		return provisionOutcome{}, err
	}

	source := "none"
	if len(result.LaunchURLs) > 0 {
		source = "game"
	} else if result.LaunchURLTemplate != "" {
		source = "game-template"
	}
	log.Printf("handoff: provision ok session=%s game=%s latency_ms=%d launch_urls=%d source=%s",
		sessionID, gameSlug, latency.Milliseconds(), len(result.LaunchURLs), source)
	return provisionOutcome{result: result}, nil
}

// finalizeMatchedSession pushes the roster to the game and returns per-user launch URLs.
// Only this path (and table start) call the game provision API for queue matches.
func (r *Resolver) finalizeMatchedSession(ctx context.Context, game *store.Game, sessionID uuid.UUID, notifyUserIDs []uuid.UUID) (map[uuid.UUID]string, error) {
	unlock := lockSessionProvision(sessionID)
	defer unlock()

	st, err := r.requireStore()
	if err != nil {
		return nil, err
	}
	authService, err := r.requireAuth()
	if err != nil {
		return nil, err
	}

	participants, err := st.ListSessionSeatAssignments(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	audience := r.resolvedAPIBaseURL(game)
	if audience == "" {
		return nil, fmt.Errorf("game API base URL is not configured")
	}

	complete, err := st.SessionProvisionComplete(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	var bases map[uuid.UUID]string
	if complete {
		bases, err = st.ListSessionParticipantLaunchURLBases(ctx, sessionID)
		if err != nil {
			return nil, err
		}
		log.Printf("handoff: finalize session=%s source=stored players=%d", sessionID, len(bases))
	} else {
		outcome, provErr := r.provisionParticipantsOnGame(ctx, game, sessionID, participants)
		if provErr != nil {
			return nil, provErr
		}
		var source string
		bases, source, err = r.launchURLBasesForParticipants(ctx, sessionID, participants, outcome.result)
		if err != nil {
			log.Printf("handoff: launch url bases fail session=%s source=%s err=%v", sessionID, source, err)
			return nil, err
		}
		log.Printf("handoff: finalize launch bases session=%s source=%s players=%d", sessionID, source, len(bases))

		if err := st.SetSessionParticipantLaunchURLBases(ctx, sessionID, bases); err != nil {
			log.Printf("handoff: persist launch url bases session=%s err=%v", sessionID, err)
			return nil, fmt.Errorf("persist launch url bases: %w", err)
		}
	}

	signer := authService.Signer()
	externalMatchID := sessionID.String()
	urls := make(map[uuid.UUID]string, len(participants))
	for _, p := range participants {
		base, ok := bases[p.UserID]
		if !ok || base == "" {
			return nil, fmt.Errorf("missing launch url base for user %s", p.UserID)
		}
		launch, err := signedLaunchURLFromBase(signer, audience, externalMatchID, base, p.UserID, p.SeatKey, p.DisplayName)
		if err != nil {
			return nil, err
		}
		urls[p.UserID] = launch
	}

	for _, uid := range notifyUserIDs {
		if _, ok := urls[uid]; !ok {
			return nil, fmt.Errorf("missing launch url for notified user %s", uid)
		}
	}
	log.Printf("handoff: finalize ok session=%s notified=%d", sessionID, len(notifyUserIDs))
	return urls, nil
}

func (r *Resolver) launchURLBasesForParticipants(
	ctx context.Context,
	sessionID uuid.UUID,
	participants []store.SessionParticipant,
	provision gameclient.ProvisionResult,
) (map[uuid.UUID]string, string, error) {
	externalMatchID := sessionID.String()
	bases := make(map[uuid.UUID]string, len(participants))

	if len(provision.LaunchURLs) > 0 {
		for _, p := range participants {
			raw, ok := provision.LaunchURLs[p.UserID.String()]
			if !ok || strings.TrimSpace(raw) == "" {
				return nil, "game", fmt.Errorf("game omitted launch url for user %s", p.UserID)
			}
			if err := validateGameLaunchURL(ctx, raw); err != nil {
				return nil, "game", fmt.Errorf("game launch url for user %s: %w", p.UserID, err)
			}
			bases[p.UserID] = raw
		}
		return bases, "game", nil
	}

	if tmpl := strings.TrimSpace(provision.LaunchURLTemplate); tmpl != "" {
		for _, p := range participants {
			raw, err := expandLaunchURLTemplate(tmpl, externalMatchID, p.UserID, p.SeatKey)
			if err != nil {
				return nil, "game-template", err
			}
			if err := validateGameLaunchURL(ctx, raw); err != nil {
				return nil, "game-template", fmt.Errorf("template launch url for user %s: %w", p.UserID, err)
			}
			bases[p.UserID] = raw
		}
		return bases, "game-template", nil
	}

	return nil, "", fmt.Errorf("game must return launchUrls or launchUrlTemplate on provision")
}

func validateGameLaunchURL(ctx context.Context, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("empty launch url")
	}
	return gameurl.ValidateOutboundURL(ctx, raw, auth.IsProductionEnv())
}

func expandLaunchURLTemplate(tmpl, externalMatchID string, userID uuid.UUID, seatKey string) (string, error) {
	out := tmpl
	out = strings.ReplaceAll(out, "{matchId}", externalMatchID)
	out = strings.ReplaceAll(out, "{externalMatchId}", externalMatchID)
	out = strings.ReplaceAll(out, "{seatKey}", seatKey)
	out = strings.ReplaceAll(out, "{lobbyUserId}", userID.String())
	return out, nil
}

func signedLaunchURLFromBase(
	signer *auth.Signer,
	audience, externalMatchID, base string,
	userID uuid.UUID,
	seatKey, displayName string,
) (string, error) {
	token, err := signer.SignSeatToken(userID, audience, externalMatchID, seatKey, displayName, 0)
	if err != nil {
		return "", err
	}
	launch, err := gameurl.AttachSeatToken(base, token)
	if err != nil {
		return "", err
	}
	if handoffDebugEnabled() {
		log.Printf("handoff: signed launch url user=%s host=%s", userID, urlHost(launch))
	}
	return launch, nil
}

func parseBannedLobbyUserIDs(ids []string) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ids))
	for _, s := range ids {
		id, err := uuid.Parse(strings.TrimSpace(s))
		if err != nil {
			continue
		}
		out = append(out, id)
	}
	return out
}

func urlHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Host
}

// signLaunchURL returns a signed launch URL from stored game-minted bases only.
// Game provision is owned by the forming worker — not triggered from queries/subscriptions.
func (r *Resolver) signLaunchURL(ctx context.Context, game *store.Game, sessionID, userID uuid.UUID) (string, error) {
	st, err := r.requireStore()
	if err != nil {
		return "", err
	}

	participants, err := st.ListSessionSeatAssignments(ctx, sessionID)
	if err != nil {
		return "", err
	}
	if len(participants) == 0 {
		return "", fmt.Errorf("session has no seated participants")
	}

	base, err := st.GetSessionParticipantLaunchURLBase(ctx, sessionID, userID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(base) == "" {
		return "", nil
	}
	log.Printf("handoff: launch url mint session=%s user=%s source=stored", sessionID, userID)
	return r.mintLaunchURLForUserFromBase(ctx, game, sessionID, userID, base, participants)
}

func (r *Resolver) mintLaunchURLForUserFromBase(
	ctx context.Context,
	game *store.Game,
	sessionID, userID uuid.UUID,
	base string,
	participants []store.SessionParticipant,
) (string, error) {
	authService, err := r.requireAuth()
	if err != nil {
		return "", err
	}
	audience := r.resolvedAPIBaseURL(game)
	for _, p := range participants {
		if p.UserID == userID {
			return signedLaunchURLFromBase(authService.Signer(), audience, sessionID.String(), base, userID, p.SeatKey, p.DisplayName)
		}
	}
	return "", fmt.Errorf("user is not seated in session")
}
