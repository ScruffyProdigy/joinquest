package integrationchecks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/scruffyprodigy/joinquest/internal/gameclient"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

const (
	StatusPass    = "pass"
	StatusFail    = "fail"
	StatusSkipped = "skipped"
)

type Result struct {
	CheckID string
	Status  string
	Message string
	Detail  json.RawMessage
}

type Runner struct {
	ManifestFetcher *gameclient.ManifestFetcher
}

func NewRunner(fetcher *gameclient.ManifestFetcher) *Runner {
	if fetcher == nil {
		fetcher = gameclient.NewManifestFetcher()
	}
	return &Runner{ManifestFetcher: fetcher}
}

// RunManifestChecks exercises reachability, status, and game-modes endpoints.
func (r *Runner) RunManifestChecks(ctx context.Context, game *store.Game) []Result {
	if game == nil || game.APIBaseURL == nil {
		return []Result{{
			CheckID: "manifest.reach_api",
			Status:  StatusFail,
			Message: "No API base URL is configured for this game.",
		}}
	}

	apiBase := strings.TrimSpace(*game.APIBaseURL)
	results := make([]Result, 0, 4)

	priorETag := ""
	if game.ManifestETag != nil {
		priorETag = *game.ManifestETag
	}

	manifest, err := r.ManifestFetcher.Fetch(ctx, apiBase, priorETag)
	if err != nil {
		results = append(results, Result{
			CheckID: "manifest.reach_api",
			Status:  StatusFail,
			Message: friendlyReachError(apiBase, err),
		})
		results = append(results,
			skipped("manifest.status", "Fix API reachability first."),
			skipped("manifest.launch_urls_on_provision", "Fix API reachability first."),
			skipped("manifest.game_modes", "Fix API reachability first."),
			skipped("manifest.sync_freshness", "Fix API reachability first."),
		)
		return results
	}

	results = append(results, Result{
		CheckID: "manifest.reach_api",
		Status:  StatusPass,
		Message: "We reached your server and healthz returned ok.",
	})

	statusDetail, _ := json.Marshal(manifest.Status)
	results = append(results, Result{
		CheckID: "manifest.status",
		Status:  StatusPass,
		Message: fmt.Sprintf("Status endpoint returned game %q (version %s).", manifest.Status.Game, manifest.Status.Version),
		Detail:  statusDetail,
	})

	if manifest.Status.LaunchUrlsOnProvision {
		results = append(results, Result{
			CheckID: "manifest.launch_urls_on_provision",
			Status:  StatusPass,
			Message: "Status reports launchUrlsOnProvision: true.",
		})
	} else {
		results = append(results, Result{
			CheckID: "manifest.launch_urls_on_provision",
			Status:  StatusFail,
			Message: "Status must include launchUrlsOnProvision: true — JoinQuest attaches seat JWTs to game-minted launch URLs.",
		})
	}

	if len(manifest.Modes) == 0 {
		results = append(results, Result{
			CheckID: "manifest.game_modes",
			Status:  StatusFail,
			Message: "Your game returned no modes from /api/v1/game-modes.",
		})
	} else {
		results = append(results, Result{
			CheckID: "manifest.game_modes",
			Status:  StatusPass,
			Message: fmt.Sprintf("Found %d playable mode(s) with valid seat templates.%s",
				len(manifest.Modes), typicalMinutesGuidance(manifest.Modes)),
		})
	}

	if game.ManifestSyncedAt != nil {
		results = append(results, Result{
			CheckID: "manifest.sync_freshness",
			Status:  StatusPass,
			Message: fmt.Sprintf("Manifest last synced %s.", game.ManifestSyncedAt.UTC().Format(time.RFC3339)),
		})
	} else {
		results = append(results, Result{
			CheckID: "manifest.sync_freshness",
			Status:  StatusFail,
			Message: "Your manifest has not been synced to JoinQuest yet — call syncMyGameManifest (or connectMyGame) after your API is reachable.",
		})
	}

	return results
}

// StubChecks is deprecated; provision and JWT checks run via RunProvisionChecks and RunJWTChecks.
func StubChecks() []Result {
	return nil
}

// typicalMinutesGuidance nudges a developer whose modes carry no declared
// session length (JQ-161).
//
// It rides on the game_modes check rather than being a check of its own, and it
// never fails one: the field is optional and a mode without it is correct, just
// missing a catalog pill. A check that went red here would be telling the
// developer they had broken something they had not.
func typicalMinutesGuidance(modes []gameclient.ModeManifest) string {
	missing := 0
	for _, mode := range modes {
		if mode.TypicalMinutes == nil {
			missing++
		}
	}
	switch {
	case missing == 0:
		return " Every mode declares a typical duration."
	case missing == len(modes):
		return " None declare typicalMinutes, so your catalog card shows no session length — add it to each mode to show \"12 min\" beside the player count."
	default:
		return fmt.Sprintf(" %d of %d modes omit typicalMinutes, so those show no session length on the catalog card.", missing, len(modes))
	}
}

func friendlyReachError(apiBase string, err error) string {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "localhost"):
		return fmt.Sprintf("We couldn't reach %s — localhost URLs only work on your machine, not on JoinQuest servers.", apiBase)
	case strings.Contains(msg, "connection refused"), strings.Contains(msg, "no such host"), strings.Contains(msg, "timeout"):
		return fmt.Sprintf("We couldn't reach %s — is it up and publicly reachable over HTTPS?", apiBase)
	default:
		return fmt.Sprintf("We couldn't reach %s — %s", apiBase, err.Error())
	}
}

func skipped(checkID, message string) Result {
	return Result{CheckID: checkID, Status: StatusSkipped, Message: message}
}
