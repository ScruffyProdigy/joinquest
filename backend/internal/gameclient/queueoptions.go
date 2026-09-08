package gameclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/scruffyprodigy/joinquest/internal/gameurl"
	"github.com/scruffyprodigy/joinquest/internal/prequeue"
	"github.com/scruffyprodigy/joinquest/internal/runtimeenv"
)

// queueOptionsBodyLimit caps a roster read. A champion roster is a few KiB; a
// megabyte of it is a broken game server, not a big game.
const queueOptionsBodyLimit = 512 * 1024

// QueueOption is one choice a game offers this player in a pre-queue group.
// Requirement reuses the mode-eligibility tree verbatim, so a locked champion
// renders its progress through the same component a locked mode does.
type QueueOption struct {
	ID            string           `json:"id"`
	Label         string           `json:"label"`
	Description   string           `json:"description,omitempty"`
	Locked        bool             `json:"locked"`
	Requirement   *RequirementNode `json:"requirement,omitempty"`
	UnlockModeKey *string          `json:"unlockModeKey,omitempty"`
}

// QueueOptionGroup is the roster the game served for one declared group.
type QueueOptionGroup struct {
	Key     string        `json:"key"`
	Choices []QueueOption `json:"choices"`
}

// QueueOptionRoster is a whole per-player response.
type QueueOptionRoster struct {
	Groups []QueueOptionGroup `json:"groups"`
}

// queueOptionsResponse accepts both the grouped shape and the flat
// {"choices": [...]} shape JQ-148 specifies, so a game serving one roster does
// not have to learn about groups to be integrated.
type queueOptionsResponse struct {
	Groups  []QueueOptionGroup `json:"groups"`
	Choices []QueueOption      `json:"choices"`
}

// ValidationView reduces a roster to what prequeue.Validate needs: which ids
// exist, and which of them this player may not take.
func (r QueueOptionRoster) ValidationView() []prequeue.RosterGroup {
	out := make([]prequeue.RosterGroup, 0, len(r.Groups))
	for _, g := range r.Groups {
		choices := make([]prequeue.Choice, 0, len(g.Choices))
		for _, c := range g.Choices {
			choices = append(choices, prequeue.Choice{ID: c.ID, Label: c.Label, Locked: c.Locked})
		}
		out = append(out, prequeue.RosterGroup{Key: g.Key, Choices: choices})
	}
	return out
}

// Group returns the roster served for one declared group key.
func (r QueueOptionRoster) Group(key string) (QueueOptionGroup, bool) {
	for _, g := range r.Groups {
		if g.Key == key {
			return g, true
		}
	}
	return QueueOptionGroup{}, false
}

// FetchQueueOptions asks a game which pre-queue choices this player may take in
// a mode.
//
// Unlike FetchModeEligibility this does not fail open. Eligibility can assume a
// silent game means "playable"; a roster has no such default — an empty guess
// blocks a legitimate join, and a permissive guess would let a client claim an
// option the game never offered. Callers surface the error instead, and the
// mode becomes unjoinable while it stands.
//
// fallbackGroupKey names the mode's single declared group, used to adopt a flat
// {"choices": [...]} response into the grouped shape.
func (c *Client) FetchQueueOptions(ctx context.Context, apiBaseURL, lobbyUserID, modeKey, fallbackGroupKey string) (*QueueOptionRoster, error) {
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
	modeKey = strings.TrimSpace(modeKey)
	if modeKey == "" {
		return nil, fmt.Errorf("gameclient: mode key is required")
	}

	endpoint := fmt.Sprintf("%s/api/v1/players/%s/queue-options?modeKey=%s",
		base, url.PathEscape(lobbyUserID), url.QueryEscape(modeKey))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gameclient: queue options for %q: %w", modeKey, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gameclient: queue options for %q: game returned %s", modeKey, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, queueOptionsBodyLimit))
	if err != nil {
		return nil, fmt.Errorf("gameclient: queue options for %q: read body: %w", modeKey, err)
	}

	var parsed queueOptionsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("gameclient: queue options for %q: decode: %w", modeKey, err)
	}

	groups := parsed.Groups
	if groups == nil && len(parsed.Choices) > 0 {
		groups = []QueueOptionGroup{{Key: strings.TrimSpace(fallbackGroupKey), Choices: parsed.Choices}}
	}
	for gi := range groups {
		for ci := range groups[gi].Choices {
			choice := &groups[gi].Choices[ci]
			// A tree we cannot read costs the player their progress bar, not
			// their lock — dropping the lock would hand out the option.
			if choice.Requirement != nil && !choice.Requirement.valid() {
				choice.Requirement = nil
			}
		}
	}
	if groups == nil {
		groups = []QueueOptionGroup{}
	}
	return &QueueOptionRoster{Groups: groups}, nil
}
