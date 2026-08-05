package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/scruffyprodigy/playhub/internal/gameclient"
)

var gameSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type RegisterMyGameParams struct {
	OwnerUserID      uuid.UUID
	Slug             string
	Name             string
	ShortDescription string
	APIBaseURL       string
	ContactEmail     string
	WebsiteURL       *string
	CommunityURL     *string
}

type RegisterMyGameResult struct {
	Game          *Game
	WebhookSecret string
	Connected     bool
	ConnectError  string
}

type GameIntegrationCheck struct {
	CheckID    string
	Status     string
	Message    *string
	DetailJSON json.RawMessage
	RanAt      time.Time
}

func ValidateGameSlug(slug string) error {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return fmt.Errorf("store: slug is required")
	}
	if len(slug) > 64 {
		return fmt.Errorf("store: slug must be at most 64 characters")
	}
	if !gameSlugPattern.MatchString(slug) {
		return fmt.Errorf("store: slug must be lowercase letters, numbers, and hyphens")
	}
	return nil
}

// RegisterMyGame saves a developer-owned game and optionally connects the API.
func (s *Store) RegisterMyGame(ctx context.Context, params RegisterMyGameParams, manifest *gameclient.Manifest, connectErr error) (*RegisterMyGameResult, error) {
	slug := strings.TrimSpace(params.Slug)
	if err := ValidateGameSlug(slug); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(params.Name)
	if name == "" {
		return nil, fmt.Errorf("store: name is required")
	}
	shortDescription := strings.TrimSpace(params.ShortDescription)
	if shortDescription == "" {
		return nil, fmt.Errorf("store: short description is required")
	}
	apiBaseURL := strings.TrimRight(strings.TrimSpace(params.APIBaseURL), "/")
	if apiBaseURL == "" {
		return nil, fmt.Errorf("store: api base URL is required")
	}
	contactEmail := strings.TrimSpace(params.ContactEmail)
	if contactEmail == "" {
		return nil, fmt.Errorf("store: contact email is required")
	}

	secret, err := generateWebhookSecret()
	if err != nil {
		return nil, err
	}

	visibility := GameVisibilityDraft
	status := ModeStatusInactive
	if manifest != nil && connectErr == nil {
		visibility = GameVisibilityPrivateTesting
		status = ModeStatusActive
	}

	iconURL := DefaultGameIconURL(slug)
	heroURL := DefaultGameHeroURL(slug)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `
		INSERT INTO games (
			name, short_description, icon_url, hero_url, slug, api_base_url,
			category, status, visibility, owner_user_id, contact_email, website_url, community_url, webhook_secret
		)
		VALUES ($1, $2, $3, $4, $5, $6, 'catalog', $7, $8, $9, $10, $11, $12, $13)
		RETURNING `+gameColumns+`
	`, name, shortDescription, iconURL, heroURL, slug, apiBaseURL, status, visibility, params.OwnerUserID, contactEmail, params.WebsiteURL, params.CommunityURL, secret)
	game, err := scanGame(row)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("store: game slug %q already exists", slug)
		}
		return nil, err
	}

	result := &RegisterMyGameResult{
		Game:          game,
		WebhookSecret: secret,
		Connected:     manifest != nil && connectErr == nil,
	}
	if connectErr != nil {
		result.ConnectError = connectErr.Error()
	}

	if manifest != nil && connectErr == nil {
		if _, _, err := applyManifestTx(ctx, tx, game, manifest); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	game, err = s.GetGameByID(ctx, game.ID)
	if err != nil {
		return nil, err
	}
	result.Game = game
	return result, nil
}

// ListMyGames returns games owned by the given user, newest first.
func (s *Store) ListMyGames(ctx context.Context, ownerUserID uuid.UUID) ([]Game, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+gameColumns+`
		FROM games
		WHERE owner_user_id = $1
		ORDER BY created_at DESC
	`, ownerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []Game
	for rows.Next() {
		g, err := scanGame(rows)
		if err != nil {
			return nil, err
		}
		games = append(games, *g)
	}
	return games, rows.Err()
}

// GetOwnedGame loads a game owned by ownerUserID.
func (s *Store) GetOwnedGame(ctx context.Context, gameID, ownerUserID uuid.UUID) (*Game, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+gameColumns+`
		FROM games
		WHERE id = $1 AND owner_user_id = $2
	`, gameID, ownerUserID)
	return scanGame(row)
}

// AssertCanCreateTableForGame enforces visibility rules for table creation.
func (s *Store) AssertCanCreateTableForGame(ctx context.Context, gameID, userID uuid.UUID) error {
	game, err := s.GetGameByID(ctx, gameID)
	if err != nil {
		return err
	}
	if !game.AllowsRoomTables() {
		return fmt.Errorf("store: game is not ready for testing yet — connect your API first")
	}
	if game.IsPublicCatalog() {
		return nil
	}
	if game.OwnerUserID != nil && *game.OwnerUserID == userID {
		return nil
	}
	return fmt.Errorf("store: only the game owner can create tables for this game")
}

// UpsertIntegrationCheck stores the latest result for a checklist row.
func (s *Store) UpsertIntegrationCheck(ctx context.Context, gameID uuid.UUID, checkID, status string, message *string, detail json.RawMessage) (*GameIntegrationCheck, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO game_integration_checks (game_id, check_id, status, message, detail_json, ran_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (game_id, check_id) DO UPDATE SET
			status = EXCLUDED.status,
			message = EXCLUDED.message,
			detail_json = EXCLUDED.detail_json,
			ran_at = NOW()
		RETURNING check_id, status, message, detail_json, ran_at
	`, gameID, checkID, status, message, nullableJSON(detail))
	return scanIntegrationCheck(row)
}

func nullableJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return raw
}

func scanIntegrationCheck(row interface{ Scan(dest ...any) error }) (*GameIntegrationCheck, error) {
	var check GameIntegrationCheck
	var message sql.NullString
	var detail []byte
	if err := row.Scan(&check.CheckID, &check.Status, &message, &detail, &check.RanAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if message.Valid {
		check.Message = &message.String
	}
	if len(detail) > 0 {
		check.DetailJSON = json.RawMessage(detail)
	}
	return &check, nil
}

// ListIntegrationChecks returns stored checklist results for a game.
func (s *Store) ListIntegrationChecks(ctx context.Context, gameID uuid.UUID) ([]GameIntegrationCheck, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT check_id, status, message, detail_json, ran_at
		FROM game_integration_checks
		WHERE game_id = $1
		ORDER BY check_id ASC
	`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checks []GameIntegrationCheck
	for rows.Next() {
		check, err := scanIntegrationCheck(rows)
		if err != nil {
			return nil, err
		}
		checks = append(checks, *check)
	}
	return checks, rows.Err()
}

type UpdateMyGameMetadataParams struct {
	Name             *string
	ShortDescription *string
	LongDescription  *string
	HowToPlay        *string
	Tags             []string
	ContactEmail     *string
	WebsiteURL       *string
	CommunityURL     *string
	ClearWebsite     bool
	ClearCommunity   bool
}

// UpdateMyGameMetadata updates owner-editable catalog and contact fields.
func (s *Store) UpdateMyGameMetadata(ctx context.Context, gameID, ownerUserID uuid.UUID, params UpdateMyGameMetadataParams) (*Game, error) {
	game, err := s.GetOwnedGame(ctx, gameID, ownerUserID)
	if err != nil {
		return nil, err
	}

	if params.Name != nil {
		name := strings.TrimSpace(*params.Name)
		if name == "" {
			return nil, fmt.Errorf("store: name is required")
		}
		game.Name = name
	}
	if params.ShortDescription != nil {
		game.ShortDescription = params.ShortDescription
	}
	if params.LongDescription != nil {
		game.Description = params.LongDescription
	}
	if params.HowToPlay != nil {
		game.HowToPlay = params.HowToPlay
	}
	if params.Tags != nil {
		game.Tags = params.Tags
	}
	tags := game.Tags
	if tags == nil {
		tags = []string{}
	}
	if params.ContactEmail != nil {
		email := strings.TrimSpace(*params.ContactEmail)
		if email == "" {
			return nil, fmt.Errorf("store: contact email is required")
		}
		game.ContactEmail = &email
	}
	if params.ClearWebsite {
		game.WebsiteURL = nil
	} else if params.WebsiteURL != nil {
		game.WebsiteURL = params.WebsiteURL
	}
	if params.ClearCommunity {
		game.CommunityURL = nil
	} else if params.CommunityURL != nil {
		game.CommunityURL = params.CommunityURL
	}

	row := s.db.QueryRowContext(ctx, `
		UPDATE games SET
			name = $3,
			short_description = $4,
			description = $5,
			how_to_play = $6,
			tags = $7,
			contact_email = $8,
			website_url = $9,
			community_url = $10,
			updated_at = NOW()
		WHERE id = $1 AND owner_user_id = $2
		RETURNING `+gameColumns+`
	`, gameID, ownerUserID, game.Name, game.ShortDescription, game.Description, game.HowToPlay, pq.Array(tags), game.ContactEmail, game.WebsiteURL, game.CommunityURL)
	return scanGame(row)
}

type ConnectOwnedGameResult struct {
	Game         *Game
	Changed      bool
	Kicked       []KickedWaiter
	Connected    bool
	ConnectError string
}

// ConnectOwnedGame updates api_base_url (optional), applies a fetched manifest, and promotes draft games.
// On connect failure the URL is not changed and existing modes are left intact.
func (s *Store) ConnectOwnedGame(ctx context.Context, gameID, ownerUserID uuid.UUID, newAPIBaseURL *string, manifest *gameclient.Manifest, connectErr error) (*ConnectOwnedGameResult, error) {
	game, err := s.GetOwnedGame(ctx, gameID, ownerUserID)
	if err != nil {
		return nil, err
	}

	if newAPIBaseURL != nil {
		trimmed := strings.TrimRight(strings.TrimSpace(*newAPIBaseURL), "/")
		if trimmed == "" {
			return nil, fmt.Errorf("store: api base URL is required")
		}
		switch game.Visibility {
		case GameVisibilityPendingReview, GameVisibilityPublic:
			return nil, fmt.Errorf("store: cannot change apiBaseUrl while the game is %s — withdraw or wait until private testing", game.Visibility)
		}
		if connectErr != nil || manifest == nil {
			return &ConnectOwnedGameResult{
				Game:         game,
				Connected:    false,
				ConnectError: connectErrorMessage(connectErr),
			}, nil
		}
		if _, err := s.db.ExecContext(ctx, `
			UPDATE games SET api_base_url = $2, updated_at = NOW() WHERE id = $1 AND owner_user_id = $3
		`, gameID, trimmed, ownerUserID); err != nil {
			return nil, err
		}
		game, err = s.GetOwnedGame(ctx, gameID, ownerUserID)
		if err != nil {
			return nil, err
		}
	}

	if connectErr != nil || manifest == nil {
		return &ConnectOwnedGameResult{
			Game:         game,
			Connected:    false,
			ConnectError: connectErrorMessage(connectErr),
		}, nil
	}

	applied, err := s.ApplyGameManifest(ctx, gameID, manifest)
	if err != nil {
		return nil, err
	}

	game, err = s.promoteOwnedGameAfterConnect(ctx, gameID, ownerUserID)
	if err != nil {
		return nil, err
	}

	return &ConnectOwnedGameResult{
		Game:      game,
		Changed:   applied.Changed,
		Kicked:    applied.Kicked,
		Connected: true,
	}, nil
}

func connectErrorMessage(err error) string {
	if err == nil {
		return "Could not reach the game API."
	}
	return err.Error()
}

func (s *Store) promoteOwnedGameAfterConnect(ctx context.Context, gameID, ownerUserID uuid.UUID) (*Game, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE games SET
			visibility = CASE WHEN visibility = $3 THEN $4 ELSE visibility END,
			status = $5,
			updated_at = NOW()
		WHERE id = $1 AND owner_user_id = $2
		RETURNING `+gameColumns+`
	`, gameID, ownerUserID, GameVisibilityDraft, GameVisibilityPrivateTesting, ModeStatusActive)
	return scanGame(row)
}

// RotateMyGameWebhookSecret replaces the webhook secret for an owned game.
func (s *Store) RotateMyGameWebhookSecret(ctx context.Context, gameID, ownerUserID uuid.UUID) (string, *Game, error) {
	if _, err := s.GetOwnedGame(ctx, gameID, ownerUserID); err != nil {
		return "", nil, err
	}
	secret, err := generateWebhookSecret()
	if err != nil {
		return "", nil, err
	}
	row := s.db.QueryRowContext(ctx, `
		UPDATE games SET webhook_secret = $3, updated_at = NOW()
		WHERE id = $1 AND owner_user_id = $2
		RETURNING `+gameColumns+`
	`, gameID, ownerUserID, secret)
	game, err := scanGame(row)
	if err != nil {
		return "", nil, err
	}
	return secret, game, nil
}

// RequestPublicRelease moves a game to pending_review when gates pass.
func (s *Store) RequestPublicRelease(ctx context.Context, gameID, ownerUserID uuid.UUID) (*Game, error) {
	game, err := s.GetOwnedGame(ctx, gameID, ownerUserID)
	if err != nil {
		return nil, err
	}
	if game.Visibility != GameVisibilityPrivateTesting {
		return nil, fmt.Errorf("store: only private testing games can request public release")
	}
	if err := ValidatePublicReleaseMetadata(game); err != nil {
		return nil, err
	}
	checks, err := s.ListIntegrationChecks(ctx, gameID)
	if err != nil {
		return nil, err
	}
	if err := ValidatePublicReleaseChecks(checks); err != nil {
		return nil, err
	}

	row := s.db.QueryRowContext(ctx, `
		UPDATE games SET visibility = $2
		WHERE id = $1 AND owner_user_id = $3
		RETURNING `+gameColumns+`
	`, gameID, GameVisibilityPendingReview, ownerUserID)
	return scanGame(row)
}

// ReviewGameRelease approves or rejects a pending game (admin).
func (s *Store) ReviewGameRelease(ctx context.Context, gameID uuid.UUID, approve bool) (*Game, error) {
	game, err := s.GetGameByID(ctx, gameID)
	if err != nil {
		return nil, err
	}
	if game.Visibility != GameVisibilityPendingReview {
		return nil, fmt.Errorf("store: game is not pending review")
	}
	next := GameVisibilityPrivateTesting
	if approve {
		next = GameVisibilityPublic
	}
	row := s.db.QueryRowContext(ctx, `
		UPDATE games SET visibility = $2
		WHERE id = $1
		RETURNING `+gameColumns+`
	`, gameID, next)
	return scanGame(row)
}

// ListPendingGameReviews returns games awaiting admin approval.
func (s *Store) ListPendingGameReviews(ctx context.Context) ([]Game, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+gameColumns+`
		FROM games
		WHERE visibility = $1
		ORDER BY created_at ASC
	`, GameVisibilityPendingReview)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []Game
	for rows.Next() {
		g, err := scanGame(rows)
		if err != nil {
			return nil, err
		}
		games = append(games, *g)
	}
	return games, rows.Err()
}

// ValidatePublicReleaseMetadata ensures catalog copy is ready for review.
func ValidatePublicReleaseMetadata(game *Game) error {
	if game == nil {
		return fmt.Errorf("store: game is required")
	}
	if strings.TrimSpace(game.Name) == "" {
		return fmt.Errorf("store: game name is required")
	}
	if game.ShortDescription == nil || strings.TrimSpace(*game.ShortDescription) == "" {
		return fmt.Errorf("store: short description is required before public release")
	}
	if game.Description == nil || strings.TrimSpace(*game.Description) == "" {
		return fmt.Errorf("store: long description is required before public release")
	}
	if len(game.Tags) == 0 {
		return fmt.Errorf("store: at least one catalog tag is required before public release")
	}
	return nil
}

var requiredPassChecks = []string{
	"manifest.reach_api",
	"manifest.status",
	"manifest.launch_urls_on_provision",
	"manifest.game_modes",
	"manifest.sync_freshness",
	"provision.happy_path",
	"provision.idempotent_repush",
	"provision.auth",
	"provision.missing_auth",
	"provision.launch_urls",
	"provision.launch_url_no_jwt",
	"jwt.jwks",
	"jwt.claim_happy_path",
	"jwt.wrong_audience",
	"jwt.unknown_match",
	"jwt.wrong_issuer",
	"jwt.expired",
	"jwt.invalid_token",
	"jwt.wrong_seat",
}

const integrationCheckStatusPass = "pass"

// ValidatePublicReleaseChecks ensures required integration checks passed.
func ValidatePublicReleaseChecks(checks []GameIntegrationCheck) error {
	byID := make(map[string]string, len(checks))
	for _, c := range checks {
		byID[c.CheckID] = c.Status
	}
	for _, id := range requiredPassChecks {
		if byID[id] != integrationCheckStatusPass {
			return fmt.Errorf("store: required check %q must pass before public release", id)
		}
	}
	return nil
}
