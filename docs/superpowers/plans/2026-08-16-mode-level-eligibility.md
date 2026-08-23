# Mode-Level Eligibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the generic mode-level eligibility mechanism from [docs/superpowers/specs/2026-08-16-mode-level-eligibility-design.md](../specs/2026-08-16-mode-level-eligibility-design.md), covering JQ-11, JQ-12, JQ-13, and JQ-16 in one PR.

**Architecture:** A new `GameMode.eligibility(playerId: ID!)` GraphQL field, resolved by calling an optional per-player HTTP endpoint on the game server (fail-open when absent), cached briefly in-memory. A generic recursive leaf/group requirement tree is exposed through a typed GraphQL interface. A seeded fixture game + standalone fixture HTTP server stand in for the real (nonexistent) prototype games, demonstrating all four gates end-to-end.

**Tech Stack:** Go (gqlgen GraphQL, Postgres via `lib/pq`, stdlib `net/http`/`testing`), React (Vitest + React Testing Library).

## Global Constraints

- GraphQL type/field names must match the spec's "Data model" section exactly: `ModeEligibility`, `ModeRequirementNode`, `RequirementLeaf`, `RequirementGroup`, `RequirementOperator` (`ALL`/`ANY`), `unlockModeKey`.
- No persistence of eligibility state in JoinQuest's own database — the game server is the source of truth; the only new DB rows are the fixture game/mode catalog entries.
- New migration file uses the next sequential number: **000038** (highest existing is `000037_guest_accounts_and_identities`).
- Go tests use plain stdlib `testing` (`t.Fatalf`), not testify — matches every existing test in this codebase.
- A game server that doesn't implement the eligibility endpoint (404/timeout/malformed response) must never break existing behavior — every mode defaults to `accessible: true`.
- Frontend tests use Vitest + React Testing Library, matching `frontend/src/components/games/GameQueueActions.test.jsx`.

---

### Task 1: GraphQL schema + codegen

**Files:**
- Modify: `backend/graph/schema/catalog.graphqls`
- Modify: `backend/gqlgen.yml`
- Generated (do not hand-edit, verify after running codegen): `backend/graph/model/models_gen.go`, `backend/graph/generated/generated.go`, `backend/graph/catalog.resolvers.go` (new stub method appended)

**Interfaces:**
- Produces: GraphQL types `ModeEligibility{Accessible bool, Reason *string, Requirement ModeRequirementNode, UnlockModeKey *string}`, `ModeRequirementNode` interface, `RequirementLeaf{Label string, Current int, Target int}`, `RequirementGroup{Label string, Operator RequirementOperator, Children []ModeRequirementNode}`, enum `RequirementOperator{ALL, ANY}`. Field `GameMode.Eligibility(ctx, playerID string) (*model.ModeEligibility, error)` stub for Task 7 to implement.

- [ ] **Step 1: Add the new types to the schema**

Add to `backend/graph/schema/catalog.graphqls`, after the existing `GameMode` type:

```graphql
type ModeEligibility {
  accessible: Boolean!
  reason: String
  requirement: ModeRequirementNode
  unlockModeKey: String
}

interface ModeRequirementNode {
  label: String!
}

type RequirementLeaf implements ModeRequirementNode {
  label: String!
  current: Int!
  target: Int!
}

type RequirementGroup implements ModeRequirementNode {
  label: String!
  operator: RequirementOperator!
  children: [ModeRequirementNode!]!
}

enum RequirementOperator {
  ALL
  ANY
}
```

Then add the new field to the existing `GameMode` type (do not create a second `extend type GameMode` block — add the line inside the existing `type GameMode { ... }` definition, alongside `queues: [ModeQueue!]!`):

```graphql
type GameMode {
  id: ID!
  modeKey: String!
  displayName: String!
  minPlayers: Int!
  maxPlayers: Int!
  status: String!
  seats: [GameModeSeat!]!
  queuePaths: [GameModeQueuePath!]!
  queues: [ModeQueue!]!
  eligibility(playerId: ID!): ModeEligibility
}
```

- [ ] **Step 2: Mark `eligibility` as resolver-backed in gqlgen.yml**

Open `backend/gqlgen.yml`. The `models: GameMode: fields:` block (around line 26) currently reads:

```yaml
  GameMode:
    fields:
      seats:
        resolver: true
      queuePaths:
        resolver: true
      queues:
        resolver: true
```

Add a fourth entry at the same indentation as `seats`/`queuePaths`/`queues`:

```yaml
      eligibility:
        resolver: true
```

- [ ] **Step 3: Run codegen**

```bash
cd backend && go run github.com/99designs/gqlgen@v0.17.81 generate
```

- [ ] **Step 4: Verify it builds and drift check passes**

```bash
cd backend && go build ./... && go test ./graph -run=TestGqlgen -v
```
Expected: build succeeds; drift test passes (confirms generated code matches schema).

- [ ] **Step 5: Commit**

```bash
git add backend/graph/schema/catalog.graphqls backend/gqlgen.yml backend/graph/model/models_gen.go backend/graph/generated/generated.go backend/graph/catalog.resolvers.go
git commit -m "Add ModeEligibility GraphQL schema and generated code"
```

---

### Task 2: gameclient — eligibility HTTP fetch

**Files:**
- Create: `backend/internal/gameclient/eligibility.go`
- Test: `backend/internal/gameclient/eligibility_test.go`

**Interfaces:**
- Consumes: `Client` struct and `c.httpClient` field from `backend/internal/gameclient/client.go`; `gameurl.ValidateOutboundURL(ctx, base, isProd bool) error` and `runtimeenv.IsProductionEnv() bool` (already imported by `client.go`).
- Produces: `type RequirementNode struct{Kind, Label string; Current, Target int; Operator string; Children []RequirementNode}`, `type ModeEligibility struct{Accessible bool; Reason string; Requirement *RequirementNode; UnlockModeKey *string}`, `func (c *Client) FetchModeEligibility(ctx context.Context, apiBaseURL, lobbyUserID string) (map[string]ModeEligibility, error)`.

- [ ] **Step 1: Write the failing tests**

Create `backend/internal/gameclient/eligibility_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd backend && go test ./internal/gameclient/ -run TestFetchModeEligibility -v
```
Expected: FAIL with `c.FetchModeEligibility undefined` (compile error).

- [ ] **Step 3: Implement**

Create `backend/internal/gameclient/eligibility.go`:

```go
package gameclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/scruffyprodigy/playhub/internal/gameurl"
	"github.com/scruffyprodigy/playhub/internal/runtimeenv"
)

// RequirementNode is one leaf or group node in a mode's eligibility requirement tree.
type RequirementNode struct {
	Kind     string            `json:"kind"`
	Label    string            `json:"label"`
	Current  int               `json:"current,omitempty"`
	Target   int               `json:"target,omitempty"`
	Operator string            `json:"operator,omitempty"`
	Children []RequirementNode `json:"children,omitempty"`
}

// valid reports whether this node and its descendants use recognized kind/operator values.
func (n *RequirementNode) valid() bool {
	switch n.Kind {
	case "leaf":
		return true
	case "group":
		op := strings.ToLower(n.Operator)
		if op != "all" && op != "any" {
			return false
		}
		for i := range n.Children {
			if !n.Children[i].valid() {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// ModeEligibility is one game mode's per-player accessibility, as reported by a game server.
type ModeEligibility struct {
	Accessible    bool             `json:"accessible"`
	Reason        string           `json:"reason"`
	Requirement   *RequirementNode `json:"requirement"`
	UnlockModeKey *string          `json:"unlockModeKey"`
}

type modeEligibilityBatch struct {
	Modes map[string]ModeEligibility `json:"modes"`
}

// FetchModeEligibility calls a game server's optional per-player mode-eligibility endpoint.
// A missing endpoint (404) or unreachable server returns an empty map, not an error — callers
// treat a missing mode key as accessible: true (fail-open). Modes with a malformed requirement
// tree (unrecognized kind/operator) are dropped from the result for the same reason.
func (c *Client) FetchModeEligibility(ctx context.Context, apiBaseURL, lobbyUserID string) (map[string]ModeEligibility, error) {
	base := strings.TrimRight(strings.TrimSpace(apiBaseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("gameclient: api base URL is required")
	}
	if err := gameurl.ValidateOutboundURL(ctx, base, runtimeenv.IsProductionEnv()); err != nil {
		return nil, fmt.Errorf("gameclient: %w", err)
	}
	lobbyUserID = strings.TrimSpace(lobbyUserID)
	if lobbyUserID == "" {
		return nil, fmt.Errorf("gameclient: lobby user id is required")
	}

	url := fmt.Sprintf("%s/api/v1/players/%s/mode-eligibility", base, lobbyUserID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return map[string]ModeEligibility{}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return map[string]ModeEligibility{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return map[string]ModeEligibility{}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return map[string]ModeEligibility{}, nil
	}

	var batch modeEligibilityBatch
	if err := json.Unmarshal(body, &batch); err != nil {
		return map[string]ModeEligibility{}, nil
	}

	result := make(map[string]ModeEligibility, len(batch.Modes))
	for modeKey, elig := range batch.Modes {
		if elig.Requirement != nil && !elig.Requirement.valid() {
			continue
		}
		result[modeKey] = elig
	}
	return result, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd backend && go test ./internal/gameclient/ -run TestFetchModeEligibility -v
```
Expected: PASS (all 4 tests).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/gameclient/eligibility.go backend/internal/gameclient/eligibility_test.go
git commit -m "Add gameclient.FetchModeEligibility with fail-open handling"
```

---

### Task 3: gameclient — TTL cache

**Files:**
- Create: `backend/internal/gameclient/eligibility_cache.go`
- Test: `backend/internal/gameclient/eligibility_cache_test.go`

**Interfaces:**
- Consumes: `func (c *Client) FetchModeEligibility(...)` from Task 2.
- Produces: `func NewEligibilityCache(client *Client, ttl time.Duration) *EligibilityCache`, `func (c *EligibilityCache) Get(ctx context.Context, gameID, apiBaseURL, lobbyUserID string) (map[string]ModeEligibility, error)`.

- [ ] **Step 1: Write the failing test**

Create `backend/internal/gameclient/eligibility_cache_test.go`:

```go
package gameclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestEligibilityCacheHitsWithinTTL(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"modes": {"legendary": {"accessible": true}}}`))
	}))
	defer srv.Close()

	cache := NewEligibilityCache(NewClient(), 50*time.Millisecond)

	if _, err := cache.Get(context.Background(), "game-1", srv.URL, "player-1"); err != nil {
		t.Fatalf("first Get failed: %v", err)
	}
	if _, err := cache.Get(context.Background(), "game-1", srv.URL, "player-1"); err != nil {
		t.Fatalf("second Get failed: %v", err)
	}
	if got := atomic.LoadInt32(&callCount); got != 1 {
		t.Fatalf("expected 1 upstream call (cache hit on second Get), got %d", got)
	}
}

func TestEligibilityCacheExpiresAfterTTL(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"modes": {}}`))
	}))
	defer srv.Close()

	cache := NewEligibilityCache(NewClient(), 10*time.Millisecond)

	if _, err := cache.Get(context.Background(), "game-1", srv.URL, "player-1"); err != nil {
		t.Fatalf("first Get failed: %v", err)
	}
	time.Sleep(30 * time.Millisecond)
	if _, err := cache.Get(context.Background(), "game-1", srv.URL, "player-1"); err != nil {
		t.Fatalf("second Get failed: %v", err)
	}
	if got := atomic.LoadInt32(&callCount); got != 2 {
		t.Fatalf("expected 2 upstream calls (cache expired), got %d", got)
	}
}

func TestEligibilityCacheKeysByGameAndPlayer(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"modes": {}}`))
	}))
	defer srv.Close()

	cache := NewEligibilityCache(NewClient(), time.Second)

	if _, err := cache.Get(context.Background(), "game-1", srv.URL, "player-1"); err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if _, err := cache.Get(context.Background(), "game-1", srv.URL, "player-2"); err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got := atomic.LoadInt32(&callCount); got != 2 {
		t.Fatalf("expected 2 upstream calls (different players), got %d", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd backend && go test ./internal/gameclient/ -run TestEligibilityCache -v
```
Expected: FAIL with `NewEligibilityCache undefined` (compile error).

- [ ] **Step 3: Implement**

Create `backend/internal/gameclient/eligibility_cache.go`:

```go
package gameclient

import (
	"context"
	"sync"
	"time"
)

// EligibilityCache caches FetchModeEligibility results briefly per (gameID, lobbyUserID).
// It is never a source of truth — just enough to absorb rapid re-renders/polling without
// hammering the game server on every panel load.
type EligibilityCache struct {
	client *Client
	ttl    time.Duration

	mu    sync.Mutex
	items map[string]map[string]ModeEligibility
}

func NewEligibilityCache(client *Client, ttl time.Duration) *EligibilityCache {
	return &EligibilityCache{
		client: client,
		ttl:    ttl,
		items:  make(map[string]map[string]ModeEligibility),
	}
}

func eligibilityCacheKey(gameID, lobbyUserID string) string {
	return gameID + "|" + lobbyUserID
}

// Get returns cached eligibility for (gameID, lobbyUserID), fetching from apiBaseURL on a miss.
func (c *EligibilityCache) Get(ctx context.Context, gameID, apiBaseURL, lobbyUserID string) (map[string]ModeEligibility, error) {
	key := eligibilityCacheKey(gameID, lobbyUserID)

	c.mu.Lock()
	if cached, ok := c.items[key]; ok {
		c.mu.Unlock()
		return cached, nil
	}
	c.mu.Unlock()

	fetched, err := c.client.FetchModeEligibility(ctx, apiBaseURL, lobbyUserID)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.items[key] = fetched
	c.mu.Unlock()
	time.AfterFunc(c.ttl, func() {
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
	})

	return fetched, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd backend && go test ./internal/gameclient/ -run TestEligibilityCache -v
```
Expected: PASS (all 3 tests).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/gameclient/eligibility_cache.go backend/internal/gameclient/eligibility_cache_test.go
git commit -m "Add EligibilityCache TTL cache for gameclient"
```

---

### Task 4: Adapter — gameclient types to GraphQL models

**Files:**
- Create: `backend/graph/catalog_eligibility_internal.go`
- Test: `backend/graph/catalog_eligibility_internal_test.go`

**Interfaces:**
- Consumes: `gameclient.RequirementNode`, `gameclient.ModeEligibility` (Task 2); generated `model.ModeEligibility`, `model.ModeRequirementNode`, `model.RequirementLeaf`, `model.RequirementGroup`, `model.RequirementOperatorAll`, `model.RequirementOperatorAny` (Task 1 — confirm exact generated enum constant names by inspecting `backend/graph/model/models_gen.go` after Task 1's codegen; gqlgen's standard convention title-cases enum values, so `ALL`/`ANY` become `RequirementOperatorAll`/`RequirementOperatorAny`).
- Produces: `func toGraphQLModeEligibility(e gameclient.ModeEligibility) *model.ModeEligibility`, used by Task 7's resolver.

- [ ] **Step 1: Write the failing test**

Create `backend/graph/catalog_eligibility_internal_test.go`:

```go
package graph

import (
	"testing"

	"github.com/scruffyprodigy/playhub/graph/model"
	"github.com/scruffyprodigy/playhub/internal/gameclient"
)

func TestToGraphQLModeEligibilityBooleanGate(t *testing.T) {
	in := gameclient.ModeEligibility{Accessible: false, Reason: "Complete the tutorial to unlock."}
	out := toGraphQLModeEligibility(in)
	if out.Accessible {
		t.Fatal("expected inaccessible")
	}
	if out.Reason == nil || *out.Reason != "Complete the tutorial to unlock." {
		t.Fatalf("unexpected reason: %+v", out.Reason)
	}
	if out.Requirement != nil {
		t.Fatalf("expected nil requirement for boolean gate, got %+v", out.Requirement)
	}
}

func TestToGraphQLModeEligibilityLeaf(t *testing.T) {
	in := gameclient.ModeEligibility{
		Accessible: false,
		Requirement: &gameclient.RequirementNode{
			Kind: "leaf", Label: "Ranked matches", Current: 12, Target: 50,
		},
	}
	out := toGraphQLModeEligibility(in)
	leaf, ok := out.Requirement.(*model.RequirementLeaf)
	if !ok {
		t.Fatalf("expected *model.RequirementLeaf, got %T", out.Requirement)
	}
	if leaf.Current != 12 || leaf.Target != 50 {
		t.Fatalf("unexpected leaf values: %+v", leaf)
	}
}

func TestToGraphQLModeEligibilityGroup(t *testing.T) {
	in := gameclient.ModeEligibility{
		Accessible: false,
		Requirement: &gameclient.RequirementNode{
			Kind: "group", Label: "Commander requirements", Operator: "all",
			Children: []gameclient.RequirementNode{
				{Kind: "leaf", Label: "Standard wins", Current: 18, Target: 25},
				{Kind: "leaf", Label: "Unique decks used", Current: 3, Target: 5},
			},
		},
	}
	out := toGraphQLModeEligibility(in)
	group, ok := out.Requirement.(*model.RequirementGroup)
	if !ok {
		t.Fatalf("expected *model.RequirementGroup, got %T", out.Requirement)
	}
	if group.Operator != model.RequirementOperatorAll {
		t.Fatalf("expected ALL operator, got %v", group.Operator)
	}
	if len(group.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(group.Children))
	}
	firstLeaf, ok := group.Children[0].(*model.RequirementLeaf)
	if !ok || firstLeaf.Current != 18 {
		t.Fatalf("unexpected first child: %+v", group.Children[0])
	}
}

func TestToGraphQLModeEligibilityUnlockModeKey(t *testing.T) {
	key := "deck-builder"
	in := gameclient.ModeEligibility{Accessible: false, UnlockModeKey: &key}
	out := toGraphQLModeEligibility(in)
	if out.UnlockModeKey == nil || *out.UnlockModeKey != "deck-builder" {
		t.Fatalf("expected unlockModeKey to pass through, got %+v", out.UnlockModeKey)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd backend && go test ./graph/ -run TestToGraphQLModeEligibility -v
```
Expected: FAIL with `toGraphQLModeEligibility undefined` (compile error).

- [ ] **Step 3: Implement**

Create `backend/graph/catalog_eligibility_internal.go`:

```go
package graph

import (
	"strings"

	"github.com/scruffyprodigy/playhub/graph/model"
	"github.com/scruffyprodigy/playhub/internal/gameclient"
)

func toGraphQLRequirementNode(n *gameclient.RequirementNode) model.ModeRequirementNode {
	if n == nil {
		return nil
	}
	if n.Kind == "group" {
		children := make([]model.ModeRequirementNode, 0, len(n.Children))
		for i := range n.Children {
			if child := toGraphQLRequirementNode(&n.Children[i]); child != nil {
				children = append(children, child)
			}
		}
		operator := model.RequirementOperatorAll
		if strings.EqualFold(n.Operator, "any") {
			operator = model.RequirementOperatorAny
		}
		return &model.RequirementGroup{
			Label:    n.Label,
			Operator: operator,
			Children: children,
		}
	}
	return &model.RequirementLeaf{
		Label:   n.Label,
		Current: n.Current,
		Target:  n.Target,
	}
}

func toGraphQLModeEligibility(e gameclient.ModeEligibility) *model.ModeEligibility {
	out := &model.ModeEligibility{
		Accessible:    e.Accessible,
		Requirement:   toGraphQLRequirementNode(e.Requirement),
		UnlockModeKey: e.UnlockModeKey,
	}
	if e.Reason != "" {
		reason := e.Reason
		out.Reason = &reason
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd backend && go test ./graph/ -run TestToGraphQLModeEligibility -v
```
Expected: PASS (all 4 tests). If the enum constant names differ from `model.RequirementOperatorAll`/`model.RequirementOperatorAny`, check `backend/graph/model/models_gen.go` for the actual generated names and fix both this file and the test.

- [ ] **Step 5: Commit**

```bash
git add backend/graph/catalog_eligibility_internal.go backend/graph/catalog_eligibility_internal_test.go
git commit -m "Add adapter from gameclient eligibility types to GraphQL models"
```

---

### Task 5: Migration — fixture game and modes

**Files:**
- Create: `backend/migrations/000038_fixture_eligibility_game.up.sql`
- Create: `backend/migrations/000038_fixture_eligibility_game.down.sql`

**Interfaces:**
- Produces: a `games` row (id `b1000000-0000-4000-8000-000000000001`) with `api_base_url` pointing at `http://localhost:9400` (the fixture binary from Task 8), and five `game_modes` rows demonstrating each gate — consumed by Task 7's integration test and Task 8's manual verification.

- [ ] **Step 1: Write the up migration**

Create `backend/migrations/000038_fixture_eligibility_game.up.sql`:

```sql
-- Fixture game for mode-level eligibility (JQ-11/12/13/16): demonstrates all four gate
-- shapes (boolean, single-counter, boolean-as-leaf, compound AND) against a local
-- fixture HTTP server (backend/cmd/fixturegame), since no real reference game implements
-- the mode-eligibility contract yet.

INSERT INTO games (
    id, name, slug, api_base_url, icon_url, hero_url, short_description,
    category, status, visibility, tags
)
VALUES (
    'b1000000-0000-4000-8000-000000000001',
    'Eligibility Fixture',
    'eligibility-fixture',
    'http://localhost:9400',
    '/games/eligibility-fixture/icon.png',
    '/games/eligibility-fixture/hero.jpg',
    'Local fixture game demonstrating mode-level eligibility gates.',
    'catalog',
    'active',
    'public',
    '{}'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO game_modes (id, game_id, mode_key, display_name, min_players, max_players, status)
VALUES
    ('b2000000-0000-4000-8000-000000000001', 'b1000000-0000-4000-8000-000000000001', 'arena', 'Arena', 2, 2, 'active'),
    ('b2000000-0000-4000-8000-000000000002', 'b1000000-0000-4000-8000-000000000001', 'legendary', 'Legendary', 2, 2, 'active'),
    ('b2000000-0000-4000-8000-000000000003', 'b1000000-0000-4000-8000-000000000001', 'standard', 'Standard', 2, 2, 'active'),
    ('b2000000-0000-4000-8000-000000000004', 'b1000000-0000-4000-8000-000000000001', 'commander', 'Commander', 2, 2, 'active'),
    ('b2000000-0000-4000-8000-000000000005', 'b1000000-0000-4000-8000-000000000001', 'deck-builder', 'Deck Builder', 1, 1, 'active')
ON CONFLICT (game_id, mode_key) DO NOTHING;

INSERT INTO game_mode_seats (mode_id, seat_key, sort_order)
VALUES
    ('b2000000-0000-4000-8000-000000000001', '1', 0),
    ('b2000000-0000-4000-8000-000000000001', '2', 1),
    ('b2000000-0000-4000-8000-000000000002', '1', 0),
    ('b2000000-0000-4000-8000-000000000002', '2', 1),
    ('b2000000-0000-4000-8000-000000000003', '1', 0),
    ('b2000000-0000-4000-8000-000000000003', '2', 1),
    ('b2000000-0000-4000-8000-000000000004', '1', 0),
    ('b2000000-0000-4000-8000-000000000004', '2', 1),
    ('b2000000-0000-4000-8000-000000000005', '1', 0)
ON CONFLICT (mode_id, seat_key) DO NOTHING;
```

- [ ] **Step 2: Write the down migration**

Create `backend/migrations/000038_fixture_eligibility_game.down.sql`:

```sql
DELETE FROM game_mode_seats WHERE mode_id IN (
    'b2000000-0000-4000-8000-000000000001',
    'b2000000-0000-4000-8000-000000000002',
    'b2000000-0000-4000-8000-000000000003',
    'b2000000-0000-4000-8000-000000000004',
    'b2000000-0000-4000-8000-000000000005'
);
DELETE FROM game_modes WHERE game_id = 'b1000000-0000-4000-8000-000000000001';
DELETE FROM games WHERE id = 'b1000000-0000-4000-8000-000000000001';
```

- [ ] **Step 3: Run the migration against the local test database and verify**

```bash
cd backend && make migrate-up
psql "$DATABASE_URL" -c "SELECT slug, api_base_url FROM games WHERE id = 'b1000000-0000-4000-8000-000000000001';"
psql "$DATABASE_URL" -c "SELECT mode_key FROM game_modes WHERE game_id = 'b1000000-0000-4000-8000-000000000001' ORDER BY mode_key;"
```
Expected: one game row, five mode rows (`arena`, `commander`, `deck-builder`, `legendary`, `standard`).

- [ ] **Step 4: Verify the down migration is reversible**

```bash
cd backend && make migrate-down
psql "$DATABASE_URL" -c "SELECT COUNT(*) FROM games WHERE id = 'b1000000-0000-4000-8000-000000000001';"
```
Expected: `0`. Then re-run `make migrate-up` to leave the DB in the migrated state for later tasks.

- [ ] **Step 5: Commit**

```bash
git add backend/migrations/000038_fixture_eligibility_game.up.sql backend/migrations/000038_fixture_eligibility_game.down.sql
git commit -m "Add fixture eligibility game and modes migration"
```

---

### Task 6: gameclient/testutil — fixture eligibility HTTP handler

**Files:**
- Create: `backend/internal/gameclient/testutil/eligibility_fixture.go`

**Interfaces:**
- Produces: `func EligibilityFixtureHandler() http.Handler` — a handler serving `GET /api/v1/players/{lobbyUserId}/mode-eligibility`, with canned responses selected by `lobbyUserId` suffix (`-locked` / `-unlocked`), covering all 4 gates from Task 5's fixture modes. Consumed by Task 7's integration test and Task 8's standalone binary.

- [ ] **Step 1: Implement**

Create `backend/internal/gameclient/testutil/eligibility_fixture.go`:

```go
// Package testutil provides fixture game-server handlers for local dev and integration tests.
package testutil

import (
	"encoding/json"
	"net/http"
	"strings"
)

// EligibilityFixtureHandler serves GET /api/v1/players/{lobbyUserId}/mode-eligibility with
// canned data for the fixture game seeded by migration 000038. The response depends on
// whether lobbyUserId ends in "-locked" or "-unlocked" (defaulting to locked), so tests and
// local dev can exercise both states without mutating real stats.
func EligibilityFixtureHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/players/") || !strings.HasSuffix(r.URL.Path, "/mode-eligibility") {
			http.NotFound(w, r)
			return
		}

		unlocked := strings.Contains(r.URL.Path, "-unlocked")

		deckBuilder := "deck-builder"
		modes := map[string]any{
			"arena": map[string]any{
				"accessible": unlocked,
				"reason":     reasonUnless(unlocked, "Complete the tutorial to unlock."),
			},
			"legendary": map[string]any{
				"accessible": unlocked,
				"reason":     reasonUnless(unlocked, "Complete 50 Ranked matches to unlock."),
				"requirement": map[string]any{
					"kind": "leaf", "label": "Ranked matches",
					"current": currentUnless(unlocked, 12, 50), "target": 50,
				},
			},
			"standard": map[string]any{
				"accessible":    unlocked,
				"reason":        reasonUnless(unlocked, "You have no Standard-legal decks."),
				"unlockModeKey": unlockKeyUnless(unlocked, &deckBuilder),
				"requirement": map[string]any{
					"kind": "leaf", "label": "Standard-legal decks",
					"current": currentUnless(unlocked, 0, 1), "target": 1,
				},
			},
			"commander": map[string]any{
				"accessible": unlocked,
				"reason":     reasonUnless(unlocked, "Requires 25 Standard wins and 5 unique decks used."),
				"requirement": map[string]any{
					"kind": "group", "label": "Commander requirements", "operator": "all",
					"children": []map[string]any{
						{"kind": "leaf", "label": "Standard wins", "current": currentUnless(unlocked, 18, 25), "target": 25},
						{"kind": "leaf", "label": "Unique decks used", "current": currentUnless(unlocked, 3, 5), "target": 5},
					},
				},
			},
			"deck-builder": map[string]any{
				"accessible": true,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"modes": modes})
	})
}

func reasonUnless(unlocked bool, reason string) any {
	if unlocked {
		return nil
	}
	return reason
}

func currentUnless(unlocked bool, lockedValue, unlockedValue int) int {
	if unlocked {
		return unlockedValue
	}
	return lockedValue
}

func unlockKeyUnless(unlocked bool, key *string) any {
	if unlocked {
		return nil
	}
	return key
}
```

- [ ] **Step 2: Verify it builds**

```bash
cd backend && go build ./internal/gameclient/testutil/
```
Expected: builds cleanly. (No standalone test for this file — it's exercised by Task 7's integration test.)

- [ ] **Step 3: Commit**

```bash
git add backend/internal/gameclient/testutil/eligibility_fixture.go
git commit -m "Add fixture eligibility HTTP handler for tests and local dev"
```

---

### Task 7: Resolver wiring + integration test

**Files:**
- Modify: `backend/graph/resolver.go`
- Create: `backend/graph/catalog_eligibility.go`
- Modify: `backend/graph/catalog.resolvers.go` (implement the `Eligibility` stub from Task 1)
- Test: `backend/graph/catalog_eligibility_integration_test.go`

**Interfaces:**
- Consumes: `gameclient.NewClient()`, `gameclient.NewEligibilityCache(client, ttl)`, `(*EligibilityCache).Get(...)` (Task 3); `toGraphQLModeEligibility` (Task 4); `testutil.EligibilityFixtureHandler()` (Task 6); fixture game/modes from migration 000038 (Task 5); `st.GetGameModeByID(ctx, modeID uuid.UUID) (*store.GameMode, error)` and `st.GetGameByID(ctx, id uuid.UUID) (*store.Game, error)` (existing store methods); `requireAuthUserID(ctx)` and `parseUUID(id, label string)` (existing `backend/graph/resolver.go` helpers).
- Produces: `func (r *gameModeResolver) Eligibility(ctx context.Context, obj *model.GameMode, playerID string) (*model.ModeEligibility, error)` — the field consumed by the frontend in Task 9.

- [ ] **Step 1: Write the failing integration test**

Create `backend/graph/catalog_eligibility_integration_test.go`. This reuses the existing `queueIntegrationEnv` test harness (`graph/queue_integration_test.go`'s `newQueueIntegrationEnv(t)`, `env.newCleaner(t)`, and `createTestUserSessionForUser(t, env, userID)` + `client.AddCookie(cookie)` for authenticating the gqlgen test client as a specific user — the same pattern `table_integration_test.go` uses), pointing the fixture game's `api_base_url` at a local `httptest.Server` running `testutil.EligibilityFixtureHandler()` for the duration of the test (overriding the DB-seeded `http://localhost:9400` so the test doesn't depend on Task 8's standalone binary being run):

```go
package graph

import (
	"net/http/httptest"
	"testing"

	"github.com/99designs/gqlgen/client"
	"github.com/scruffyprodigy/playhub/internal/gameclient/testutil"
	"github.com/scruffyprodigy/playhub/internal/store"
)

const fixtureGameID = "b1000000-0000-4000-8000-000000000001"

func pointFixtureGameAt(t *testing.T, env *queueIntegrationEnv, apiBaseURL string) {
	t.Helper()
	if _, err := env.DB.Exec(`UPDATE games SET api_base_url = $1 WHERE id = $2`, apiBaseURL, fixtureGameID); err != nil {
		t.Fatalf("point fixture game at test server: %v", err)
	}
	t.Cleanup(func() {
		_, _ = env.DB.Exec(`UPDATE games SET api_base_url = 'http://localhost:9400' WHERE id = $1`, fixtureGameID)
	})
}

func TestGameModeEligibilityLockedLeaf(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	ctx := t.Context()
	cleaner := env.newCleaner(t)

	srv := httptest.NewServer(testutil.EligibilityFixtureHandler())
	defer srv.Close()
	pointFixtureGameAt(t, env, srv.URL)

	user, err := env.Store.CreateUser(ctx, store.CreateUserParams{Email: "player-legendary-locked@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(user.ID)
	_, cookie := createTestUserSessionForUser(t, env, user.ID)

	var resp struct {
		Game struct {
			Modes []struct {
				ModeKey     string `json:"modeKey"`
				Eligibility struct {
					Accessible bool    `json:"accessible"`
					Reason     *string `json:"reason"`
				} `json:"eligibility"`
			} `json:"modes"`
		} `json:"game"`
	}
	err = env.Client.Post(`
		query($gameId: ID!, $playerId: ID!) {
			game(id: $gameId) {
				modes {
					modeKey
					eligibility(playerId: $playerId) { accessible reason }
				}
			}
		}
	`, &resp, client.AddCookie(cookie), client.Var("gameId", fixtureGameID), client.Var("playerId", user.ID.String()))
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	found := false
	for _, mode := range resp.Game.Modes {
		if mode.ModeKey == "legendary" {
			found = true
			if mode.Eligibility.Accessible {
				t.Fatal("expected legendary to be locked")
			}
			if mode.Eligibility.Reason == nil || *mode.Eligibility.Reason != "Complete 50 Ranked matches to unlock." {
				t.Fatalf("unexpected reason: %+v", mode.Eligibility.Reason)
			}
		}
	}
	if !found {
		t.Fatal("legendary mode not found in response")
	}
}

func TestGameModeEligibilityCompoundGroupUnlocked(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	ctx := t.Context()
	cleaner := env.newCleaner(t)

	srv := httptest.NewServer(testutil.EligibilityFixtureHandler())
	defer srv.Close()
	pointFixtureGameAt(t, env, srv.URL)

	user, err := env.Store.CreateUser(ctx, store.CreateUserParams{Email: "player-commander-unlocked@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(user.ID)
	_, cookie := createTestUserSessionForUser(t, env, user.ID)

	var resp struct {
		Game struct {
			Modes []struct {
				ModeKey     string `json:"modeKey"`
				Eligibility struct {
					Accessible  bool `json:"accessible"`
					Requirement struct {
						Typename string `json:"__typename"`
						Operator string `json:"operator"`
						Children []struct {
							Current int `json:"current"`
							Target  int `json:"target"`
						} `json:"children"`
					} `json:"requirement"`
				} `json:"eligibility"`
			} `json:"modes"`
		} `json:"game"`
	}
	err = env.Client.Post(`
		query($gameId: ID!, $playerId: ID!) {
			game(id: $gameId) {
				modes {
					modeKey
					eligibility(playerId: $playerId) {
						accessible
						requirement {
							__typename
							... on RequirementGroup {
								operator
								children { ... on RequirementLeaf { current target } }
							}
						}
					}
				}
			}
		}
	`, &resp, client.AddCookie(cookie), client.Var("gameId", fixtureGameID), client.Var("playerId", user.ID.String()))
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	for _, mode := range resp.Game.Modes {
		if mode.ModeKey != "commander" {
			continue
		}
		if !mode.Eligibility.Accessible {
			t.Fatal("expected commander to be unlocked for -unlocked player")
		}
		if mode.Eligibility.Requirement.Operator != "ALL" {
			t.Fatalf("expected ALL operator, got %q", mode.Eligibility.Requirement.Operator)
		}
		if len(mode.Eligibility.Requirement.Children) != 2 {
			t.Fatalf("expected 2 children, got %d", len(mode.Eligibility.Requirement.Children))
		}
	}
}

func TestGameModeEligibilityRejectsQueryingAnotherPlayer(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	ctx := t.Context()
	cleaner := env.newCleaner(t)

	srv := httptest.NewServer(testutil.EligibilityFixtureHandler())
	defer srv.Close()
	pointFixtureGameAt(t, env, srv.URL)

	user, err := env.Store.CreateUser(ctx, store.CreateUserParams{Email: "player-a@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(user.ID)
	other, err := env.Store.CreateUser(ctx, store.CreateUserParams{Email: "player-b@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(other.ID)
	_, cookie := createTestUserSessionForUser(t, env, user.ID)

	var resp struct{}
	err = env.Client.Post(`
		query($gameId: ID!, $playerId: ID!) {
			game(id: $gameId) { modes { eligibility(playerId: $playerId) { accessible } } }
		}
	`, &resp, client.AddCookie(cookie), client.Var("gameId", fixtureGameID), client.Var("playerId", other.ID.String()))
	if err == nil {
		t.Fatal("expected an authorization error when querying another player's eligibility")
	}
}
```

This file lives in `package graph` (not `graph_test`), matching every other integration test in this directory, so it can reach `queueIntegrationEnv`, `newQueueIntegrationEnv`, and `createTestUserSessionForUser` directly without exporting them.

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd backend && go test ./graph/... -run TestGameModeEligibility -v
```
Expected: FAIL — `gameModeResolver.Eligibility` returns `nil, nil` from the gqlgen-generated stub (Task 1), so `accessible` reads as `false` (zero value) instead of the expected fixture data, and the "reject another player" test fails because no authz check exists yet.

- [ ] **Step 3: Wire the EligibilityCache into Resolver**

In `backend/graph/resolver.go`, add a field to the `Resolver` struct (alongside the existing `ManifestFetcher` field):

```go
	// EligibilityCache resolves GameMode.eligibility; nil uses a default 5s in-memory cache.
	EligibilityCache *gameclient.EligibilityCache
```

Add a lazy-init helper, mirroring `manifestFetcher()` in `backend/graph/catalog_manifest.go`:

Create `backend/graph/catalog_eligibility.go`:

```go
package graph

import (
	"time"

	"github.com/scruffyprodigy/playhub/internal/gameclient"
)

func (r *Resolver) eligibilityCache() *gameclient.EligibilityCache {
	if r.EligibilityCache != nil {
		return r.EligibilityCache
	}
	return gameclient.NewEligibilityCache(gameclient.NewClient(), 5*time.Second)
}
```

- [ ] **Step 4: Implement the resolver method**

In `backend/graph/catalog.resolvers.go`, replace the gqlgen-generated `Eligibility` stub (added by Task 1's codegen) with:

```go
// Eligibility is the resolver for the eligibility field.
func (r *gameModeResolver) Eligibility(ctx context.Context, obj *model.GameMode, playerID string) (*model.ModeEligibility, error) {
	st, err := r.requireStore()
	if err != nil {
		return nil, err
	}

	authUserID, err := requireAuthUserID(ctx)
	if err != nil {
		return nil, err
	}
	requestedID, err := parseUUID(playerID, "player id")
	if err != nil {
		return nil, err
	}
	if requestedID != authUserID {
		return nil, fmt.Errorf("cannot query eligibility for another player")
	}

	modeID, err := parseUUID(obj.ID, "mode id")
	if err != nil {
		return nil, err
	}
	mode, err := st.GetGameModeByID(ctx, modeID)
	if err != nil {
		return nil, err
	}
	game, err := st.GetGameByID(ctx, mode.GameID)
	if err != nil {
		return nil, err
	}
	if game.APIBaseURL == nil || *game.APIBaseURL == "" {
		return &model.ModeEligibility{Accessible: true}, nil
	}

	batch, err := r.eligibilityCache().Get(ctx, game.ID.String(), *game.APIBaseURL, playerID)
	if err != nil {
		// Fail open: a game server we can't reach doesn't block play.
		return &model.ModeEligibility{Accessible: true}, nil
	}

	elig, ok := batch[mode.ModeKey]
	if !ok {
		return &model.ModeEligibility{Accessible: true}, nil
	}
	return toGraphQLModeEligibility(elig), nil
}
```

Verify `fmt` is imported in `catalog.resolvers.go` (it's used elsewhere in the file already, per existing resolvers — confirm and add the import if gqlgen's stub generation didn't already include it).

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd backend && go build ./... && go test ./graph/... -run TestGameModeEligibility -v
```
Expected: PASS (all 3 tests).

- [ ] **Step 6: Run the full backend test suite to check for regressions**

```bash
cd backend && go test ./...
```
Expected: all existing tests still pass.

- [ ] **Step 7: Commit**

```bash
git add backend/graph/resolver.go backend/graph/catalog_eligibility.go backend/graph/catalog.resolvers.go backend/graph/catalog_eligibility_integration_test.go
git commit -m "Implement GameMode.eligibility resolver with fail-open and self-only authz"
```

---

### Task 8: Standalone fixture binary

**Files:**
- Create: `backend/cmd/fixturegame/main.go`

**Interfaces:**
- Consumes: `testutil.EligibilityFixtureHandler()` (Task 6).
- Produces: a runnable binary listening on port 9400 (matching the `api_base_url` seeded in Task 5's migration), for manual/visual frontend verification in Task 13.

- [ ] **Step 1: Implement**

Create `backend/cmd/fixturegame/main.go`:

```go
// Standalone fixture game server for local dev: serves the mode-eligibility endpoint the
// "Eligibility Fixture" catalog game (migration 000038) points at, so the frontend locked/
// unlocked UI can be visually verified without a real third-party game integration.
// Usage: go run ./cmd/fixturegame
package main

import (
	"log"
	"net/http"

	"github.com/scruffyprodigy/playhub/internal/gameclient/testutil"
)

func main() {
	addr := ":9400"
	log.Printf("fixturegame listening on %s (mode-eligibility endpoint)", addr)
	if err := http.ListenAndServe(addr, testutil.EligibilityFixtureHandler()); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 2: Verify it runs and responds**

```bash
cd backend && go run ./cmd/fixturegame &
sleep 1
curl -s http://localhost:9400/api/v1/players/test-legendary-locked/mode-eligibility | head -c 300
kill %1
```
Expected: JSON response containing `"legendary"` with `"accessible":false`.

- [ ] **Step 3: Commit**

```bash
git add backend/cmd/fixturegame/main.go
git commit -m "Add standalone fixturegame binary for local eligibility UI verification"
```

---

### Task 9: Frontend — thread playerId through the modes query

**Files:**
- Modify: `frontend/src/lib/games.js`

**Interfaces:**
- Consumes: `graphqlRequest(query, variables)` from `frontend/src/lib/graphql.js` (existing).
- Produces: `fetchGames(playerId)`, `fetchGameBySlug(slug, playerId)` — both accept `playerId` as `''` when no user is signed in (paired with `hasPlayer: Boolean(playerId)`), consumed by Task 10's callers and rendering `mode.eligibility` for Task 12's `ModeRow`.

- [ ] **Step 1: Add the eligibility fragment and thread query variables**

In `frontend/src/lib/games.js`, modify `GAME_MODE_FIELDS` and both query constants:

```js
const GAME_MODE_FIELDS = `
  modes {
    id
    modeKey
    displayName
    status
    queuePaths {
      queuePath
      displayName
      playersToStart
    }
    seats {
      queuePath
    }
    queues {
      id
      name
      playersToStart
      status
    }
    eligibility(playerId: $playerId) @include(if: $hasPlayer) {
      accessible
      reason
      unlockModeKey
      requirement {
        __typename
        label
        ... on RequirementLeaf {
          current
          target
        }
        ... on RequirementGroup {
          operator
          children {
            __typename
            label
            ... on RequirementLeaf {
              current
              target
            }
          }
        }
      }
    }
  }
`

const GAME_CARD_FIELDS = `
  id
  slug
  name
  iconUrl
  heroUrl
  catalogHeroUrl
  shortDescription
  longDescription
  howToPlay
  tutorialUrl
  screenshots
  tags
  createdAt
  ${GAME_MODE_FIELDS}
`

const GAMES_QUERY = `
  query Games($playerId: ID!, $hasPlayer: Boolean!) {
    games {
      ${GAME_CARD_FIELDS}
    }
  }
`

const GAME_BY_SLUG_QUERY = `
  query GameBySlug($slug: String!, $playerId: ID!, $hasPlayer: Boolean!) {
    gameBySlug(slug: $slug) {
      ${GAME_CARD_FIELDS}
    }
  }
`
```

- [ ] **Step 2: Update `fetchGames` and `fetchGameBySlug` signatures**

Replace the two functions at the bottom of `frontend/src/lib/games.js`:

```js
export async function fetchGames(playerId = '') {
  const data = await graphqlRequest(GAMES_QUERY, {
    playerId,
    hasPlayer: Boolean(playerId),
  })
  return data.games ?? []
}

export async function fetchGameBySlug(slug, playerId = '') {
  const trimmed = String(slug || '').trim()
  if (!trimmed) {
    return null
  }
  const data = await graphqlRequest(GAME_BY_SLUG_QUERY, {
    slug: trimmed,
    playerId,
    hasPlayer: Boolean(playerId),
  })
  return data.gameBySlug ?? null
}
```

- [ ] **Step 3: Verify existing frontend tests still pass**

```bash
cd frontend && npm run test:run
```
Expected: all existing tests pass (no test currently exercises `fetchGames`/`fetchGameBySlug` directly per the earlier codebase survey, so this should be a clean pass — if any test does mock/assert on these functions' call signatures, update the mock to match the new signature).

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/games.js
git commit -m "Thread playerId through games queries for mode eligibility"
```

---

### Task 10: Frontend — update callers

**Files:**
- Modify: `frontend/src/components/games/GameLobby.jsx`
- Modify: `frontend/src/components/games/GameDetailPage.jsx`

**Interfaces:**
- Consumes: `fetchGames(playerId)`, `fetchGameBySlug(slug, playerId)` (Task 9); `useAuth()` returning `{ user, loading }` (existing `AuthProvider.jsx`).

- [ ] **Step 1: Update `GameLobby.jsx`**

In `frontend/src/components/games/GameLobby.jsx`, `GameLobby` already gates its effect on `authLoading || !user`, so `user.id` is guaranteed present. Change:

```js
    fetchGames()
```
to:
```js
    fetchGames(user.id)
```
and add `user.id` to the effect's dependency array (currently `[authLoading, user]` — `user` already covers this, no change needed there since `user` object identity changing implies `user.id` may have changed too, but add it explicitly for clarity):

```js
  }, [authLoading, user])
```
(No change needed to the dependency array — `user` is already a dependency and `user.id` is derived from it.)

- [ ] **Step 2: Update `GameDetailPage.jsx`**

`GameDetailPage` does **not** gate its fetch on `user` being present (game detail pages are viewable while signed out). Change the effect to depend on `user?.id` and pass it through:

```js
    fetchGameBySlug(slug, user?.id ?? '')
```
And add `user?.id` to whatever dependency array wraps this effect (find the existing `useEffect` dependency array in this file — likely `[slug]` — and change it to `[slug, user?.id]` so the query re-runs with real eligibility data once the user finishes loading/signing in).

- [ ] **Step 3: Manually verify no regression in signed-out game detail view**

```bash
cd frontend && npm run dev
```
Open a game detail page in an incognito/signed-out browser session; confirm the page still loads and modes render without an `eligibility` section (since `hasPlayer` is `false`).

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/games/GameLobby.jsx frontend/src/components/games/GameDetailPage.jsx
git commit -m "Pass current player id into games queries for eligibility"
```

---

### Task 11: Frontend — `ModeRequirement` component

**Files:**
- Create: `frontend/src/components/games/ModeRequirement.jsx`
- Test: `frontend/src/components/games/ModeRequirement.test.jsx`

**Interfaces:**
- Consumes: a `requirement` prop shaped like the GraphQL fragment from Task 9 (`{__typename, label, current?, target?, operator?, children?}`, or `null`).
- Produces: `export default function ModeRequirement({ requirement })`, consumed by Task 12's `ModeRow`.

- [ ] **Step 1: Write the failing tests**

Create `frontend/src/components/games/ModeRequirement.test.jsx`:

```jsx
import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import ModeRequirement from './ModeRequirement'

describe('ModeRequirement', () => {
  it('renders nothing for a null requirement', () => {
    const { container } = render(<ModeRequirement requirement={null} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('renders a leaf as current/target label', () => {
    render(
      <ModeRequirement
        requirement={{ __typename: 'RequirementLeaf', label: 'Ranked matches', current: 12, target: 50 }}
      />,
    )
    expect(screen.getByText('12/50 Ranked matches')).toBeInTheDocument()
  })

  it('renders an ALL group joined by "and"', () => {
    render(
      <ModeRequirement
        requirement={{
          __typename: 'RequirementGroup',
          label: 'Commander requirements',
          operator: 'ALL',
          children: [
            { __typename: 'RequirementLeaf', label: 'Standard wins', current: 18, target: 25 },
            { __typename: 'RequirementLeaf', label: 'Unique decks used', current: 3, target: 5 },
          ],
        }}
      />,
    )
    expect(screen.getByText('18/25 Standard wins')).toBeInTheDocument()
    expect(screen.getByText('and')).toBeInTheDocument()
    expect(screen.getByText('3/5 Unique decks used')).toBeInTheDocument()
  })

  it('renders an ANY group joined by "or"', () => {
    render(
      <ModeRequirement
        requirement={{
          __typename: 'RequirementGroup',
          label: 'x',
          operator: 'ANY',
          children: [
            { __typename: 'RequirementLeaf', label: 'A', current: 1, target: 2 },
            { __typename: 'RequirementLeaf', label: 'B', current: 3, target: 4 },
          ],
        }}
      />,
    )
    expect(screen.getByText('or')).toBeInTheDocument()
  })
})
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd frontend && npx vitest run src/components/games/ModeRequirement.test.jsx
```
Expected: FAIL — module `./ModeRequirement` does not exist.

- [ ] **Step 3: Implement**

Create `frontend/src/components/games/ModeRequirement.jsx`:

```jsx
export default function ModeRequirement({ requirement }) {
  if (!requirement) {
    return null
  }

  if (requirement.__typename === 'RequirementGroup') {
    const joiner = requirement.operator === 'ANY' ? 'or' : 'and'
    return (
      <span className="mode-requirement mode-requirement--group">
        {requirement.children.map((child, index) => (
          <span key={`${child.label}-${index}`}>
            {index > 0 ? <span className="mode-requirement__joiner"> {joiner} </span> : null}
            <ModeRequirement requirement={child} />
          </span>
        ))}
      </span>
    )
  }

  return (
    <span className="mode-requirement mode-requirement--leaf">
      {requirement.current}/{requirement.target} {requirement.label}
    </span>
  )
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd frontend && npx vitest run src/components/games/ModeRequirement.test.jsx
```
Expected: PASS (all 4 tests).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/games/ModeRequirement.jsx frontend/src/components/games/ModeRequirement.test.jsx
git commit -m "Add ModeRequirement component for recursive leaf/group rendering"
```

---

### Task 12: Frontend — `ModeRow` locked state

**Files:**
- Modify: `frontend/src/components/games/GameModesPanel.jsx`
- Test: `frontend/src/components/games/GameModesPanel.test.jsx`

**Interfaces:**
- Consumes: `ModeRequirement` (Task 11); `mode.eligibility` shape from Task 9's query.
- Produces: locked-state rendering in `ModeRow`, gating the existing `GameQueueActions`/create-private-table controls behind `mode.eligibility?.accessible !== false`.

- [ ] **Step 1: Write the failing tests**

Create `frontend/src/components/games/GameModesPanel.test.jsx`:

```jsx
import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import GameModesPanel from './GameModesPanel'

function baseGame(mode) {
  return {
    id: 'game-1',
    modes: [{ id: 'mode-1', modeKey: 'legendary', displayName: 'Legendary', status: 'active', queues: [], seats: [], queuePaths: [], ...mode }],
  }
}

describe('GameModesPanel locked mode', () => {
  it('renders queue actions when accessible', () => {
    render(<GameModesPanel game={baseGame({ eligibility: { accessible: true } })} />)
    expect(screen.queryByText(/Complete 50 Ranked matches/)).not.toBeInTheDocument()
  })

  it('renders reason and progress when locked with a leaf requirement', () => {
    render(
      <GameModesPanel
        game={baseGame({
          eligibility: {
            accessible: false,
            reason: 'Complete 50 Ranked matches to unlock.',
            unlockModeKey: null,
            requirement: { __typename: 'RequirementLeaf', label: 'Ranked matches', current: 12, target: 50 },
          },
        })}
      />,
    )
    expect(screen.getByText('Complete 50 Ranked matches to unlock.')).toBeInTheDocument()
    expect(screen.getByText('12/50 Ranked matches')).toBeInTheDocument()
  })

  it('renders a boolean gate with no progress readout', () => {
    render(
      <GameModesPanel
        game={baseGame({
          eligibility: { accessible: false, reason: 'Complete the tutorial to unlock.', unlockModeKey: null, requirement: null },
        })}
      />,
    )
    expect(screen.getByText('Complete the tutorial to unlock.')).toBeInTheDocument()
  })

  it('routes to unlockModeKey when the locked reason is clicked', async () => {
    const onNavigateToMode = vi.fn()
    render(
      <GameModesPanel
        game={baseGame({
          modeKey: 'standard',
          eligibility: {
            accessible: false,
            reason: 'You have no Standard-legal decks.',
            unlockModeKey: 'deck-builder',
            requirement: { __typename: 'RequirementLeaf', label: 'Standard-legal decks', current: 0, target: 1 },
          },
        })}
        onNavigateToMode={onNavigateToMode}
      />,
    )
    await userEvent.click(screen.getByRole('button', { name: 'You have no Standard-legal decks.' }))
    expect(onNavigateToMode).toHaveBeenCalledWith('deck-builder')
  })
})
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd frontend && npx vitest run src/components/games/GameModesPanel.test.jsx
```
Expected: FAIL — locked-state text not found (component doesn't render it yet), and `onNavigateToMode` prop doesn't exist yet.

- [ ] **Step 3: Implement**

In `frontend/src/components/games/GameModesPanel.jsx`:

Add the import at the top:
```js
import ModeRequirement from './ModeRequirement'
```

Add `onNavigateToMode` to both component signatures (`ModeRow` and the default-exported `GameModesPanel`), and thread it through the `<ModeRow>` call site:

```js
function ModeRow({
  game,
  mode,
  activeIntent,
  activeTableSeat,
  onQueueChange,
  onQueueJoined,
  onTableChange,
  onNavigateToMode,
  prominent = false,
}) {
```

Insert a locked-state branch right after the existing `<div className="game-mode-row__copy">...</div>` block's closing tag, replacing the `<div className="game-mode-row__actions">` block conditionally. The simplest correct change is to wrap the existing actions `<div>` so it only renders when accessible, and render a locked variant otherwise:

```jsx
      <div className="game-mode-row__actions">
        {mode.eligibility?.accessible === false ? (
          <div className="game-mode-row__locked" role="status">
            <button
              type="button"
              className="game-mode-row__locked-reason"
              disabled={!mode.eligibility.unlockModeKey}
              onClick={() => {
                if (mode.eligibility.unlockModeKey) {
                  onNavigateToMode?.(mode.eligibility.unlockModeKey)
                }
              }}
            >
              {mode.eligibility.reason}
            </button>
            {mode.eligibility.requirement ? (
              <p className="game-mode-row__locked-progress">
                <ModeRequirement requirement={mode.eligibility.requirement} />
              </p>
            ) : null}
          </div>
        ) : (
          <>
            <GameQueueActions
              joinOptions={joinOptions}
              queueState={resolvedQueueState}
              joinUrl={resolvedJoinUrl}
              busy={queue.busy}
              selectedQueuePath={
                queue.selectedQueuePath || (isThisQueue ? activeIntent?.queuePath : '') || ''
              }
              onJoin={handleJoin}
              onLeave={handleLeave}
              disabled={!defaultQueue || blockedByMatch || Boolean(activeTableSeat?.tableId && !seatedHere)}
              prominent={prominent}
            />
            <button
              type="button"
              className={`game-list-button game-list-button-secondary${prominent ? ' game-list-button--prominent' : ''}`}
              disabled={tableBusy || blockedByMatch}
              onClick={handleCreatePrivate}
            >
              {tableBusy ? '…' : CREATE_PRIVATE_GAME}
            </button>
          </>
        )}
      </div>
```
(This replaces the existing unconditional `<GameQueueActions ... /><button ...>...</button>` pair inside `<div className="game-mode-row__actions">`.)

Update the default export to accept and forward `onNavigateToMode`:

```js
export default function GameModesPanel({
  game,
  activeIntent,
  activeTableSeat,
  onQueueChange,
  onQueueJoined,
  onTableChange,
  onNavigateToMode,
  heading = 'Play',
  variant = 'default',
}) {
```
and in the `<ModeRow>` mapping:
```jsx
          <ModeRow
            key={mode.id ?? mode.modeKey}
            game={game}
            mode={mode}
            activeIntent={activeIntent}
            activeTableSeat={activeTableSeat}
            onQueueChange={onQueueChange}
            onQueueJoined={onQueueJoined}
            onTableChange={onTableChange}
            onNavigateToMode={onNavigateToMode}
            prominent={prominent}
          />
```

Use `role="button"`-compatible semantics: since the test queries `getByRole('button', { name: ... })`, the `<button>` element used above already satisfies this — no extra `role` attribute needed.

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd frontend && npx vitest run src/components/games/GameModesPanel.test.jsx
```
Expected: PASS (all 4 tests).

- [ ] **Step 5: Run the full frontend test suite to check for regressions**

```bash
cd frontend && npm run test:run
```
Expected: all existing tests pass, including `GameQueueActions.test.jsx` (unaffected — `GameQueueActions` itself wasn't changed).

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/games/GameModesPanel.jsx frontend/src/components/games/GameModesPanel.test.jsx
git commit -m "Render locked mode state with progress and unlock routing"
```

---

### Task 13: Manual/visual verification

**Files:** none (verification only).

- [ ] **Step 1: Start the backend, fixture server, and frontend**

```bash
cd backend && go run . &
cd backend && go run ./cmd/fixturegame &
cd frontend && npm run dev
```

- [ ] **Step 2: Sign in and view the Eligibility Fixture game in the browser**

Navigate to the "Eligibility Fixture" game's detail page. Confirm all four gated modes (Arena, Legendary, Standard, Commander) render as locked with their respective reason text, and Legendary/Standard/Commander show progress readouts (Legendary: `12/50 Ranked matches`; Standard: `0/1 Standard-legal decks`; Commander: `18/25 Standard wins and 3/5 Unique decks used`). Deck Builder renders as a normal joinable mode.

- [ ] **Step 3: Verify unlock routing**

Click the Standard mode's locked reason text; confirm it navigates to the Deck Builder mode.

**Note:** the fixture's locked/unlocked state is keyed by the signed-in user's ID string containing `-locked`/`-unlocked` (per Task 6's handler) — since real signed-in users won't have IDs matching that pattern, they'll see the default-locked state for all four modes. To see the unlocked state, either temporarily edit `EligibilityFixtureHandler` to check a query param instead for this manual pass, or treat "all four locked" as the expected steady-state screenshot and trust Task 7's integration tests (which construct requests for both `-locked`- and `-unlocked`-suffixed test users directly) to cover the unlocked path. Document which approach was used when reporting this step's result.

- [ ] **Step 4: Take a screenshot for the PR description**

Capture the locked-mode UI (browser dev tools or OS screenshot) to attach to the pull request.

---

## Notes on scope

This plan implements JQ-11, JQ-12, JQ-13, and JQ-16 as a single mechanism in one branch, per the consolidated design spec. It does not touch `docs/composition-and-join-options.md`'s separate within-mode queue-option mechanism (JQ-15), and does not modify a real third-party reference game. The known frontend overlap with branch `jq-40-derive-gamemode-min-max` (both touch `ModeRow` in `GameModesPanel.jsx`, on different fields) should be re-checked at merge time.
