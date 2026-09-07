package graph

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/graph/model"
	"github.com/scruffyprodigy/joinquest/internal/auth"
	"github.com/scruffyprodigy/joinquest/internal/pubsub"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

func toGraphQLRoom(room *store.Room) *model.Room {
	if room == nil {
		return nil
	}
	return &model.Room{
		ID:         room.ID.String(),
		InviteCode: room.InviteCode,
	}
}

func toGraphQLRoomMessage(msg *store.RoomMessage) *model.RoomMessage {
	if msg == nil {
		return nil
	}
	return &model.RoomMessage{
		ID:        msg.ID.String(),
		Body:      msg.Body,
		CreatedAt: msg.CreatedAt,
	}
}

func roomJoinURL(inviteCode string) string {
	base := strings.TrimRight(auth.LobbyPublicURL(), "/")
	if base == "" {
		base = "http://localhost:5173"
	}
	return fmt.Sprintf("%s/room/%s", base, strings.TrimSpace(inviteCode))
}

func (r *Resolver) loadRoomModel(ctx context.Context, roomID uuid.UUID) (*model.Room, error) {
	st, err := r.requireStore()
	if err != nil {
		return nil, err
	}
	room, err := st.GetRoomByID(ctx, roomID)
	if err != nil {
		return nil, err
	}
	return toGraphQLRoom(room), nil
}

func (r *Resolver) requireRoomMember(ctx context.Context, roomID, userID uuid.UUID) error {
	st, err := r.requireStore()
	if err != nil {
		return err
	}
	ok, err := st.IsRoomMember(ctx, roomID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("not a member of this room")
	}
	return nil
}

func (r *Resolver) publishRoomUpdated(ctx context.Context, roomID uuid.UUID) error {
	if r.PubSub == nil {
		return nil
	}
	return pubsub.PublishRoomEvent(ctx, r.PubSub, roomID.String(), pubsub.RoomEvent{
		Type: pubsub.RoomEventUpdated,
	})
}

func (r *Resolver) publishRoomMessage(ctx context.Context, roomID, messageID uuid.UUID) error {
	if r.PubSub == nil {
		return nil
	}
	return pubsub.PublishRoomEvent(ctx, r.PubSub, roomID.String(), pubsub.RoomEvent{
		Type:      pubsub.RoomEventMessage,
		MessageID: messageID.String(),
	})
}

func (r *Resolver) roomAfterMutation(ctx context.Context, room *store.Room) (*model.Room, error) {
	if err := r.publishRoomUpdated(ctx, room.ID); err != nil {
		return nil, err
	}
	return toGraphQLRoom(room), nil
}
