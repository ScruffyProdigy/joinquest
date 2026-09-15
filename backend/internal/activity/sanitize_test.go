package activity

import "testing"

func TestForbiddenPayloadKey(t *testing.T) {
	cases := []struct {
		key       string
		forbidden bool
		why       string
	}{
		// Personal data, in the spellings it actually turns up in.
		{"email", true, "bare"},
		{"player_email", true, "snake_case"},
		{"contactEmail", true, "camelCase"},
		{"display_name", true, "the common one"},
		{"username", true, "survives tokenisation as one word"},
		{"ip", true, "bare"},
		{"client_ip", true, "snake_case"},
		{"ipAddress", true, "camelCase splits on the hump"},
		{"user_agent", true, "both halves"},
		{"userAgent", true, "camelCase"},
		{"auth_token", true, "credential"},
		{"phone_number", true, "phone half is enough"},

		// Legitimate signals that a substring rule would have eaten. These are the
		// regressions worth pinning: participant_count contains "ip", and losing it
		// silently would leave an analysis short with nothing to point at.
		{"participant_count", false, "contains ip"},
		{"participants", false, "contains ip"},
		{"recipient_kind", false, "contains ip"},
		{"equipment_slot", false, "contains ip"},

		// Ordinary payload keys these events actually carry.
		{"reason", false, "finish reason"},
		{"waited_seconds", false, "duration"},
		{"queue_path", false, "routing"},
		{"mode_key", false, "mode identity"},
		{"placement", false, "result"},
		{"session_id", false, "an id, not a person"},
		{"party_id", false, "an id, not a person"},
	}

	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			if got := forbiddenPayloadKey(tc.key); got != tc.forbidden {
				t.Fatalf("forbiddenPayloadKey(%q) = %v, want %v (%s)", tc.key, got, tc.forbidden, tc.why)
			}
		})
	}
}

func TestSanitizePayloadStripsOnlyForbiddenKeys(t *testing.T) {
	cleaned := sanitizePayload(map[string]any{
		"reason":            "FORFEIT",
		"participant_count": 4,
		"display_name":      "Ryan",
		"client_ip":         "203.0.113.7",
	})

	if _, ok := cleaned["display_name"]; ok {
		t.Error("display_name survived sanitising")
	}
	if _, ok := cleaned["client_ip"]; ok {
		t.Error("client_ip survived sanitising")
	}
	if cleaned["reason"] != "FORFEIT" {
		t.Errorf("reason = %v, want FORFEIT", cleaned["reason"])
	}
	if cleaned["participant_count"] != 4 {
		t.Errorf("participant_count = %v, want 4", cleaned["participant_count"])
	}
}

// An event whose payload was entirely personal data still records -- the event
// happening is itself the signal, and dropping it would trade a privacy problem for
// a data problem.
func TestSanitizePayloadReturnsNilWhenEverythingIsStripped(t *testing.T) {
	if got := sanitizePayload(map[string]any{"email": "a@b.c"}); got != nil {
		t.Fatalf("sanitizePayload = %v, want nil", got)
	}
	if got := sanitizePayload(nil); got != nil {
		t.Fatalf("sanitizePayload(nil) = %v, want nil", got)
	}
	if got := sanitizePayload(map[string]any{}); got != nil {
		t.Fatalf("sanitizePayload(empty) = %v, want nil", got)
	}
}

func TestPayloadKeyTokens(t *testing.T) {
	cases := []struct {
		key  string
		want []string
	}{
		{"participant_count", []string{"participant", "count"}},
		{"ipAddress", []string{"ip", "address"}},
		{"user_agent", []string{"user", "agent"}},
		{"queue-path", []string{"queue", "path"}},
		{"sessionID", []string{"session", "id"}},
		{"waited_seconds_p95", []string{"waited", "seconds", "p95"}},
	}

	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			got := payloadKeyTokens(tc.key)
			if len(got) != len(tc.want) {
				t.Fatalf("payloadKeyTokens(%q) = %v, want %v", tc.key, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("payloadKeyTokens(%q) = %v, want %v", tc.key, got, tc.want)
				}
			}
		})
	}
}
