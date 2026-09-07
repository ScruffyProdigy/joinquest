package spiritanimal

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

const minStaleBeforeResume = 90 * time.Second

var resumingReadings sync.Map

func staleThreshold(status string) time.Duration {
	estimate := time.Duration(EstimateSecondsForStatus(status)) * time.Second
	threshold := estimate + estimate/2
	if threshold < minStaleBeforeResume {
		return minStaleBeforeResume
	}
	return threshold
}

// IsReadingStale reports whether an async spirit-animal phase looks abandoned.
func IsReadingStale(reading *store.AvatarReading, now time.Time) bool {
	if reading == nil {
		return false
	}
	switch reading.Status {
	case store.ReadingStatusGeneratingQuestions, store.ReadingStatusProcessing:
	default:
		return false
	}
	return now.Sub(reading.UpdatedAt) >= staleThreshold(reading.Status)
}

// MaybeResumeReading restarts a stale async phase in the background.
func (r *Runner) MaybeResumeReading(ctx context.Context, reading *store.AvatarReading) {
	if r == nil || reading == nil || !IsReadingStale(reading, time.Now()) {
		return
	}
	if !markResuming(reading.ID) {
		return
	}

	readingID := reading.ID
	status := reading.Status
	log.Printf("spiritanimal: resuming stale reading %s (%s)", readingID, status)

	go func() {
		defer unmarkResuming(readingID)
		bg := context.Background()
		if _, err := r.Store.UpdateAvatarReadingStatus(bg, readingID, status, nil); err != nil {
			log.Printf("spiritanimal: refresh phase start %s: %v", readingID, err)
		}
		switch status {
		case store.ReadingStatusGeneratingQuestions:
			r.generateQuestions(bg, readingID)
		case store.ReadingStatusProcessing:
			r.processAnswers(bg, readingID)
		}
	}()
}

// ResumeAllStaleReadings scans for abandoned readings and resumes them.
func (r *Runner) ResumeAllStale(ctx context.Context) {
	if r == nil || r.Store == nil {
		return
	}
	readings, err := r.Store.ListIncompleteAvatarReadings(ctx)
	if err != nil {
		log.Printf("spiritanimal: list incomplete readings: %v", err)
		return
	}
	now := time.Now()
	for _, reading := range readings {
		if reading == nil || !IsReadingStale(reading, now) {
			continue
		}
		r.MaybeResumeReading(ctx, reading)
	}
}

func markResuming(readingID uuid.UUID) bool {
	_, loaded := resumingReadings.LoadOrStore(readingID, struct{}{})
	return !loaded
}

func unmarkResuming(readingID uuid.UUID) {
	resumingReadings.Delete(readingID)
}
