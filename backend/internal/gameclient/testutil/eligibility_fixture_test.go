package testutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEligibilityFixtureHandlerLockStateMatching(t *testing.T) {
	tests := []struct {
		name             string
		lobbyUserId      string
		expectedUnlocked bool
	}{
		{
			name:             "suffix -unlocked is treated as unlocked",
			lobbyUserId:      "player-legendary-unlocked",
			expectedUnlocked: true,
		},
		{
			name:             "substring -unlocked in middle is treated as locked",
			lobbyUserId:      "bob-unlocked-002",
			expectedUnlocked: false,
		},
		{
			name:             "suffix -locked is treated as locked",
			lobbyUserId:      "player-arena-locked",
			expectedUnlocked: false,
		},
		{
			name:             "no suffix defaults to locked",
			lobbyUserId:      "regular-player",
			expectedUnlocked: false,
		},
	}

	handler := EligibilityFixtureHandler()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := "/api/v1/players/" + tt.lobbyUserId + "/mode-eligibility"
			req := httptest.NewRequest("GET", path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d", rec.Code)
			}

			var resp map[string]any
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			modes, ok := resp["modes"].(map[string]any)
			if !ok {
				t.Fatal("expected modes to be a map")
			}

			legendary, ok := modes["legendary"].(map[string]any)
			if !ok {
				t.Fatal("expected legendary to be a map")
			}

			accessible, ok := legendary["accessible"].(bool)
			if !ok {
				t.Fatal("expected accessible to be a bool")
			}

			if accessible != tt.expectedUnlocked {
				t.Errorf("expected accessible=%v, got %v", tt.expectedUnlocked, accessible)
			}
		})
	}
}
