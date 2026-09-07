package spiritanimal

import (
	"log"
	"sync/atomic"
	"time"

	"github.com/scruffyprodigy/joinquest/internal/store"
)

const (
	defaultQuestionsEstimate = 45 * time.Second
	defaultMascotsEstimate   = 2 * time.Minute
)

var (
	questionsEstimateNanos atomic.Int64
	mascotsEstimateNanos   atomic.Int64
)

func init() {
	questionsEstimateNanos.Store(defaultQuestionsEstimate.Nanoseconds())
	mascotsEstimateNanos.Store(defaultMascotsEstimate.Nanoseconds())
}

func RecordQuestionsDuration(d time.Duration) {
	if d <= 0 {
		return
	}
	log.Printf("spiritanimal: questions phase completed in %s", d.Round(time.Millisecond))
	updateEstimate(&questionsEstimateNanos, d, defaultQuestionsEstimate)
}

func RecordMascotsDuration(d time.Duration) {
	if d <= 0 {
		return
	}
	log.Printf("spiritanimal: mascots phase completed in %s", d.Round(time.Millisecond))
	updateEstimate(&mascotsEstimateNanos, d, defaultMascotsEstimate)
}

func QuestionsEstimateSeconds() int {
	return estimateSeconds(questionsEstimateNanos.Load(), defaultQuestionsEstimate)
}

func MascotsEstimateSeconds() int {
	return estimateSeconds(mascotsEstimateNanos.Load(), defaultMascotsEstimate)
}

func EstimateSecondsForStatus(status string) int {
	switch status {
	case store.ReadingStatusGeneratingQuestions:
		return QuestionsEstimateSeconds()
	case store.ReadingStatusProcessing:
		return MascotsEstimateSeconds()
	default:
		return 0
	}
}

func updateEstimate(target *atomic.Int64, observed time.Duration, floor time.Duration) {
	current := time.Duration(target.Load())
	if current <= 0 {
		current = floor
	}
	next := time.Duration(float64(current)*0.7 + float64(observed)*0.3)
	if next < floor/2 {
		next = floor / 2
	}
	target.Store(next.Nanoseconds())
}

func estimateSeconds(nanos int64, fallback time.Duration) int {
	if nanos <= 0 {
		return int(fallback.Seconds())
	}
	seconds := int(time.Duration(nanos).Round(time.Second) / time.Second)
	if seconds < 1 {
		return 1
	}
	return seconds
}
