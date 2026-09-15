package store

import (
	"context"
	"testing"
	"time"
)

// The question the publish trigger has to ask: has this member's reading actually
// flipped, and if so, whose room needs telling. Past the window both halves are
// answered at once.
func TestRoomForDisconnectedMemberNamesTheRoomOncePastTheWindow(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	userID := newPresenceUser(t, st, cleaner, ctx)

	room, _ := roomAndDisconnect(t, st, ctx, userID)
	backdateDisconnect(t, st, ctx, userID, DefaultRoomRosterPresenceGrace+time.Second)

	roomID, ok, err := st.RoomForDisconnectedMember(ctx, userID, DefaultRoomRosterPresenceGrace)
	if err != nil {
		t.Fatalf("RoomForDisconnectedMember: %v", err)
	}
	if !ok {
		t.Fatal("no room named for a member the roster has stopped calling present, so nobody would be told")
	}
	if roomID != room.ID {
		t.Fatalf("named room %s, want the room the member is in (%s)", roomID, room.ID)
	}
}

// Inside the window the roster still calls the member present, so there is nothing to
// tell anyone. This is the half that keeps the publish honest: firing here would push a
// room whose reading has not changed, and — worse — would let a trigger armed on the
// wrong clock look like it worked.
func TestRoomForDisconnectedMemberNamesNothingInsideTheWindow(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	userID := newPresenceUser(t, st, cleaner, ctx)

	roomAndDisconnect(t, st, ctx, userID)

	if _, ok, err := st.RoomForDisconnectedMember(ctx, userID, DefaultRoomRosterPresenceGrace); err != nil {
		t.Fatalf("RoomForDisconnectedMember: %v", err)
	} else if ok {
		t.Fatalf("named a room for a member who dropped their socket moments ago, %s inside the window",
			DefaultRoomRosterPresenceGrace)
	}
}

// A player back before the trigger ran is not away, however old the stamp they left
// behind was. The reading and the trigger read the same row, so a reconnect closes both
// in one write.
func TestRoomForDisconnectedMemberNamesNothingForAReconnectedMember(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	userID := newPresenceUser(t, st, cleaner, ctx)

	roomAndDisconnect(t, st, ctx, userID)
	backdateDisconnect(t, st, ctx, userID, 10*DefaultRoomRosterPresenceGrace)
	if _, ok, err := st.RoomForDisconnectedMember(ctx, userID, DefaultRoomRosterPresenceGrace); err != nil || !ok {
		t.Fatalf("expected the long-gone member to name their room before reconnecting (ok=%t, err=%v)", ok, err)
	}

	if _, err := st.PresenceConnected(ctx, userID); err != nil {
		t.Fatalf("reconnect: %v", err)
	}

	if _, ok, err := st.RoomForDisconnectedMember(ctx, userID, DefaultRoomRosterPresenceGrace); err != nil {
		t.Fatalf("RoomForDisconnectedMember: %v", err)
	} else if ok {
		t.Fatal("named a room for a member who has reconnected")
	}
}

// A disconnected player who is in no room names nothing, rather than naming the zero
// uuid — the caller publishes to whatever it is handed, and a channel keyed on
// 00000000-… is a publish nobody receives and nobody notices.
func TestRoomForDisconnectedMemberNamesNothingForAPlayerInNoRoom(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()
	userID := newPresenceUser(t, st, cleaner, ctx)

	if _, err := st.PresenceConnected(ctx, userID); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if _, err := st.PresenceDisconnected(ctx, userID); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	backdateDisconnect(t, st, ctx, userID, DefaultRoomRosterPresenceGrace+time.Second)

	if _, ok, err := st.RoomForDisconnectedMember(ctx, userID, DefaultRoomRosterPresenceGrace); err != nil {
		t.Fatalf("RoomForDisconnectedMember: %v", err)
	} else if ok {
		t.Fatal("named a room for a player who is not in one")
	}
}
