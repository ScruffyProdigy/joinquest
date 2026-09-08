package gameclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scruffyprodigy/joinquest/internal/developer"
	"github.com/scruffyprodigy/joinquest/internal/prequeue"
	"github.com/scruffyprodigy/joinquest/internal/seattemplate"
)

// ModeManifest is one playable mode from GET /api/v1/game-modes.
type ModeManifest struct {
	Key         string `json:"key"`
	DisplayName string `json:"displayName"`
	// SocialMode is how play is structured in this mode — one id from
	// developer.SocialModeTaxonomy. Optional: a manifest written before JQ-162
	// omits it and the mode simply carries no social shape.
	SocialMode   string          `json:"socialMode"`
	SeatTemplate json.RawMessage `json:"seatTemplate"`
	Seats        json.RawMessage `json:"seats"`
	// PreQueue declares the mode's option groups. The roster inside them is
	// per player and comes from the game at request time, never from here.
	PreQueue json.RawMessage `json:"preQueue"`
}

// StatusResponse is returned by GET /api/v1/status.
type StatusResponse struct {
	Game                  string `json:"game"`
	Version               string `json:"version"`
	AppEnv                string `json:"appEnv"`
	Standalone            bool   `json:"standalone"`
	LaunchUrlsOnProvision bool   `json:"launchUrlsOnProvision"`
}

// Manifest is the cached game-modes payload plus sync metadata.
type Manifest struct {
	Modes      []ModeManifest
	Status     StatusResponse
	ETag       string
	RawJSON    []byte
	SHA256Hash string
}

// ManifestFetcher pulls health, status, and game-modes from a third-party game API.
type ManifestFetcher struct {
	httpClient *http.Client
}

func NewManifestFetcher() *ManifestFetcher {
	return &ManifestFetcher{
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// Fetch loads /healthz, /api/v1/status, and /api/v1/game-modes from apiBaseURL.
func (f *ManifestFetcher) Fetch(ctx context.Context, apiBaseURL string, priorETag string) (*Manifest, error) {
	base := strings.TrimRight(strings.TrimSpace(apiBaseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("gameclient: api base URL is required")
	}

	if err := f.checkHealth(ctx, base); err != nil {
		return nil, err
	}

	status, err := f.fetchStatus(ctx, base)
	if err != nil {
		return nil, err
	}

	modes, etag, raw, err := f.fetchGameModes(ctx, base, priorETag)
	if err != nil {
		return nil, err
	}

	sum := sha256.Sum256(raw)
	return &Manifest{
		Modes:      modes,
		Status:     status,
		ETag:       etag,
		RawJSON:    raw,
		SHA256Hash: hex.EncodeToString(sum[:]),
	}, nil
}

func (f *ManifestFetcher) checkHealth(ctx context.Context, base string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/healthz", nil)
	if err != nil {
		return err
	}
	resp, err := f.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gameclient: healthz: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gameclient: healthz: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if text := strings.TrimSpace(string(body)); text != "" && text != "ok" {
		return fmt.Errorf("gameclient: healthz: unexpected body %q", text)
	}
	return nil
}

func (f *ManifestFetcher) fetchStatus(ctx context.Context, base string) (StatusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/v1/status", nil)
	if err != nil {
		return StatusResponse{}, err
	}
	resp, err := f.httpClient.Do(req)
	if err != nil {
		return StatusResponse{}, fmt.Errorf("gameclient: status: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode != http.StatusOK {
		return StatusResponse{}, fmt.Errorf("gameclient: status: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var status StatusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		return StatusResponse{}, fmt.Errorf("gameclient: status: decode: %w", err)
	}
	return status, nil
}

func (f *ManifestFetcher) fetchGameModes(ctx context.Context, base, priorETag string) ([]ModeManifest, string, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/v1/game-modes", nil)
	if err != nil {
		return nil, "", nil, err
	}
	if priorETag != "" {
		req.Header.Set("If-None-Match", priorETag)
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, "", nil, fmt.Errorf("gameclient: game-modes: %w", err)
	}
	defer resp.Body.Close()

	etag := strings.TrimSpace(resp.Header.Get("ETag"))
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))

	if resp.StatusCode == http.StatusNotModified {
		return nil, priorETag, nil, ErrManifestNotModified
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", nil, fmt.Errorf("gameclient: game-modes: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	modes, err := parseGameModes(body)
	if err != nil {
		return nil, "", nil, err
	}
	if err := validateModes(modes); err != nil {
		return nil, "", nil, err
	}
	return modes, etag, body, nil
}

func parseGameModes(body []byte) ([]ModeManifest, error) {
	var envelope struct {
		Modes []ModeManifest `json:"modes"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && len(envelope.Modes) > 0 {
		return envelope.Modes, nil
	}
	var modes []ModeManifest
	if err := json.Unmarshal(body, &modes); err != nil {
		return nil, fmt.Errorf("gameclient: game-modes: decode: %w", err)
	}
	return modes, nil
}

func validateModes(modes []ModeManifest) error {
	if len(modes) == 0 {
		return fmt.Errorf("gameclient: game-modes: at least one mode is required")
	}
	for _, mode := range modes {
		key := strings.TrimSpace(mode.Key)
		if key == "" {
			return fmt.Errorf("gameclient: game-modes: mode key is required")
		}
		if len(mode.Seats) > 0 && string(mode.Seats) != "null" {
			return fmt.Errorf("gameclient: game-modes: mode %q uses flat seats[]; use seatTemplate", key)
		}
		if len(mode.SeatTemplate) == 0 {
			return fmt.Errorf("gameclient: game-modes: mode %q must define seatTemplate", key)
		}
		if err := developer.ValidateSocialMode(strings.TrimSpace(mode.SocialMode)); err != nil {
			return fmt.Errorf("gameclient: game-modes: mode %q: %w", key, err)
		}
		leaves, err := seattemplate.Expand(mode.SeatTemplate)
		if err != nil {
			return fmt.Errorf("gameclient: game-modes: mode %q: %w", key, err)
		}
		seen := make(map[string]struct{}, len(leaves))
		for _, leaf := range leaves {
			if _, ok := seen[leaf.SeatKey]; ok {
				return fmt.Errorf("gameclient: game-modes: mode %q duplicate seat %q", key, leaf.SeatKey)
			}
			seen[leaf.SeatKey] = struct{}{}
		}
		// A declaration the lobby cannot act on fails the whole sync: syncing
		// the mode anyway would list a picker that can never render.
		if _, err := prequeue.Parse(mode.PreQueue); err != nil {
			return fmt.Errorf("gameclient: game-modes: mode %q: %w", key, err)
		}
	}
	return nil
}

// ExpandModeSeats expands a mode's seatTemplate into catalog leaves.
func ExpandModeSeats(mode ModeManifest) ([]seattemplate.Leaf, error) {
	return seattemplate.Expand(mode.SeatTemplate)
}

// ErrManifestNotModified means the remote manifest etag is unchanged.
var ErrManifestNotModified = fmt.Errorf("gameclient: manifest not modified")
