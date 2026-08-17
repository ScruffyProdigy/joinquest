package gameclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newEligibilityTestServer(t *testing.T, body string, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/players/player-1/mode-eligibility" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func TestFetchModeEligibilityLeaf(t *testing.T) {
	srv := newEligibilityTestServer(t, `{
		"modes": {
			"legendary": {
				"accessible": false,
				"reason": "Complete 50 Ranked matches to unlock.",
				"requirement": {"kind": "leaf", "label": "Ranked matches", "current": 12, "target": 50}
			}
		}
	}`, http.StatusOK)
	defer srv.Close()

	c := NewClient()
	result, err := c.FetchModeEligibility(context.Background(), srv.URL, "player-1")
	if err != nil {
		t.Fatalf("FetchModeEligibility failed: %v", err)
	}
	legendary, ok := result["legendary"]
	if !ok {
		t.Fatal("expected legendary mode in result")
	}
	if legendary.Accessible {
		t.Fatal("expected legendary to be inaccessible")
	}
	if legendary.Requirement == nil || legendary.Requirement.Current != 12 || legendary.Requirement.Target != 50 {
		t.Fatalf("unexpected requirement: %+v", legendary.Requirement)
	}
}

func TestFetchModeEligibilityCompoundGroup(t *testing.T) {
	srv := newEligibilityTestServer(t, `{
		"modes": {
			"commander": {
				"accessible": false,
				"reason": "Requires 25 Standard wins and 5 unique decks used.",
				"requirement": {
					"kind": "group",
					"label": "Commander requirements",
					"operator": "all",
					"children": [
						{"kind": "leaf", "label": "Standard wins", "current": 18, "target": 25},
						{"kind": "leaf", "label": "Unique decks used", "current": 3, "target": 5}
					]
				}
			}
		}
	}`, http.StatusOK)
	defer srv.Close()

	c := NewClient()
	result, err := c.FetchModeEligibility(context.Background(), srv.URL, "player-1")
	if err != nil {
		t.Fatalf("FetchModeEligibility failed: %v", err)
	}
	commander := result["commander"]
	if commander.Requirement == nil || commander.Requirement.Kind != "group" {
		t.Fatalf("expected group requirement, got %+v", commander.Requirement)
	}
	if len(commander.Requirement.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(commander.Requirement.Children))
	}
}

func TestFetchModeEligibilityMissingEndpointFailsOpen(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := NewClient()
	result, err := c.FetchModeEligibility(context.Background(), srv.URL, "player-1")
	if err != nil {
		t.Fatalf("expected no error on 404, got %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty result map on 404, got %+v", result)
	}
}

func TestFetchModeEligibilityDropsInvalidOperator(t *testing.T) {
	srv := newEligibilityTestServer(t, `{
		"modes": {
			"commander": {
				"accessible": false,
				"reason": "bad data",
				"requirement": {"kind": "group", "label": "x", "operator": "xor", "children": []}
			},
			"legendary": {
				"accessible": true
			}
		}
	}`, http.StatusOK)
	defer srv.Close()

	c := NewClient()
	result, err := c.FetchModeEligibility(context.Background(), srv.URL, "player-1")
	if err != nil {
		t.Fatalf("FetchModeEligibility failed: %v", err)
	}
	if _, ok := result["commander"]; ok {
		t.Fatal("expected commander to be dropped due to invalid operator")
	}
	if _, ok := result["legendary"]; !ok {
		t.Fatal("expected legendary to still be present")
	}
}
