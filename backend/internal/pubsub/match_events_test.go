package pubsub

import "testing"

func TestMatchEventRoundTrip(t *testing.T) {
	payload, err := MarshalMatchEvent(MatchEvent{Type: MatchEventResult, MatchID: "abc"})
	if err != nil {
		t.Fatalf("MarshalMatchEvent: %v", err)
	}
	got, err := UnmarshalMatchEvent(payload)
	if err != nil {
		t.Fatalf("UnmarshalMatchEvent: %v", err)
	}
	if got.Type != MatchEventResult || got.MatchID != "abc" {
		t.Fatalf("round trip = %+v", got)
	}
	if MatchChannel("abc") != "lobby:match:abc" {
		t.Fatalf("MatchChannel = %q", MatchChannel("abc"))
	}
}
