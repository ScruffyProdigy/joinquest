package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

func scanGame(row interface{ Scan(dest ...any) error }) (*Game, error) {
	var g Game
	var description, iconURL, heroURL, catalogHeroURL, shortDescription, howToPlay, tutorialURL, slug, apiBaseURL sql.NullString
	var titleURL, titleAnchor sql.NullString
	var titleWidthPct sql.NullFloat64
	var visibility, contactEmail, websiteURL, communityURL, accentColor sql.NullString
	var ownerUserID uuid.NullUUID
	var manifestHash, manifestETag, gameVersion, webhookSecret sql.NullString
	var manifestSyncedAt sql.NullTime
	var tags, screenshots pq.StringArray
	if err := row.Scan(
		&g.ID, &g.Name, &description, &iconURL, &heroURL, &catalogHeroURL, &titleURL, &titleAnchor, &titleWidthPct,
		&shortDescription, &howToPlay, &tutorialURL, &screenshots, &tags, &slug, &apiBaseURL,
		&g.Status,
		&visibility, &ownerUserID, &contactEmail, &websiteURL, &communityURL, &accentColor,
		&manifestHash, &manifestETag, &manifestSyncedAt, &gameVersion, &webhookSecret,
		&g.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if description.Valid {
		g.Description = &description.String
	}
	if iconURL.Valid {
		g.IconURL = iconURL.String
	}
	if heroURL.Valid {
		g.HeroURL = heroURL.String
	}
	if catalogHeroURL.Valid {
		g.CatalogHeroURL = &catalogHeroURL.String
	}
	if titleURL.Valid {
		g.TitleURL = &titleURL.String
	}
	if titleAnchor.Valid {
		g.TitleAnchor = &titleAnchor.String
	}
	if titleWidthPct.Valid {
		g.TitleWidthPct = &titleWidthPct.Float64
	}
	if shortDescription.Valid {
		g.ShortDescription = &shortDescription.String
	}
	if howToPlay.Valid {
		g.HowToPlay = &howToPlay.String
	}
	if tutorialURL.Valid {
		g.TutorialURL = &tutorialURL.String
	}
	if len(screenshots) > 0 {
		g.Screenshots = []string(screenshots)
	}
	if len(tags) > 0 {
		g.Tags = []string(tags)
	}
	if slug.Valid {
		g.Slug = &slug.String
	}
	if apiBaseURL.Valid {
		g.APIBaseURL = &apiBaseURL.String
	}
	if visibility.Valid {
		g.Visibility = visibility.String
	}
	if ownerUserID.Valid {
		g.OwnerUserID = &ownerUserID.UUID
	}
	if contactEmail.Valid {
		g.ContactEmail = &contactEmail.String
	}
	if websiteURL.Valid {
		g.WebsiteURL = &websiteURL.String
	}
	if communityURL.Valid {
		g.CommunityURL = &communityURL.String
	}
	if accentColor.Valid {
		g.AccentColor = &accentColor.String
	}
	if manifestHash.Valid {
		g.ManifestHash = &manifestHash.String
	}
	if manifestETag.Valid {
		g.ManifestETag = &manifestETag.String
	}
	if manifestSyncedAt.Valid {
		g.ManifestSyncedAt = &manifestSyncedAt.Time
	}
	if gameVersion.Valid {
		g.GameVersion = &gameVersion.String
	}
	if webhookSecret.Valid {
		g.WebhookSecret = &webhookSecret.String
	}
	return &g, nil
}

// DefaultGameIconURL returns the catalog icon path for a game slug.
func DefaultGameIconURL(slug string) string {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return "/games/default.svg"
	}
	if icon, ok := slugIconOverrides[slug]; ok {
		return icon
	}
	return "/games/" + slug + ".svg"
}

var slugIconOverrides = map[string]string{
	"rock-paper-scissors-lizard-robot": "/games/rpslr-icon.png",
	"rock-paper-scissors-lizard-spock": "/games/rpslr-icon.png", // legacy slug
	"word-hunt":                        "/games/word-hunt-icon.png",
}

// DefaultGameHeroURL returns the catalog hero banner path for a game slug.
func DefaultGameHeroURL(slug string) string {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return "/games/default-hero.svg"
	}
	if hero, ok := slugHeroOverrides[slug]; ok {
		return hero
	}
	return "/games/" + slug + "-hero.svg"
}

var slugHeroOverrides = map[string]string{
	"rock-paper-scissors-lizard-robot": "/games/rpslr-hero.jpg",
	"rock-paper-scissors-lizard-spock": "/games/rpslr-hero.jpg", // legacy slug
	"word-hunt":                        "/games/word-hunt-hero.jpg",
}

const gameColumns = `id, name, description, icon_url, hero_url, catalog_hero_url, title_url, title_anchor, title_width_pct,
	short_description, how_to_play, tutorial_url, screenshots, tags, slug, api_base_url, status,
	visibility, owner_user_id, contact_email, website_url, community_url, accent_color,
	manifest_hash, manifest_etag, manifest_synced_at, game_version, webhook_secret, created_at`

func (s *Store) ListGames(ctx context.Context, limit, offset int) ([]Game, error) {
	return s.ListCatalogGames(ctx, limit, offset)
}

// ListCatalogGames returns active games that have at least one active mode queue.
func (s *Store) ListCatalogGames(ctx context.Context, limit, offset int) ([]Game, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT `+gameColumns+`
		FROM games g
		WHERE g.status = 'active'
		  AND g.visibility = 'public'
		  AND EXISTS (
		    SELECT 1
		    FROM game_modes gm
		    INNER JOIN mode_queues mq ON mq.mode_id = gm.id
		    WHERE gm.game_id = g.id
		      AND gm.status = 'active'
		      AND mq.status = 'active'
		  )
		ORDER BY g.created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
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

func (s *Store) GetGameByID(ctx context.Context, id uuid.UUID) (*Game, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+gameColumns+`
		FROM games
		WHERE id = $1
	`, id)
	return scanGame(row)
}

func (s *Store) InsertTestGame(ctx context.Context, name string) (*Game, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("store: game name is required")
	}

	row := s.db.QueryRowContext(ctx, `
		INSERT INTO games (name, icon_url, hero_url)
		VALUES ($1, $2, $3)
		RETURNING `+gameColumns+`
	`, name, DefaultGameIconURL(""), DefaultGameHeroURL(""))
	return scanGame(row)
}
