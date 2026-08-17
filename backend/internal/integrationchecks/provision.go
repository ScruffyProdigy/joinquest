package integrationchecks

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/playhub/internal/auth"
	"github.com/scruffyprodigy/playhub/internal/gameclient"
	"github.com/scruffyprodigy/playhub/internal/store"
)

const (
	checkMatchIDPrefix     = "joinquest-check-"
	checkUserOne           = "a0000000-0000-4000-8000-000000000001"
	checkUserTwo           = "a0000000-0000-4000-8000-000000000002"
	checkBannedUser        = "a0000000-0000-4000-8000-000000000099"
	wrongAudienceBaseURL   = "https://joinquest-check-invalid.example"
)

// ProvisionConfig carries Lobby-side values for synthetic provision checks.
type ProvisionConfig struct {
	LobbyID      string
	Lobby        gameclient.LobbyInfo
	ServiceToken string
}

// ProvisionCheckOutcome includes JWT follow-up context from a successful happy path.
type ProvisionCheckOutcome struct {
	Results       []Result
	MatchID       string
	SeatKey       string
	SecondSeatKey string
}

// RunProvisionChecks exercises provision, auth, banlist shape, and launch URLs.
func (r *Runner) RunProvisionChecks(
	ctx context.Context,
	game *store.Game,
	modes []store.GameMode,
	seatsByMode map[uuid.UUID][]store.GameModeSeat,
	provisioner gameclient.MatchProvisioner,
	cfg ProvisionConfig,
) ProvisionCheckOutcome {
	if game == nil || game.APIBaseURL == nil {
		return ProvisionCheckOutcome{Results: provisionUnreachable("No API base URL is configured for this game.")}
	}
	if provisioner == nil {
		provisioner = gameclient.NewClient()
	}

	mode, seats, ok := pickCheckMode(modes, seatsByMode)
	if !ok {
		return ProvisionCheckOutcome{Results: []Result{{
			CheckID: "provision.happy_path",
			Status:  StatusFail,
			Message: "No active game mode with seats found — sync your manifest first.",
		}}}
	}

	apiBase := strings.TrimRight(strings.TrimSpace(*game.APIBaseURL), "/")
	matchID := checkMatchIDPrefix + uuid.NewString()
	assignment := syntheticAssignment(mode, seats, matchID, []string{checkUserOne, checkUserTwo})
	firstSeatKey := assignment.Seats[0].SeatKey
	secondSeatKey := ""
	if len(assignment.Seats) > 1 {
		secondSeatKey = assignment.Seats[1].SeatKey
	}

	results := make([]Result, 0, 8)

	// Happy path
	happyReq := gameclient.ProvisionRequest{
		APIBaseURL:   apiBase,
		ServiceToken: cfg.ServiceToken,
		LobbyID:      cfg.LobbyID,
		Lobby:        cfg.Lobby,
		Assignment:   assignment,
	}
	result, err := provisioner.ProvisionMatch(ctx, happyReq)
	if err != nil {
		results = append(results, Result{
			CheckID: "provision.happy_path",
			Status:  StatusFail,
			Message: fmt.Sprintf("Provision failed: %s", friendlyProvisionError(err)),
		})
		results = append(results,
			skipped("provision.auth", "Fix the happy-path provision first."),
			skipped("provision.missing_auth", "Fix the happy-path provision first."),
			skipped("provision.banlist", "Fix the happy-path provision first."),
			skipped("provision.launch_urls", "Fix the happy-path provision first."),
			skipped("provision.launch_url_no_jwt", "Fix the happy-path provision first."),
			skipped("provision.idempotent_repush", "Fix the happy-path provision first."),
		)
		return ProvisionCheckOutcome{Results: results}
	}

	results = append(results, Result{
		CheckID: "provision.happy_path",
		Status:  StatusPass,
		Message: "We provisioned a synthetic test match successfully.",
	})

	// Idempotent re-push with the same externalMatchId.
	if _, err := provisioner.ProvisionMatch(ctx, happyReq); err != nil {
		results = append(results, Result{
			CheckID: "provision.idempotent_repush",
			Status:  StatusFail,
			Message: fmt.Sprintf("Re-provisioning the same match id should succeed (idempotent): %s", friendlyProvisionError(err)),
		})
	} else {
		results = append(results, Result{
			CheckID: "provision.idempotent_repush",
			Status:  StatusPass,
			Message: "Re-provisioning the same externalMatchId succeeded.",
		})
	}

	// Launch URLs
	if len(result.LaunchURLs) >= len(assignment.Seats) || result.LaunchURLTemplate != "" {
		results = append(results, Result{
			CheckID: "provision.launch_urls",
			Status:  StatusPass,
			Message: "Provision returned launch URL bases for seated players.",
		})
	} else {
		results = append(results, Result{
			CheckID: "provision.launch_urls",
			Status:  StatusFail,
			Message: fmt.Sprintf("Provision succeeded but launchUrls only covered %d of %d seats.", len(result.LaunchURLs), len(assignment.Seats)),
		})
	}

	if launchURLsContainJWT(result.LaunchURLs, result.LaunchURLTemplate) {
		results = append(results, Result{
			CheckID: "provision.launch_url_no_jwt",
			Status:  StatusFail,
			Message: "Launch URL bases must not include a JWT — JoinQuest attaches token= when linking players.",
		})
	} else {
		results = append(results, Result{
			CheckID: "provision.launch_url_no_jwt",
			Status:  StatusPass,
			Message: "Launch URL bases do not embed a seat JWT.",
		})
	}

	// Auth — wrong token should not succeed
	badAuthReq := happyReq
	badAuthReq.ServiceToken = "Bearer invalid-check-token"
	if _, err := provisioner.ProvisionMatch(ctx, badAuthReq); err == nil {
		results = append(results, Result{
			CheckID: "provision.auth",
			Status:  StatusFail,
			Message: "Your server accepted a provision request with an invalid service token.",
		})
	} else if strings.Contains(strings.ToLower(err.Error()), "401") || strings.Contains(strings.ToLower(err.Error()), "403") {
		results = append(results, Result{
			CheckID: "provision.auth",
			Status:  StatusPass,
			Message: "Your server rejected our invalid service token.",
		})
	} else {
		results = append(results, Result{
			CheckID: "provision.auth",
			Status:  StatusPass,
			Message: "Your server rejected our invalid service token.",
		})
	}

	// Auth — missing Authorization when service token is required
	noAuthReq := happyReq
	noAuthReq.ServiceToken = ""
	if _, err := provisioner.ProvisionMatch(ctx, noAuthReq); err == nil {
		results = append(results, Result{
			CheckID: "provision.missing_auth",
			Status:  StatusFail,
			Message: "Your server accepted a provision request with no Authorization header.",
		})
	} else {
		results = append(results, Result{
			CheckID: "provision.missing_auth",
			Status:  StatusPass,
			Message: "Your server rejected a provision request with no Authorization header.",
		})
	}

	// Banlist — optional unless dev bans the test user
	banAssignment := syntheticAssignment(mode, seats, checkMatchIDPrefix+"ban-"+uuid.NewString(), []string{checkUserOne, checkBannedUser})
	banReq := happyReq
	banReq.Assignment = banAssignment
	if _, err := provisioner.ProvisionMatch(ctx, banReq); err != nil {
		if banned, ok := err.(*gameclient.BannedPlayersError); ok {
			if len(banned.BannedLobbyUserIDs) > 0 {
				results = append(results, Result{
					CheckID: "provision.banlist",
					Status:  StatusPass,
					Message: "Your server returned 403 with bannedLobbyUserIds as expected.",
				})
			} else {
				results = append(results, Result{
					CheckID: "provision.banlist",
					Status:  StatusFail,
					Message: "Your server returned 403 but bannedLobbyUserIds was missing or empty.",
				})
			}
		} else {
			results = append(results, Result{
				CheckID: "provision.banlist",
				Status:  StatusSkipped,
				Message: fmt.Sprintf("Banlist check skipped — temporarily ban lobby user %s during checks to verify 403 shape.", checkBannedUser),
			})
		}
	} else {
		results = append(results, Result{
			CheckID: "provision.banlist",
			Status:  StatusSkipped,
			Message: fmt.Sprintf("Banlist check skipped — temporarily ban lobby user %s during checks to verify 403 shape.", checkBannedUser),
		})
	}

	return ProvisionCheckOutcome{Results: results, MatchID: matchID, SeatKey: firstSeatKey, SecondSeatKey: secondSeatKey}
}

func launchURLsContainJWT(urls map[string]string, template string) bool {
	for _, raw := range urls {
		if strings.Contains(strings.ToLower(raw), "token=") || strings.Contains(raw, "eyJ") {
			return true
		}
	}
	t := strings.ToLower(template)
	return strings.Contains(t, "token=") || strings.Contains(template, "eyJ")
}

// RunJWTChecks exercises claim endpoints using a provisioned synthetic match.
func (r *Runner) RunJWTChecks(ctx context.Context, game *store.Game, signer *auth.Signer, provisionMatchID, seatKey, secondSeatKey string) []Result {
	if game == nil || game.APIBaseURL == nil {
		return jwtSkippedAll("Connect your API before JWT checks.")
	}
	if signer == nil {
		return jwtSkippedAll("Lobby JWT signer is not configured.")
	}
	if provisionMatchID == "" || seatKey == "" {
		return jwtSkippedAll("Run provision checks first.")
	}

	apiBase := strings.TrimRight(strings.TrimSpace(*game.APIBaseURL), "/")
	audience := apiBase

	results := make([]Result, 0, 9)

	jwksURL := strings.TrimRight(auth.LobbyIssuer(), "/") + "/.well-known/jwks.json"
	if err := r.checkJWKSReachable(ctx, jwksURL); err != nil {
		results = append(results, Result{
			CheckID: "jwt.jwks",
			Status:  StatusFail,
			Message: fmt.Sprintf("We couldn't reach Lobby JWKS at %s — %v", jwksURL, err),
		})
	} else {
		results = append(results, Result{
			CheckID: "jwt.jwks",
			Status:  StatusPass,
			Message: "Lobby JWKS is reachable for your game to verify tokens.",
		})
	}

	userID := uuid.MustParse(checkUserOne)
	token, err := signer.SignSeatToken(userID, audience, provisionMatchID, seatKey, "JoinQuest Check", time.Hour)
	if err != nil {
		results = append(results, Result{
			CheckID: "jwt.claim_happy_path",
			Status:  StatusSkipped,
			Message: "Could not mint a test seat token.",
		})
		return append(results, jwtSkippedRest("Fix token minting first.")...)
	}

	claimURL := fmt.Sprintf("%s/api/v1/matches/%s/claim", apiBase, provisionMatchID)
	status, body := r.postClaim(ctx, claimURL, token)
	switch {
	case status >= 200 && status < 300:
		results = append(results, Result{
			CheckID: "jwt.claim_happy_path",
			Status:  StatusPass,
			Message: "Claim succeeded with a valid seat token.",
		})
	case status == 404:
		results = append(results, Result{
			CheckID: "jwt.claim_happy_path",
			Status:  StatusFail,
			Message: "Claim returned 404 — the synthetic test match may have expired. Re-run checks.",
		})
	default:
		results = append(results, Result{
			CheckID: "jwt.claim_happy_path",
			Status:  StatusFail,
			Message: fmt.Sprintf("Claim failed (HTTP %d): %s", status, strings.TrimSpace(body)),
		})
	}

	badToken, err := signer.SignSeatToken(userID, wrongAudienceBaseURL, provisionMatchID, seatKey, "", time.Hour)
	if err == nil {
		status, _ := r.postClaim(ctx, claimURL, badToken)
		if status == 401 || status == 403 {
			results = append(results, Result{
				CheckID: "jwt.wrong_audience",
				Status:  StatusPass,
				Message: "Your game rejected a token with the wrong audience.",
			})
		} else {
			results = append(results, Result{
				CheckID: "jwt.wrong_audience",
				Status:  StatusFail,
				Message: fmt.Sprintf("Your game should reject wrong aud with 401/403, got HTTP %d.", status),
			})
		}
	}

	bogusURL := fmt.Sprintf("%s/api/v1/matches/%s/claim", apiBase, checkMatchIDPrefix+"missing-"+uuid.NewString())
	status, _ = r.postClaim(ctx, bogusURL, token)
	if status == 404 {
		results = append(results, Result{
			CheckID: "jwt.unknown_match",
			Status:  StatusPass,
			Message: "Unknown match returned 404 as expected.",
		})
	} else {
		results = append(results, Result{
			CheckID: "jwt.unknown_match",
			Status:  StatusFail,
			Message: fmt.Sprintf("Unknown match should return 404, got HTTP %d.", status),
		})
	}

	wrongIssToken, err := signer.SignSeatTokenWithIssuer(userID, "https://joinquest-check-wrong-issuer.example", audience, provisionMatchID, seatKey, "", time.Hour)
	if err == nil {
		status, _ = r.postClaim(ctx, claimURL, wrongIssToken)
		if status == 401 || status == 403 {
			results = append(results, Result{
				CheckID: "jwt.wrong_issuer",
				Status:  StatusPass,
				Message: "Your game rejected a token with the wrong issuer.",
			})
		} else {
			results = append(results, Result{
				CheckID: "jwt.wrong_issuer",
				Status:  StatusFail,
				Message: fmt.Sprintf("Your game should reject wrong iss with 401/403, got HTTP %d.", status),
			})
		}
	}

	expiredToken, err := signer.SignSeatToken(userID, audience, provisionMatchID, seatKey, "", -time.Hour)
	if err == nil {
		status, _ = r.postClaim(ctx, claimURL, expiredToken)
		if status == 401 || status == 403 {
			results = append(results, Result{
				CheckID: "jwt.expired",
				Status:  StatusPass,
				Message: "Your game rejected an expired seat token.",
			})
		} else {
			results = append(results, Result{
				CheckID: "jwt.expired",
				Status:  StatusFail,
				Message: fmt.Sprintf("Expired token should be rejected with 401/403, got HTTP %d.", status),
			})
		}
	}

	status, _ = r.postClaim(ctx, claimURL, "not.a.valid.jwt")
	if status == 401 || status == 403 {
		results = append(results, Result{
			CheckID: "jwt.invalid_token",
			Status:  StatusPass,
			Message: "Your game rejected a malformed seat token.",
		})
	} else {
		results = append(results, Result{
			CheckID: "jwt.invalid_token",
			Status:  StatusFail,
			Message: fmt.Sprintf("Malformed token should be rejected with 401/403, got HTTP %d.", status),
		})
	}

	if secondSeatKey != "" && secondSeatKey != seatKey {
		wrongSeatToken, err := signer.SignSeatToken(userID, audience, provisionMatchID, secondSeatKey, "", time.Hour)
		if err == nil {
			status, _ = r.postClaim(ctx, claimURL, wrongSeatToken)
			if status == 401 || status == 403 {
				results = append(results, Result{
					CheckID: "jwt.wrong_seat",
					Status:  StatusPass,
					Message: "Your game rejected a token for another player's reserved seat.",
				})
			} else {
				results = append(results, Result{
					CheckID: "jwt.wrong_seat",
					Status:  StatusFail,
					Message: fmt.Sprintf("Token for another seat should be rejected with 401/403, got HTTP %d.", status),
				})
			}
		}
	} else {
		results = append(results, skipped("jwt.wrong_seat", "No second seat in test mode."))
	}

	if kid, rotPriv, cleanup, rotErr := signer.AddRotationKey(); rotErr == nil {
		func() {
			defer cleanup()

			primaryToken, primaryErr := signer.SignSeatToken(userID, audience, provisionMatchID, seatKey, "", time.Hour)
			rotationToken, rotationErr := signer.SignSeatTokenWithKey(kid, rotPriv, userID, audience, provisionMatchID, seatKey, "", time.Hour)
			if primaryErr != nil || rotationErr != nil {
				results = append(results, Result{
					CheckID: "jwt.rotation_overlap",
					Status:  StatusSkipped,
					Message: "Could not mint rotation test tokens.",
				})
				return
			}

			primaryStatus, _ := r.postClaim(ctx, claimURL, primaryToken)
			rotationStatus, _ := r.postClaim(ctx, claimURL, rotationToken)
			primaryOK := primaryStatus >= 200 && primaryStatus < 300
			rotationOK := rotationStatus >= 200 && rotationStatus < 300

			switch {
			case !primaryOK:
				results = append(results, Result{
					CheckID: "jwt.rotation_overlap",
					Status:  StatusSkipped,
					Message: "Couldn't establish a baseline claim — see jwt.claim_happy_path.",
				})
			case rotationOK:
				results = append(results, Result{
					CheckID: "jwt.rotation_overlap",
					Status:  StatusPass,
					Message: "Your game accepted valid tokens signed under two different active keys.",
				})
			default:
				results = append(results, Result{
					CheckID: "jwt.rotation_overlap",
					Status:  StatusFail,
					Message: fmt.Sprintf("Your game rejected a token signed with a newly added key while the old key was still active (HTTP %d) — verify tokens by matching the JWT's kid header against every key in the JWKS response, not just the first/cached one.", rotationStatus),
				})
			}
		}()
	} else {
		results = append(results, Result{
			CheckID: "jwt.rotation_overlap",
			Status:  StatusSkipped,
			Message: "Could not set up a temporary rotation key for this check.",
		})
	}

	return results
}

func (r *Runner) checkJWKSReachable(ctx context.Context, jwksURL string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func (r *Runner) postClaim(ctx context.Context, url, token string) (int, string) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return 0, err.Error()
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return 0, err.Error()
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	return resp.StatusCode, string(payload)
}

func pickCheckMode(modes []store.GameMode, seatsByMode map[uuid.UUID][]store.GameModeSeat) (store.GameMode, []store.GameModeSeat, bool) {
	var bestMode store.GameMode
	var bestSeats []store.GameModeSeat
	bestScore := int(^uint(0) >> 1)

	for _, mode := range modes {
		if mode.Status != store.ModeStatusActive {
			continue
		}
		seats := seatsByMode[mode.ID]
		if len(seats) == 0 {
			continue
		}
		score := mode.MinPlayers
		if score < 2 {
			score = len(seats)
		}
		if score < bestScore {
			bestScore = score
			bestMode = mode
			bestSeats = seats
		}
	}
	if len(bestSeats) == 0 {
		return store.GameMode{}, nil, false
	}
	return bestMode, bestSeats, true
}

func syntheticAssignment(mode store.GameMode, seats []store.GameModeSeat, matchID string, userIDs []string) gameclient.Assignment {
	seatCount := len(userIDs)
	if seatCount > len(seats) {
		seatCount = len(seats)
	}
	out := gameclient.Assignment{
		ExternalMatchID: matchID,
		GameMode:        mode.ModeKey,
		Seats:           make([]gameclient.AssignmentSeat, 0, seatCount),
	}
	for i := 0; i < seatCount; i++ {
		seat := seats[i]
		out.Seats = append(out.Seats, gameclient.AssignmentSeat{
			SeatKey:     seat.SeatKey,
			LobbyUserID: userIDs[i],
		})
	}
	return out
}

func provisionUnreachable(msg string) []Result {
	return []Result{{
		CheckID: "provision.happy_path",
		Status:  StatusFail,
		Message: msg,
	}}
}

func friendlyProvisionError(err error) string {
	if banned, ok := err.(*gameclient.BannedPlayersError); ok {
		return fmt.Sprintf("roster banned: %v", banned.BannedLobbyUserIDs)
	}
	return err.Error()
}

func jwtSkippedAll(msg string) []Result {
	return []Result{
		skipped("jwt.jwks", msg),
		skipped("jwt.claim_happy_path", msg),
		skipped("jwt.wrong_audience", msg),
		skipped("jwt.unknown_match", msg),
		skipped("jwt.wrong_issuer", msg),
		skipped("jwt.expired", msg),
		skipped("jwt.invalid_token", msg),
		skipped("jwt.wrong_seat", msg),
		skipped("jwt.rotation_overlap", msg),
	}
}

func jwtSkippedRest(msg string) []Result {
	return []Result{
		skipped("jwt.wrong_audience", msg),
		skipped("jwt.unknown_match", msg),
		skipped("jwt.wrong_issuer", msg),
		skipped("jwt.expired", msg),
		skipped("jwt.invalid_token", msg),
		skipped("jwt.wrong_seat", msg),
		skipped("jwt.rotation_overlap", msg),
	}
}

// JWTSkippedNoProvision returns skipped JWT rows when provision did not succeed.
func JWTSkippedNoProvision() []Result {
	return jwtSkippedAll("Fix provision checks first.")
}
