package spiritanimal

import (
	"testing"
	"time"

	"github.com/scruffyprodigy/joinquest/internal/store"
)

func TestIsReadingStale(t *testing.T) {
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	reading := &store.AvatarReading{
		Status:    store.ReadingStatusProcessing,
		UpdatedAt: now.Add(-3 * time.Minute),
	}
	if !IsReadingStale(reading, now) {
		t.Fatalf("expected processing reading older than estimate to be stale")
	}

	fresh := &store.AvatarReading{
		Status:    store.ReadingStatusProcessing,
		UpdatedAt: now.Add(-10 * time.Second),
	}
	if IsReadingStale(fresh, now) {
		t.Fatalf("expected fresh processing reading not to be stale")
	}

	ready := &store.AvatarReading{
		Status:    store.ReadingStatusReady,
		UpdatedAt: now.Add(-1 * time.Hour),
	}
	if IsReadingStale(ready, now) {
		t.Fatalf("expected ready reading not to be stale")
	}
}

func TestStaleThresholdRespectsMinimum(t *testing.T) {
	threshold := staleThreshold(store.ReadingStatusGeneratingQuestions)
	if threshold < minStaleBeforeResume {
		t.Fatalf("threshold %s below minimum %s", threshold, minStaleBeforeResume)
	}
}
