package graph

import (
	"encoding/json"
	"strings"

	"github.com/scruffyprodigy/playhub/graph/model"
	"github.com/scruffyprodigy/playhub/internal/integrationchecks"
	"github.com/scruffyprodigy/playhub/internal/seattemplate"
	"github.com/scruffyprodigy/playhub/internal/store"
)

// ToGraphQLUser maps a store user to the GraphQL model.
func ToGraphQLUser(user *store.User) *model.User {
	if user == nil {
		return nil
	}
	var emailPtr *string
	if strings.TrimSpace(user.Email) != "" {
		email := user.Email
		emailPtr = &email
	}
	displayName := user.DisplayName
	return &model.User{
		ID:           user.ID.String(),
		Email:        emailPtr,
		DisplayName:  &displayName,
		AvatarURL:    userAvatarURL(user),
		AvatarKey:    user.AvatarKey,
		AvatarSource: toGraphQLAvatarSource(user.AvatarSource),
		CreatedAt:    user.CreatedAt,
		IsGuest:      user.IsGuest,
	}
}

// ToGraphQLPublicPlayer maps a store user to the public player profile.
func ToGraphQLPublicPlayer(user *store.User) *model.PublicPlayer {
	if user == nil {
		return nil
	}
	displayName := user.DisplayName
	return &model.PublicPlayer{
		ID:           user.ID.String(),
		DisplayName:  &displayName,
		AvatarURL:    userAvatarURL(user),
		AvatarSource: toGraphQLAvatarSource(user.AvatarSource),
	}
}

// ToGraphQLGame maps a store game to the GraphQL model.
func ToGraphQLGame(game *store.Game) *model.Game {
	if game == nil {
		return nil
	}
	result := &model.Game{
		ID:        game.ID.String(),
		Name:      game.Name,
		CreatedAt: game.CreatedAt,
		IconURL:   game.IconURL,
		HeroURL:   game.HeroURL,
		Tags:      game.Tags,
	}
	if result.IconURL == "" {
		if game.Slug != nil {
			result.IconURL = store.DefaultGameIconURL(*game.Slug)
		} else {
			result.IconURL = store.DefaultGameIconURL("")
		}
	}
	if result.HeroURL == "" {
		if game.Slug != nil {
			result.HeroURL = store.DefaultGameHeroURL(*game.Slug)
		} else {
			result.HeroURL = store.DefaultGameHeroURL("")
		}
	}
	if result.Tags == nil {
		result.Tags = []string{}
	}
	if game.Screenshots != nil {
		result.Screenshots = game.Screenshots
	} else {
		result.Screenshots = []string{}
	}
	if game.Description != nil {
		result.LongDescription = game.Description
	}
	if game.CatalogHeroURL != nil {
		result.CatalogHeroURL = game.CatalogHeroURL
	}
	if game.HowToPlay != nil {
		result.HowToPlay = game.HowToPlay
	}
	if game.TutorialURL != nil {
		result.TutorialURL = game.TutorialURL
	}
	if game.ShortDescription != nil {
		result.ShortDescription = game.ShortDescription
	}
	if game.Slug != nil {
		result.Slug = game.Slug
	}
	if game.APIBaseURL != nil {
		result.APIBaseURL = game.APIBaseURL
	}
	if game.ManifestSyncedAt != nil {
		result.ManifestSyncedAt = game.ManifestSyncedAt
	}
	if game.ManifestHash != nil {
		result.ManifestHash = game.ManifestHash
	}
	if game.GameVersion != nil {
		result.GameVersion = game.GameVersion
	}
	result.Visibility = toGraphQLGameVisibility(game.Visibility)
	if game.ContactEmail != nil {
		result.ContactEmail = game.ContactEmail
	}
	if game.WebsiteURL != nil {
		result.WebsiteURL = game.WebsiteURL
	}
	if game.CommunityURL != nil {
		result.CommunityURL = game.CommunityURL
	}
	if game.OwnerUserID != nil {
		ownerID := game.OwnerUserID.String()
		result.OwnerUserID = &ownerID
	}
	return result
}

func toGraphQLGameVisibility(visibility string) model.GameVisibility {
	switch visibility {
	case store.GameVisibilityPrivateTesting:
		return model.GameVisibilityPrivateTesting
	case store.GameVisibilityPendingReview:
		return model.GameVisibilityPendingReview
	case store.GameVisibilityPublic:
		return model.GameVisibilityPublic
	default:
		return model.GameVisibilityDraft
	}
}

func ToGraphQLIntegrationChecks(checks []store.GameIntegrationCheck) []*model.GameIntegrationCheck {
	result := make([]*model.GameIntegrationCheck, len(checks))
	for i := range checks {
		result[i] = ToGraphQLIntegrationCheck(&checks[i])
	}
	return result
}

func ToGraphQLIntegrationCheck(check *store.GameIntegrationCheck) *model.GameIntegrationCheck {
	if check == nil {
		return nil
	}
	return &model.GameIntegrationCheck{
		CheckID: check.CheckID,
		Status:  toGraphQLIntegrationCheckStatus(check.Status),
		Message: check.Message,
		Detail:  rawJSONToStringPtr(check.DetailJSON),
		RanAt:   &check.RanAt,
	}
}

func toGraphQLIntegrationCheckStatus(status string) model.IntegrationCheckStatus {
	switch status {
	case integrationchecks.StatusPass:
		return model.IntegrationCheckStatusPass
	case integrationchecks.StatusFail:
		return model.IntegrationCheckStatusFail
	default:
		return model.IntegrationCheckStatusSkipped
	}
}

func rawJSONToStringPtr(raw json.RawMessage) *string {
	if len(raw) == 0 {
		return nil
	}
	s := string(raw)
	return &s
}

func ToGraphQLDeveloperAPIKey(key *store.DeveloperAPIKey) *model.DeveloperAPIKey {
	if key == nil {
		return nil
	}
	return &model.DeveloperAPIKey{
		ID:         key.ID.String(),
		Name:       key.Name,
		KeyPrefix:  key.KeyPrefix,
		CreatedAt:  key.CreatedAt,
		LastUsedAt: key.LastUsedAt,
	}
}

func ToGraphQLGameModes(modes []store.GameMode) []*model.GameMode {
	result := make([]*model.GameMode, len(modes))
	for i := range modes {
		result[i] = ToGraphQLGameMode(&modes[i])
	}
	return result
}

func ToGraphQLGameMode(mode *store.GameMode) *model.GameMode {
	if mode == nil {
		return nil
	}
	result := &model.GameMode{
		ID:          mode.ID.String(),
		ModeKey:     mode.ModeKey,
		DisplayName: mode.DisplayName,
		MinPlayers:  mode.MinPlayers,
		MaxPlayers:  mode.MaxPlayers,
		Status:      mode.Status,
	}
	return result
}

func ToGraphQLQueuePaths(template json.RawMessage) ([]*model.GameModeQueuePath, error) {
	specs, err := seattemplate.PathSpecs(template)
	if err != nil {
		return nil, err
	}
	if len(specs) == 1 && specs[0].QueuePath == "" {
		return []*model.GameModeQueuePath{}, nil
	}
	result := make([]*model.GameModeQueuePath, len(specs))
	for i, spec := range specs {
		result[i] = &model.GameModeQueuePath{
			QueuePath:      spec.QueuePath,
			DisplayName:    spec.DisplayName,
			MinPlayers:     spec.Min,
			MaxPlayers:     spec.Max,
			PlayersToStart: spec.PlayersToStart(),
		}
	}
	return result, nil
}

func ToGraphQLGameModeSeats(seats []store.GameModeSeat) []*model.GameModeSeat {
	result := make([]*model.GameModeSeat, len(seats))
	for i := range seats {
		result[i] = &model.GameModeSeat{
			SeatKey:   seats[i].SeatKey,
			Team:      seats[i].Team,
			Role:      seats[i].Role,
			QueuePath: seats[i].QueuePath,
			SortOrder: seats[i].SortOrder,
		}
	}
	return result
}

func ToGraphQLModeQueues(queues []store.ModeQueue) []*model.ModeQueue {
	result := make([]*model.ModeQueue, len(queues))
	for i := range queues {
		result[i] = &model.ModeQueue{
			ID:             queues[i].ID.String(),
			Name:           queues[i].Name,
			PlayersToStart: queues[i].PlayersToStart,
			Status:         queues[i].Status,
		}
	}
	return result
}

// ToGraphQLGames maps a slice of store games to GraphQL models.
func ToGraphQLGames(games []store.Game) []*model.Game {
	result := make([]*model.Game, len(games))
	for i := range games {
		result[i] = ToGraphQLGame(&games[i])
	}
	return result
}

// ToGraphQLSession maps a store session to the GraphQL model.
func ToGraphQLSession(session *store.Session, game *store.Game) *model.Session {
	if session == nil {
		return nil
	}
	result := &model.Session{
		ID:        session.ID.String(),
		Status:    ToGraphQLSessionStatus(session.Status),
		CreatedAt: session.StartedAt,
	}
	if game != nil {
		result.Game = ToGraphQLGame(game)
	}
	return result
}

// ToGraphQLSessionStatus maps database session status values to GraphQL enums.
func ToGraphQLSessionStatus(dbStatus string) model.SessionStatus {
	switch dbStatus {
	case "active":
		return model.SessionStatusActive
	case "completed", "cancelled":
		return model.SessionStatusEnded
	default:
		return model.SessionStatusPending
	}
}

// ToGraphQLDigitalGood maps a store digital good to the GraphQL model.
func ToGraphQLDigitalGood(good *store.DigitalGood) *model.DigitalGood {
	if good == nil {
		return nil
	}
	code := good.ID.String()
	return &model.DigitalGood{
		ID:          good.ID.String(),
		Code:        code,
		Name:        good.Name,
		Description: good.Description,
	}
}

// ToGraphQLDigitalGoods maps a slice of store goods to GraphQL models.
func ToGraphQLDigitalGoods(goods []store.DigitalGood) []*model.DigitalGood {
	result := make([]*model.DigitalGood, len(goods))
	for i := range goods {
		result[i] = ToGraphQLDigitalGood(&goods[i])
	}
	return result
}

// ToGraphQLEntitlement maps a store inventory item to the GraphQL model.
func ToGraphQLEntitlement(item *store.InventoryItem) *model.Entitlement {
	if item == nil {
		return nil
	}
	return &model.Entitlement{
		Good:      ToGraphQLDigitalGood(&item.Good),
		Quantity:  item.Quantity,
		GrantedAt: item.AcquiredAt,
	}
}

// ToGraphQLEntitlements maps a slice of inventory items to GraphQL models.
func ToGraphQLEntitlements(items []store.InventoryItem) []*model.Entitlement {
	result := make([]*model.Entitlement, len(items))
	for i := range items {
		result[i] = ToGraphQLEntitlement(&items[i])
	}
	return result
}

// ToGraphQLUsers maps a slice of store users to GraphQL models.
func ToGraphQLUsers(users []store.User) []*model.User {
	result := make([]*model.User, len(users))
	for i := range users {
		result[i] = ToGraphQLUser(&users[i])
	}
	return result
}
