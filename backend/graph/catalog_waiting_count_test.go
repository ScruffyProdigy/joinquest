package graph

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/graph/model"
	"github.com/scruffyprodigy/joinquest/internal/queuewait"
)

// fakeCountSource stands in for the store's grouped aggregate so the resolver's
// own behaviour is testable without a database, and so a test can count how
// many aggregates a page of cards actually costs.
type fakeCountSource struct {
	byQueue map[queuewait.QueueKey]int
	err     error
	calls   int
}

func (f *fakeCountSource) load(context.Context) (map[queuewait.QueueKey]int, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.byQueue, nil
}

// resolverForCounts wires a resolver to a fake aggregate, through a real cache.
func resolverForCounts(src *fakeCountSource) *Resolver {
	return &Resolver{WaitingCounts: queuewait.NewCountCache(src.load, time.Minute)}
}

func TestModeQueueWaitingCountCostsOneCountForAWholePageOfCards(t *testing.T) {
	first, second, third := uuid.New(), uuid.New(), uuid.New()
	src := &fakeCountSource{byQueue: map[queuewait.QueueKey]int{
		{ModeQueueID: first}:  2,
		{ModeQueueID: second}: 4,
		{ModeQueueID: third}:  6,
	}}
	r := resolverForCounts(src)

	wantCounts := []int{2, 4, 6}
	for i, queueID := range []uuid.UUID{first, second, third} {
		got, err := r.ModeQueue().WaitingCount(context.Background(), &model.ModeQueue{ID: queueID.String()})
		if err != nil {
			t.Fatalf("WaitingCount(%v): %v", queueID, err)
		}
		if got != wantCounts[i] {
			t.Errorf("queue %v got %d waiting, want %d", queueID, got, wantCounts[i])
		}
	}

	if src.calls != 1 {
		t.Errorf("counted %d times for 3 mode cards, want 1 — this field must not be an N+1", src.calls)
	}
}

func TestModeQueueWaitingCountReadsAnEmptyQueueAsZero(t *testing.T) {
	src := &fakeCountSource{byQueue: map[queuewait.QueueKey]int{{ModeQueueID: uuid.New()}: 3}}
	r := resolverForCounts(src)

	got, err := r.ModeQueue().WaitingCount(context.Background(), &model.ModeQueue{ID: uuid.NewString()})
	if err != nil {
		t.Fatalf("WaitingCount: %v", err)
	}
	// A queue with nobody in it returns no rows from the aggregate. That has to
	// read as a plain zero, not as an error or a gap in the data.
	if got != 0 {
		t.Errorf("got %d for a queue with nobody waiting, want 0", got)
	}
}

func TestModeQueueWaitingCountTotalsACompositionModesRoles(t *testing.T) {
	queueID := uuid.New()
	r := resolverForCounts(&fakeCountSource{byQueue: map[queuewait.QueueKey]int{
		{ModeQueueID: queueID, QueuePath: "tank"}:    1,
		{ModeQueueID: queueID, QueuePath: "damage"}:  9,
		{ModeQueueID: queueID, QueuePath: "support"}: 2,
	}})

	got, err := r.ModeQueue().WaitingCount(context.Background(), &model.ModeQueue{ID: queueID.String()})
	if err != nil {
		t.Fatalf("WaitingCount: %v", err)
	}
	// Unlike estimatedWaitSeconds, which stays null for a composition mode
	// because averaging its roles would describe nobody, a count of people is
	// meaningfully additive: twelve players really are waiting for this mode.
	if got != 12 {
		t.Errorf("got %d, want this mode's three roles summed to 12", got)
	}
}

func TestModeQueueWaitingCountPropagatesFailure(t *testing.T) {
	wantErr := errors.New("database is down")
	r := resolverForCounts(&fakeCountSource{err: wantErr})

	got, err := r.ModeQueue().WaitingCount(context.Background(), &model.ModeQueue{ID: uuid.NewString()})
	if !errors.Is(err, wantErr) {
		t.Fatalf("got error %v, want it to wrap %v — a broken lookup must not read as an empty queue", err, wantErr)
	}
	if got != 0 {
		t.Errorf("got %d alongside an error, want 0", got)
	}
}

func TestModeQueueWaitingCountRejectsAMalformedQueueID(t *testing.T) {
	r := resolverForCounts(&fakeCountSource{})

	if _, err := r.ModeQueue().WaitingCount(context.Background(), &model.ModeQueue{ID: "not-a-uuid"}); err == nil {
		t.Fatal("got no error for a malformed queue id, want one")
	}
}

func TestModeQueueWaitingCountsByPathReportsEveryRole(t *testing.T) {
	queueID := uuid.New()
	r := resolverForCounts(&fakeCountSource{byQueue: map[queuewait.QueueKey]int{
		{ModeQueueID: queueID, QueuePath: "tank"}:    1,
		{ModeQueueID: queueID, QueuePath: "damage"}:  9,
		{ModeQueueID: queueID, QueuePath: "support"}: 2,
		{ModeQueueID: uuid.New(), QueuePath: "tank"}: 99,
	}})

	got, err := r.ModeQueue().WaitingCountsByPath(context.Background(), &model.ModeQueue{ID: queueID.String()})
	if err != nil {
		t.Fatalf("WaitingCountsByPath: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d paths, want this queue's 3", len(got))
	}
	// Sorted by path so a card renders in a stable order across polls.
	wantPaths := []string{"damage", "support", "tank"}
	wantCounts := []int{9, 2, 1}
	for i, want := range wantPaths {
		if got[i].QueuePath != want {
			t.Errorf("path %d is %q, want %q", i, got[i].QueuePath, want)
		}
		if got[i].WaitingCount != wantCounts[i] {
			t.Errorf("%s got %d waiting, want %d", want, got[i].WaitingCount, wantCounts[i])
		}
	}
}

func TestModeQueueWaitingCountsByPathCostsOneCountAlongsideTheTotal(t *testing.T) {
	queueID := uuid.New()
	src := &fakeCountSource{byQueue: map[queuewait.QueueKey]int{
		{ModeQueueID: queueID, QueuePath: "tank"}:   1,
		{ModeQueueID: queueID, QueuePath: "damage"}: 9,
	}}
	r := resolverForCounts(src)
	card := &model.ModeQueue{ID: queueID.String()}
	ctx := context.Background()

	// One card reads both fields, which is what a composition mode's card does.
	if _, err := r.ModeQueue().WaitingCount(ctx, card); err != nil {
		t.Fatalf("WaitingCount: %v", err)
	}
	if _, err := r.ModeQueue().WaitingCountsByPath(ctx, card); err != nil {
		t.Fatalf("WaitingCountsByPath: %v", err)
	}

	if src.calls != 1 {
		t.Errorf("counted %d times for one card's two fields, want 1 — both read the same snapshot", src.calls)
	}
}

func TestModeQueueWaitingCountsByPathIsEmptyForAModeWithoutRoles(t *testing.T) {
	queueID := uuid.New()
	r := resolverForCounts(&fakeCountSource{byQueue: map[queuewait.QueueKey]int{
		{ModeQueueID: queueID}: 6,
	}})

	got, err := r.ModeQueue().WaitingCountsByPath(context.Background(), &model.ModeQueue{ID: queueID.String()})
	if err != nil {
		t.Fatalf("WaitingCountsByPath: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d paths, want none — this mode does not split by role", len(got))
	}
}

func TestModeQueueWaitingCountsByPathPropagatesFailure(t *testing.T) {
	wantErr := errors.New("database is down")
	r := resolverForCounts(&fakeCountSource{err: wantErr})

	if _, err := r.ModeQueue().WaitingCountsByPath(context.Background(), &model.ModeQueue{ID: uuid.NewString()}); !errors.Is(err, wantErr) {
		t.Fatalf("got error %v, want it to wrap %v", err, wantErr)
	}
}

func TestWaitingCountsDefaultToACacheOverTheStore(t *testing.T) {
	r := &Resolver{}

	if r.waitingCounts() == nil {
		t.Error("got no waiting-count cache, want one built by default")
	}
}

func TestWaitingCountsAreServedFresherThanWaitEstimates(t *testing.T) {
	// Estimates are medians over days and barely move; a waiting count is a live
	// population a joining player expects to see themselves in. Serving the two
	// behind the same TTL would make one of them wrong.
	if waitingCountTTL >= waitEstimateTTL {
		t.Errorf("waiting counts are cached for %v and estimates for %v, want counts strictly fresher", waitingCountTTL, waitEstimateTTL)
	}
}

func TestWaitingCountSourceFailsWithoutAStore(t *testing.T) {
	if _, err := waitingCountSource(nil)(context.Background()); err == nil {
		t.Fatal("got no error counting without a store, want one rather than a silent zero")
	}
}
