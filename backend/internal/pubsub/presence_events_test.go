package pubsub

import (
	"context"
	"testing"
)

func TestUserPresenceChannelIsPerUserAndDistinctFromQueue(t *testing.T) {
	got := UserPresenceChannel("abc")
	if got != "lobby:user:abc:presence" {
		t.Fatalf("channel: got %q", got)
	}
	// Presence and queue events have different shapes; sharing a channel would make
	// every queue subscriber try to unmarshal presence payloads.
	if got == UserQueueChannel("abc") {
		t.Fatal("presence must not share the queue channel")
	}
}

func TestPresenceEventRoundTrips(t *testing.T) {
	in := PresenceEvent{Status: PresenceStatusDisconnected, DisconnectedAt: "2026-09-09T12:00:00Z"}
	payload, err := MarshalPresenceEvent(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out, err := UnmarshalPresenceEvent(payload)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out != in {
		t.Fatalf("round trip: got %+v, want %+v", out, in)
	}
}

func TestPublishPresenceEventReachesASubscriber(t *testing.T) {
	broker := NewMemory()
	defer broker.Close()
	ctx := context.Background()

	messages, unsubscribe, err := broker.Subscribe(ctx, UserPresenceChannel("u1"))
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer unsubscribe()

	want := PresenceEvent{Status: PresenceStatusConnected}
	if err := PublishPresenceEvent(ctx, broker, "u1", want); err != nil {
		t.Fatalf("publish: %v", err)
	}

	payload := <-messages
	got, err := UnmarshalPresenceEvent(payload)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got != want {
		t.Fatalf("delivered: got %+v, want %+v", got, want)
	}
}

func TestPublishPresenceEventRequiresABroker(t *testing.T) {
	if err := PublishPresenceEvent(context.Background(), nil, "u1", PresenceEvent{}); err == nil {
		t.Fatal("expected an error for a nil broker, matching PublishQueueEvent")
	}
}
