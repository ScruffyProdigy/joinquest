package store

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	GameVisibilityDraft          = "draft"
	GameVisibilityPrivateTesting = "private_testing"
	GameVisibilityPendingReview  = "pending_review"
	GameVisibilityPublic         = "public"
)

type Game struct {
	ID               uuid.UUID
	Name             string
	Description      *string
	IconURL          string
	HeroURL          string
	CatalogHeroURL   *string
	TitleURL         *string
	TitleAnchor      *string
	TitleWidthPct    *float64
	ShortDescription *string
	HowToPlay        *string
	TutorialURL      *string
	Screenshots      []string
	// Tags is the retired flat tag list (JQ-162). Read-only; nothing writes it.
	Tags             []string
	Genre            *string
	Difficulty       *string
	Slug             *string
	APIBaseURL       *string
	Status           string
	Visibility       string
	OwnerUserID      *uuid.UUID
	ContactEmail     *string
	WebsiteURL       *string
	CommunityURL     *string
	AccentColor      *string
	ManifestHash     *string
	ManifestETag     *string
	ManifestSyncedAt *time.Time
	GameVersion      *string
	WebhookSecret    *string
	CreatedAt        time.Time
}

func (g *Game) IsPublicCatalog() bool {
	return g != nil && g.Visibility == GameVisibilityPublic
}

func (g *Game) AllowsRoomTables() bool {
	if g == nil {
		return false
	}
	switch g.Visibility {
	case GameVisibilityPrivateTesting, GameVisibilityPendingReview, GameVisibilityPublic:
		return true
	default:
		return false
	}
}

type GameMode struct {
	ID          uuid.UUID
	GameID      uuid.UUID
	ModeKey     string
	DisplayName string
	MinPlayers  int
	MaxPlayers  int
	SocialMode  *string
	// TypicalMinutes is the mode's declared session length, or nil when the
	// developer has not declared one. Per mode rather than per game: a game's
	// Arena and Duel modes genuinely run for different lengths.
	TypicalMinutes *int
	SeatTemplate   json.RawMessage
	// PreQueue is the mode's option-group declaration, or nil when the mode has
	// no pre-queue step. The choices inside the groups are per player and never
	// stored here — see internal/prequeue.
	PreQueue json.RawMessage
	// SkillMatchingEnabled is the mode's opt-out from skill-aware lobby
	// formation (JQ-226). On by default, and deliberately not a population
	// threshold: a thin queue is already handled by the arrival-rate gate,
	// which fires immediately and unbiased when nobody is arriving. This is for
	// the other case — a mode whose skill signal says little about whether its
	// players enjoy each other, which is true however busy the mode is.
	SkillMatchingEnabled bool
	Status               string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type GameModeSeat struct {
	ID          uuid.UUID
	ModeID      uuid.UUID
	SeatKey     string
	AffinityKey *string
	QueuePath   *string
	SortOrder   int
}

type ModeQueue struct {
	ID             uuid.UUID
	ModeID         uuid.UUID
	Name           string
	PlayersToStart int
	Status         string
	IsDefault      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type RegisterGameParams struct {
	Slug        string
	Name        string
	Description *string
	IconURL     string
	HeroURL     string
	APIBaseURL  string
}

type KickedWaiter struct {
	UserID      uuid.UUID
	GameID      uuid.UUID
	ModeQueueID uuid.UUID
	Message     string
}

type ApplyManifestResult struct {
	Game          *Game
	Changed       bool
	Kicked        []KickedWaiter
	WebhookSecret string // set only on register
}
