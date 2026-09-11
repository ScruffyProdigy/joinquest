package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestRoomCreateJoinLeave(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	host, err := st.CreateUser(ctx, CreateUserParams{Email: "host-room@example.com"})
	if err != nil {
		t.Fatalf("create host: %v", err)
	}
	cleaner.TrackUser(host.ID)
	guest, err := st.CreateUser(ctx, CreateUserParams{Email: "guest-room@example.com"})
	if err != nil {
		t.Fatalf("create guest: %v", err)
	}
	cleaner.TrackUser(guest.ID)

	room, err := st.CreateRoom(ctx, host.ID)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	if len(room.InviteCode) != roomInviteCodeLen {
		t.Fatalf("invite code length = %d, want %d", len(room.InviteCode), roomInviteCodeLen)
	}

	got, err := st.GetUserRoom(ctx, host.ID)
	if err != nil {
		t.Fatalf("get user room: %v", err)
	}
	if got.ID != room.ID {
		t.Fatalf("host room id mismatch")
	}

	joined, err := st.JoinRoom(ctx, guest.ID, room.InviteCode)
	if err != nil {
		t.Fatalf("join room: %v", err)
	}
	if joined.ID != room.ID {
		t.Fatalf("joined wrong room")
	}

	members, err := st.ListRoomRoster(ctx, room.ID, DefaultRoomRosterPresenceGrace)
	if err != nil {
		t.Fatalf("list members: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("member count = %d, want 2", len(members))
	}

	left, err := st.LeaveRoom(ctx, guest.ID)
	if err != nil || !left {
		t.Fatalf("leave room: left=%v err=%v", left, err)
	}
	members, err = st.ListRoomRoster(ctx, room.ID, DefaultRoomRosterPresenceGrace)
	if err != nil {
		t.Fatalf("list members after leave: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("member count after leave = %d, want 1", len(members))
	}
}

func TestRoomOneRoomPerUser(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	user, err := st.CreateUser(ctx, CreateUserParams{Email: "solo-room@example.com"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	cleaner.TrackUser(user.ID)

	roomA, err := st.CreateRoom(ctx, user.ID)
	if err != nil {
		t.Fatalf("create room A: %v", err)
	}
	roomB, err := st.CreateRoom(ctx, user.ID)
	if err != nil {
		t.Fatalf("create room B: %v", err)
	}
	if roomA.ID == roomB.ID {
		t.Fatal("expected new room on second create")
	}

	got, err := st.GetUserRoom(ctx, user.ID)
	if err != nil {
		t.Fatalf("get user room: %v", err)
	}
	if got.ID != roomB.ID {
		t.Fatalf("user should be in newest room")
	}

	members, err := st.ListRoomRoster(ctx, roomA.ID, DefaultRoomRosterPresenceGrace)
	if err != nil {
		t.Fatalf("list room A members: %v", err)
	}
	if len(members) != 0 {
		t.Fatalf("room A should be empty after host recreated room")
	}
}

func TestRoomMessages(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	user, err := st.CreateUser(ctx, CreateUserParams{Email: "chat-room@example.com"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	cleaner.TrackUser(user.ID)
	room, err := st.CreateRoom(ctx, user.ID)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}

	msg, err := st.SendRoomMessage(ctx, room.ID, user.ID, "hello room")
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if msg.Body != "hello room" {
		t.Fatalf("unexpected body %q", msg.Body)
	}

	msgs, err := st.ListRoomMessages(ctx, room.ID, 10, nil)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(msgs) != 1 || msgs[0].ID != msg.ID {
		t.Fatalf("expected one message, got %+v", msgs)
	}

	other := uuid.New()
	if _, err := st.SendRoomMessage(ctx, room.ID, other, "nope"); err == nil {
		t.Fatal("expected non-member send to fail")
	}
}

func TestRoomHostTransferOnLeave(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	host, _ := st.CreateUser(ctx, CreateUserParams{Email: "host-transfer@example.com"})
	cleaner.TrackUser(host.ID)
	guest, _ := st.CreateUser(ctx, CreateUserParams{Email: "guest-transfer@example.com"})
	cleaner.TrackUser(guest.ID)

	room, err := st.CreateRoom(ctx, host.ID)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	if _, err := st.JoinRoom(ctx, guest.ID, room.InviteCode); err != nil {
		t.Fatalf("join: %v", err)
	}
	if _, err := st.LeaveRoom(ctx, host.ID); err != nil {
		t.Fatalf("host leave: %v", err)
	}

	updated, err := st.GetRoomByID(ctx, room.ID)
	if err != nil {
		t.Fatalf("get room: %v", err)
	}
	if updated.HostUserID != guest.ID {
		t.Fatalf("host should transfer to guest")
	}
}
