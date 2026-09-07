package graph

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/graph/model"
	"github.com/scruffyprodigy/joinquest/internal/gameclient"
	"github.com/scruffyprodigy/joinquest/internal/prequeue"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

func (r *Resolver) queueOptionsCache() *gameclient.QueueOptionsCache {
	if r.QueueOptionsCache != nil {
		return r.QueueOptionsCache
	}
	return gameclient.NewQueueOptionsCache(gameclient.NewClient(), 5*time.Second)
}

// declaredGroups reads a mode's option-group declaration.
//
// A declaration the lobby cannot parse is treated as "no groups" rather than an
// error: the manifest sync already rejects bad declarations, so reaching here
// means a row predates that check, and a mode nobody can join is worse than a
// mode with no picker.
func declaredGroups(mode *store.GameMode) []prequeue.Group {
	if mode == nil {
		return nil
	}
	decl, err := prequeue.Parse(mode.PreQueue)
	if err != nil || decl == nil {
		return nil
	}
	return decl.Groups
}

// soleGroupKey names the mode's only group, so a game may serve the flat
// {"choices": […]} shape without knowing about groups.
func soleGroupKey(groups []prequeue.Group) string {
	if len(groups) != 1 {
		return ""
	}
	return groups[0].Key
}

// playerRoster fetches this player's choices for a mode.
func (r *Resolver) playerRoster(
	ctx context.Context,
	game *store.Game,
	mode *store.GameMode,
	groups []prequeue.Group,
	lobbyUserID string,
) (*gameclient.QueueOptionRoster, error) {
	if game == nil || mode == nil {
		return nil, fmt.Errorf("game mode is required")
	}
	apiBaseURL := ""
	if game.APIBaseURL != nil {
		apiBaseURL = strings.TrimSpace(*game.APIBaseURL)
	}
	if apiBaseURL == "" {
		return nil, fmt.Errorf("this game has not published an API base URL, so its options cannot be loaded")
	}
	return r.queueOptionsCache().Get(ctx, game.ID.String(), apiBaseURL, lobbyUserID, mode.ModeKey, soleGroupKey(groups))
}

// resolveSelections turns the picks on a mutation into what gets stored.
//
// It is the single point where a client's claim about what it chose meets what
// the game actually offered: unknown ids, locked ids and counts outside the
// declared bounds are all rejected here, and the stored labels come from the
// roster rather than from the request.
func (r *Resolver) resolveSelections(
	ctx context.Context,
	game *store.Game,
	mode *store.GameMode,
	lobbyUserID string,
	input []*model.QueueOptionSelectionInput,
) ([]prequeue.Selection, error) {
	groups := declaredGroups(mode)
	selections := selectionsFromInput(input)

	if len(groups) == 0 {
		if len(selections) > 0 {
			return nil, fmt.Errorf("this mode does not use pre-queue options")
		}
		return nil, nil
	}

	roster, err := r.playerRoster(ctx, game, mode, groups, lobbyUserID)
	if err != nil {
		return nil, fmt.Errorf("options for this mode are unavailable right now: %w", err)
	}
	view := roster.ValidationView()
	if err := prequeue.Validate(groups, view, selections); err != nil {
		return nil, err
	}
	return prequeue.Resolve(groups, view, selections), nil
}

func selectionsFromInput(input []*model.QueueOptionSelectionInput) []prequeue.Selection {
	out := make([]prequeue.Selection, 0, len(input))
	for _, sel := range input {
		if sel == nil {
			continue
		}
		out = append(out, prequeue.Selection{GroupKey: sel.GroupKey, OptionIDs: sel.OptionIds})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// modeForQueue loads the game and catalog mode behind a mode queue.
func (r *Resolver) modeForQueue(ctx context.Context, modeQueueID uuid.UUID) (*store.Game, *store.GameMode, error) {
	queue, err := r.Store.GetModeQueueByID(ctx, modeQueueID)
	if err != nil {
		return nil, nil, err
	}
	mode, err := r.Store.GetGameModeByID(ctx, queue.ModeID)
	if err != nil {
		return nil, nil, err
	}
	game, err := r.Store.GetGameByID(ctx, mode.GameID)
	if err != nil {
		return nil, nil, err
	}
	return game, mode, nil
}

// modeForTable loads the game and catalog mode a table is playing.
func (r *Resolver) modeForTable(ctx context.Context, tableID uuid.UUID) (*store.Game, *store.GameMode, error) {
	table, err := r.Store.GetRoomTableByID(ctx, tableID)
	if err != nil {
		return nil, nil, err
	}
	mode, err := r.Store.GetGameModeByID(ctx, table.ModeID)
	if err != nil {
		return nil, nil, err
	}
	game, err := r.Store.GetGameByID(ctx, table.GameID)
	if err != nil {
		return nil, nil, err
	}
	return game, mode, nil
}

// --- GraphQL mapping ---

func toGraphQLPreQueueGroups(groups []prequeue.Group) []*model.PreQueueGroup {
	out := make([]*model.PreQueueGroup, 0, len(groups))
	for _, g := range groups {
		out = append(out, &model.PreQueueGroup{
			Key:   g.Key,
			Kind:  toGraphQLPreQueueKind(g.Kind),
			Label: g.Label,
			Min:   g.Min,
			Max:   g.Max,
		})
	}
	return out
}

func toGraphQLPreQueueKind(kind prequeue.Kind) model.PreQueueKind {
	switch kind {
	case prequeue.KindCharacter:
		return model.PreQueueKindCharacter
	case prequeue.KindDeck:
		return model.PreQueueKindDeck
	default:
		return model.PreQueueKindLoadout
	}
}

func toGraphQLQueueOptions(roster *gameclient.QueueOptionRoster) *model.QueueOptions {
	out := &model.QueueOptions{Available: true, Groups: []*model.QueueOptionGroup{}}
	if roster == nil {
		return out
	}
	for _, g := range roster.Groups {
		group := &model.QueueOptionGroup{Key: g.Key, Choices: make([]*model.QueueOption, 0, len(g.Choices))}
		for _, c := range g.Choices {
			choice := &model.QueueOption{
				ID:            c.ID,
				Label:         c.Label,
				Locked:        c.Locked,
				UnlockModeKey: c.UnlockModeKey,
			}
			if strings.TrimSpace(c.Description) != "" {
				description := c.Description
				choice.Description = &description
			}
			if c.Requirement != nil {
				choice.Requirement = toGraphQLRequirementNode(c.Requirement)
			}
			group.Choices = append(group.Choices, choice)
		}
		out.Groups = append(out.Groups, group)
	}
	return out
}

func unavailableQueueOptions(reason string) *model.QueueOptions {
	return &model.QueueOptions{
		Available:         false,
		UnavailableReason: &reason,
		Groups:            []*model.QueueOptionGroup{},
	}
}

func toGraphQLSelections(selections []prequeue.Selection) []*model.QueueOptionSelection {
	out := make([]*model.QueueOptionSelection, 0, len(selections))
	for _, sel := range selections {
		labels := sel.Labels
		if len(labels) != len(sel.OptionIDs) {
			// An older row stored before labels existed still names something.
			labels = sel.OptionIDs
		}
		out = append(out, &model.QueueOptionSelection{
			GroupKey:  sel.GroupKey,
			OptionIds: sel.OptionIDs,
			Labels:    labels,
		})
	}
	return out
}
