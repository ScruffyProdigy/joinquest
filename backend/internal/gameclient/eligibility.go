package gameclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/scruffyprodigy/joinquest/internal/gameurl"
	"github.com/scruffyprodigy/joinquest/internal/runtimeenv"
)

// RequirementNode is one leaf or group node in a mode's eligibility requirement tree.
type RequirementNode struct {
	Kind     string            `json:"kind"`
	Label    string            `json:"label"`
	Current  int               `json:"current,omitempty"`
	Target   int               `json:"target,omitempty"`
	Operator string            `json:"operator,omitempty"`
	Children []RequirementNode `json:"children,omitempty"`
}

// valid reports whether this node and its descendants use recognized kind/operator values.
func (n *RequirementNode) valid() bool {
	switch n.Kind {
	case "leaf":
		return true
	case "group":
		op := strings.ToLower(n.Operator)
		if op != "all" && op != "any" {
			return false
		}
		for i := range n.Children {
			if !n.Children[i].valid() {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// ModeEligibility is one game mode's per-player accessibility, as reported by a game server.
type ModeEligibility struct {
	Accessible    bool             `json:"accessible"`
	Reason        string           `json:"reason"`
	Requirement   *RequirementNode `json:"requirement"`
	UnlockModeKey *string          `json:"unlockModeKey"`
}

type modeEligibilityBatch struct {
	Modes map[string]ModeEligibility `json:"modes"`
}

// FetchModeEligibility calls a game server's optional per-player mode-eligibility endpoint.
// A missing endpoint (404) or unreachable server returns an empty map, not an error — callers
// treat a missing mode key as accessible: true (fail-open). Modes with a malformed requirement
// tree (unrecognized kind/operator) are dropped from the result for the same reason.
func (c *Client) FetchModeEligibility(ctx context.Context, apiBaseURL, lobbyUserID string) (map[string]ModeEligibility, error) {
	base := strings.TrimRight(strings.TrimSpace(apiBaseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("gameclient: api base URL is required")
	}
	if err := gameurl.ValidateOutboundURL(ctx, base, runtimeenv.IsProductionEnv()); err != nil {
		return nil, fmt.Errorf("gameclient: %w", err)
	}
	lobbyUserID = strings.TrimSpace(lobbyUserID)
	if lobbyUserID == "" {
		return nil, fmt.Errorf("gameclient: lobby user id is required")
	}

	url := fmt.Sprintf("%s/api/v1/players/%s/mode-eligibility", base, lobbyUserID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return map[string]ModeEligibility{}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return map[string]ModeEligibility{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return map[string]ModeEligibility{}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return map[string]ModeEligibility{}, nil
	}

	var batch modeEligibilityBatch
	if err := json.Unmarshal(body, &batch); err != nil {
		return map[string]ModeEligibility{}, nil
	}

	result := make(map[string]ModeEligibility, len(batch.Modes))
	for modeKey, elig := range batch.Modes {
		if elig.Requirement != nil && !elig.Requirement.valid() {
			continue
		}
		result[modeKey] = elig
	}
	return result, nil
}
