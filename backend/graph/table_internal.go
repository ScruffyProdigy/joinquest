package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/graph/model"
	"github.com/scruffyprodigy/joinquest/internal/pubsub"
	"github.com/scruffyprodigy/joinquest/internal/seattemplate"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

func toGraphQLTable(table *store.RoomTable) *model.Table {
	if table == nil {
		return nil
	}
	return &model.Table{
		ID:        table.ID.String(),
		CreatedAt: table.CreatedAt,
		Status:    table.Status,
	}
}

func (r *Resolver) loadTableModel(ctx context.Context, tableID uuid.UUID) (*model.Table, error) {
	st, err := r.requireStore()
	if err != nil {
		return nil, err
	}
	table, err := st.GetRoomTableByID(ctx, tableID)
	if err != nil {
		return nil, err
	}
	return toGraphQLTable(table), nil
}

func (r *Resolver) publishTableUpdated(ctx context.Context, roomID, tableID uuid.UUID) error {
	if r.PubSub == nil {
		return nil
	}
	return pubsub.PublishRoomEvent(ctx, r.PubSub, roomID.String(), pubsub.RoomEvent{
		Type:    pubsub.RoomEventTableUpdated,
		TableID: tableID.String(),
	})
}

func (r *Resolver) tableAfterMutation(ctx context.Context, table *store.RoomTable) (*model.Table, error) {
	if err := r.publishTableUpdated(ctx, table.RoomID, table.ID); err != nil {
		return nil, err
	}
	return toGraphQLTable(table), nil
}

func seatDisplayName(modeSeats []store.GameModeSeat, seatKey string, template json.RawMessage) string {
	for _, seat := range modeSeats {
		if seat.SeatKey != seatKey {
			continue
		}
		path := ""
		if seat.QueuePath != nil {
			path = strings.TrimSpace(*seat.QueuePath)
		}
		if path != "" {
			roleLabel := path
			if specs, err := seattemplate.PathSpecs(template); err == nil {
				for _, spec := range specs {
					if spec.QueuePath == path {
						roleLabel = spec.DisplayName
						break
					}
				}
			}
			if suffix := seatKeySuffix(seatKey, path); suffix != "" {
				return roleLabel + " · " + suffix
			}
			return roleLabel
		}
		return seatKey
	}
	return seatKey
}

func seatKeySuffix(seatKey, queuePath string) string {
	prefix := queuePath + "-"
	if strings.HasPrefix(seatKey, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(seatKey, prefix))
	}
	return ""
}

func (r *Resolver) buildTableSeatSlots(ctx context.Context, tableID uuid.UUID) ([]*model.TableSeatSlot, error) {
	st, err := r.requireStore()
	if err != nil {
		return nil, err
	}
	table, err := st.GetRoomTableByID(ctx, tableID)
	if err != nil {
		return nil, err
	}
	mode, err := st.GetGameModeByID(ctx, table.ModeID)
	if err != nil {
		return nil, err
	}
	modeSeats, err := st.ListGameModeSeats(ctx, table.ModeID)
	if err != nil {
		return nil, err
	}
	seated, err := st.ListTableSeats(ctx, tableID)
	if err != nil {
		return nil, err
	}
	byKey := map[string]uuid.UUID{}
	for _, s := range seated {
		byKey[s.SeatKey] = s.UserID
	}

	out := make([]*model.TableSeatSlot, len(modeSeats))
	for i, seat := range modeSeats {
		slot := &model.TableSeatSlot{
			SeatKey:     seat.SeatKey,
			DisplayName: seatDisplayName(modeSeats, seat.SeatKey, mode.SeatTemplate),
			TeamPrefix:  storeTeamPrefix(seat.SeatKey),
		}
		if seat.QueuePath != nil {
			slot.QueuePath = seat.QueuePath
		}
		if uid, ok := byKey[seat.SeatKey]; ok {
			user, uErr := st.GetUserByID(ctx, uid)
			if uErr == nil {
				slot.User = ToGraphQLPublicPlayer(user)
			}
		}
		out[i] = slot
	}
	return out, nil
}

func storeTeamPrefix(seatKey string) *string {
	prefix := teamPrefixFromSeatKey(seatKey)
	if prefix == "" {
		return nil
	}
	return &prefix
}

func teamPrefixFromSeatKey(seatKey string) string {
	parts := strings.Split(seatKey, "-")
	if len(parts) >= 2 && strings.EqualFold(parts[0], "Team") {
		return parts[0] + "-" + parts[1]
	}
	return ""
}

func tableLookForGroupVisible(mode *store.GameMode, modeSeats []store.GameModeSeat, seated []store.TableSeat) (bool, error) {
	if len(seated) < 1 {
		return false, nil
	}
	specs, err := seattemplate.PathSpecs(mode.SeatTemplate)
	if err != nil {
		return false, err
	}
	if len(specs) == 0 {
		return len(seated) < mode.MaxPlayers, nil
	}
	for _, spec := range specs {
		if countSeatedInPath(seated, modeSeats, spec.QueuePath) < spec.Max {
			return true, nil
		}
	}
	return false, nil
}

func countSeatedInPath(seated []store.TableSeat, modeSeats []store.GameModeSeat, queuePath string) int {
	byKey := make(map[string]string, len(modeSeats))
	for _, seat := range modeSeats {
		path := ""
		if seat.QueuePath != nil {
			path = strings.TrimSpace(*seat.QueuePath)
		}
		byKey[seat.SeatKey] = path
	}
	count := 0
	for _, s := range seated {
		if byKey[s.SeatKey] == queuePath {
			count++
		}
	}
	return count
}

func (r *Resolver) buildLookForGroupOptions(ctx context.Context, tableID uuid.UUID) ([]*model.TableLookForGroupOption, error) {
	st, err := r.requireStore()
	if err != nil {
		return nil, err
	}
	table, err := st.GetRoomTableByID(ctx, tableID)
	if err != nil {
		return nil, err
	}
	seated, err := st.ListTableSeats(ctx, tableID)
	if err != nil {
		return nil, err
	}
	mode, err := st.GetGameModeByID(ctx, table.ModeID)
	if err != nil {
		return nil, err
	}
	modeSeats, err := st.ListGameModeSeats(ctx, table.ModeID)
	if err != nil {
		return nil, err
	}
	visibleBase, err := tableLookForGroupVisible(mode, modeSeats, seated)
	if err != nil {
		return nil, err
	}
	backfillActive, err := st.TableBackfillActive(ctx, tableID)
	if err != nil {
		return nil, err
	}

	queues, err := st.ListModeQueuesByModeID(ctx, table.ModeID)
	if err != nil {
		return nil, err
	}
	out := make([]*model.TableLookForGroupOption, 0, len(queues))
	for _, q := range queues {
		if q.Status != store.ModeQueueActive {
			continue
		}
		out = append(out, &model.TableLookForGroupOption{
			QueueID:   q.ID.String(),
			QueueName: q.Name,
			Visible:   visibleBase,
			Enabled:   visibleBase && !backfillActive,
		})
	}
	return out, nil
}

func toGraphQLMyTableSeat(view *store.UserTableSeatView, seatDisplayName, status, joinURL string) *model.MyTableSeat {
	if view == nil {
		return nil
	}
	out := &model.MyTableSeat{
		TableID:         view.TableID.String(),
		RoomID:          view.RoomID.String(),
		InviteCode:      view.InviteCode,
		GameID:          view.GameID.String(),
		GameName:        view.GameName,
		ModeID:          view.ModeID.String(),
		ModeName:        view.ModeName,
		SeatKey:         view.SeatKey,
		SeatDisplayName: seatDisplayName,
		Status:          status,
	}
	if strings.TrimSpace(joinURL) != "" {
		out.JoinURL = &joinURL
	}
	return out
}

func (r *queryResolver) resolveMyTableSeat(ctx context.Context, userID uuid.UUID) (*model.MyTableSeat, error) {
	st, err := r.requireStore()
	if err != nil {
		return nil, err
	}

	view, err := st.GetUserTableSeat(ctx, userID)
	if err != nil {
		return nil, err
	}
	if view != nil {
		mode, err := st.GetGameModeByID(ctx, view.ModeID)
		if err != nil {
			return nil, err
		}
		modeSeats, err := st.ListGameModeSeats(ctx, view.ModeID)
		if err != nil {
			return nil, err
		}
		display := seatDisplayName(modeSeats, view.SeatKey, mode.SeatTemplate)
		return toGraphQLMyTableSeat(view, display, "forming", ""), nil
	}

	started, err := st.GetUserStartedTableSession(ctx, userID)
	if err != nil {
		return nil, err
	}
	if started == nil || started.SessionID == nil {
		return nil, nil
	}
	mode, err := st.GetGameModeByID(ctx, started.ModeID)
	if err != nil {
		return nil, err
	}
	modeSeats, err := st.ListGameModeSeats(ctx, started.ModeID)
	if err != nil {
		return nil, err
	}
	display := seatDisplayName(modeSeats, started.SeatKey, mode.SeatTemplate)
	game, err := st.GetGameByID(ctx, started.GameID)
	if err != nil {
		return nil, err
	}
	launch, err := r.signLaunchURL(ctx, game, *started.SessionID, userID)
	if err != nil {
		return toGraphQLMyTableSeat(started, display, "started", ""), nil
	}
	return toGraphQLMyTableSeat(started, display, "started", launch), nil
}

func (r *Resolver) myTableSeatUpdatedSubscription(ctx context.Context) (<-chan *model.MyTableSeat, error) {
	if r.PubSub == nil {
		return nil, fmt.Errorf("pubsub is not configured")
	}
	userID, err := requireAuthUserID(ctx)
	if err != nil {
		return nil, err
	}

	qr := &queryResolver{r}
	initial, err := qr.resolveMyTableSeat(ctx, userID)
	if err != nil {
		initial = nil
	}

	messages, unsubscribe, err := r.PubSub.Subscribe(ctx, pubsub.UserTableSeatChannel(userID.String()))
	if err != nil {
		return nil, err
	}

	releasePresence := r.Presence.Track(ctx, userID)

	updates := make(chan *model.MyTableSeat, 4)
	go func() {
		defer close(updates)
		defer unsubscribe()
		defer releasePresence()

		if initial != nil {
			select {
			case updates <- initial:
			case <-ctx.Done():
				return
			}
		}

		for {
			select {
			case <-ctx.Done():
				return
			case payload, ok := <-messages:
				if !ok {
					return
				}
				event, err := pubsub.UnmarshalTableSeatEvent(payload)
				if err != nil {
					continue
				}
				switch event.Status {
				case pubsub.TableSeatStatusStarted, pubsub.TableSeatStatusLeft:
				default:
					continue
				}
				seat, err := qr.resolveMyTableSeat(ctx, userID)
				if err != nil {
					seat = nil
				}
				if seat != nil && event.JoinURL != "" && (seat.JoinURL == nil || *seat.JoinURL == "") {
					joinURL := event.JoinURL
					seat.JoinURL = &joinURL
				}
				if seat == nil {
					continue
				}
				select {
				case updates <- seat:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return updates, nil
}

func (r *mutationResolver) startTableInternal(ctx context.Context, tableID uuid.UUID) (*model.JoinResult, error) {
	st, err := r.requireStore()
	if err != nil {
		return nil, err
	}
	userID, err := requireAuthUserID(ctx)
	if err != nil {
		return nil, err
	}

	table, err := st.GetRoomTableByID(ctx, tableID)
	if err != nil {
		return nil, err
	}

	result, err := st.StartTable(ctx, tableID, userID)
	if err != nil {
		return nil, err
	}

	game, err := st.GetGameByID(ctx, result.GameID)
	if err != nil {
		return nil, err
	}
	launchURLs, err := r.finalizeMatchedSession(ctx, game, result.SessionID, result.NotifyUserIDs)
	if err != nil {
		return nil, err
	}
	if err := r.publishTableSeatStarted(ctx, tableID, result.NotifyUserIDs, launchURLs); err != nil {
		return nil, err
	}
	if err := r.publishTableUpdated(ctx, table.RoomID, tableID); err != nil {
		return nil, err
	}

	launch := launchURLs[userID]
	sessionID := result.SessionID.String()
	return &model.JoinResult{
		Queued:    false,
		SessionID: &sessionID,
		JoinURL:   &launch,
	}, nil
}

func (r *Resolver) requireTableRoomMember(ctx context.Context, tableID, userID uuid.UUID) (*store.RoomTable, error) {
	st, err := r.requireStore()
	if err != nil {
		return nil, err
	}
	table, err := st.GetRoomTableByID(ctx, tableID)
	if err != nil {
		return nil, err
	}
	if err := r.requireRoomMember(ctx, table.RoomID, userID); err != nil {
		return nil, err
	}
	return table, nil
}

func tableRoomIDFromObj(obj *model.Table) (uuid.UUID, error) {
	if obj == nil || strings.TrimSpace(obj.ID) == "" {
		return uuid.Nil, fmt.Errorf("table is required")
	}
	tableID, err := parseUUID(obj.ID, "table id")
	if err != nil {
		return uuid.Nil, err
	}
	return tableID, nil
}
