package gameclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scruffyprodigy/playhub/internal/gameurl"
	"github.com/scruffyprodigy/playhub/internal/runtimeenv"
)

// ProvisionPlayer is presentation data games can show before any GraphQL lookup.
type ProvisionPlayer struct {
	DisplayName string `json:"displayName,omitempty"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
}

// AssignmentSeat is one seat in a Lobby-pushed roster.
type AssignmentSeat struct {
	SeatKey     string           `json:"seatKey"`
	LobbyUserID string           `json:"lobbyUserId"`
	Team        string           `json:"team,omitempty"`
	Role        string           `json:"role,omitempty"`
	Player      *ProvisionPlayer `json:"player,omitempty"`
}

// LobbyInfo tells the game how to reach Lobby after and during a match.
// serviceToken is the per-lobby credential for GraphQL (and future callbacks); games must
// store it per match — do not rely on a global env var in multi-lobby deployments.
type LobbyInfo struct {
	ReturnURL    string `json:"returnUrl"`
	GraphqlURL   string `json:"graphqlUrl"`
	ServiceToken string `json:"serviceToken,omitempty"`
}

// Assignment is the match roster pushed to the game server.
type Assignment struct {
	ExternalMatchID string           `json:"externalMatchId"`
	GameMode        string           `json:"gameMode"`
	Seats           []AssignmentSeat `json:"seats"`
}

// BannedPlayersError is returned when the game rejects the roster (HTTP 403).
type BannedPlayersError struct {
	BannedLobbyUserIDs []string
	Message            string
}

func (e *BannedPlayersError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("roster rejected: banned %v", e.BannedLobbyUserIDs)
}

// Client pushes match assignments to a third-party game API.
type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// ProvisionMatch creates or confirms a match on the game server (idempotent on externalMatchId).
func (c *Client) ProvisionMatch(ctx context.Context, req ProvisionRequest) (ProvisionResult, error) {
	base := strings.TrimRight(strings.TrimSpace(req.APIBaseURL), "/")
	if base == "" {
		return ProvisionResult{}, errors.New("gameclient: api base URL is required")
	}
	if err := gameurl.ValidateOutboundURL(ctx, base, runtimeenv.IsProductionEnv()); err != nil {
		return ProvisionResult{}, fmt.Errorf("gameclient: %w", err)
	}
	provisionURL := base + "/api/v1/matches"
	lobbyID := strings.TrimSpace(req.LobbyID)
	if lobbyID == "" {
		return ProvisionResult{}, errors.New("gameclient: lobby id is required")
	}
	if strings.TrimSpace(req.Lobby.ReturnURL) == "" {
		return ProvisionResult{}, errors.New("gameclient: lobby return URL is required")
	}
	if strings.TrimSpace(req.Lobby.GraphqlURL) == "" {
		return ProvisionResult{}, errors.New("gameclient: lobby graphql URL is required")
	}

	body, err := json.Marshal(map[string]any{
		"lobbyId":    lobbyID,
		"lobby":      req.Lobby,
		"assignment": req.Assignment,
	})
	if err != nil {
		return ProvisionResult{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, provisionURL, bytes.NewReader(body))
	if err != nil {
		return ProvisionResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if header := serviceAuthHeader(req.ServiceToken); header != "" {
		httpReq.Header.Set("Authorization", header)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return ProvisionResult{}, fmt.Errorf("gameclient: provision match: %w", err)
	}
	defer resp.Body.Close()

	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode == http.StatusForbidden {
		var errBody struct {
			Error              string   `json:"error"`
			BannedLobbyUserIDs []string `json:"bannedLobbyUserIds"`
		}
		_ = json.Unmarshal(payload, &errBody)
		return ProvisionResult{}, &BannedPlayersError{
			BannedLobbyUserIDs: errBody.BannedLobbyUserIDs,
			Message:            errBody.Error,
		}
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return ProvisionResult{}, fmt.Errorf("gameclient: provision match: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	return parseProvisionResponse(payload), nil
}

func parseProvisionResponse(payload []byte) ProvisionResult {
	var body struct {
		LaunchURLs        map[string]string `json:"launchUrls"`
		LaunchURLTemplate string            `json:"launchUrlTemplate"`
	}
	_ = json.Unmarshal(payload, &body)
	out := ProvisionResult{
		LaunchURLs:        make(map[string]string, len(body.LaunchURLs)),
		LaunchURLTemplate: strings.TrimSpace(body.LaunchURLTemplate),
	}
	for id, raw := range body.LaunchURLs {
		id = strings.TrimSpace(id)
		raw = strings.TrimSpace(raw)
		if id != "" && raw != "" {
			out.LaunchURLs[id] = raw
		}
	}
	return out
}

func serviceAuthHeader(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	if len(token) >= 7 && strings.EqualFold(token[:7], "bearer ") {
		return token
	}
	return "Bearer " + token
}
