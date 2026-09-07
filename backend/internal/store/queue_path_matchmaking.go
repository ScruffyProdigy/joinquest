package store

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/seattemplate"
)

func seatQueuePathValue(seat GameModeSeat) string {
	if seat.QueuePath == nil {
		return ""
	}
	return strings.TrimSpace(*seat.QueuePath)
}

func entryQueuePathValue(entry QueueEntry) string {
	if entry.QueuePath == nil {
		return ""
	}
	return strings.TrimSpace(*entry.QueuePath)
}

func modeUsesComposition(seats []GameModeSeat) bool {
	for _, seat := range seats {
		if seatQueuePathValue(seat) != "" {
			return true
		}
	}
	return false
}

func validQueuePaths(seats []GameModeSeat) map[string]struct{} {
	paths := make(map[string]struct{})
	for _, seat := range seats {
		if path := seatQueuePathValue(seat); path != "" {
			paths[path] = struct{}{}
		}
	}
	return paths
}

func validateJoinQueuePath(seats []GameModeSeat, queuePath string) error {
	queuePath = strings.TrimSpace(queuePath)
	if !modeUsesComposition(seats) {
		if queuePath != "" {
			return fmt.Errorf("store: queue path is not used for this mode")
		}
		return nil
	}
	if queuePath == "" {
		return fmt.Errorf("store: queue path is required for this mode")
	}
	if _, ok := validQueuePaths(seats)[queuePath]; !ok {
		return fmt.Errorf("store: invalid queue path %q", queuePath)
	}
	return nil
}

// matchSeatsFromTemplate returns the subset of seats required to fire (per-path sizeForQueue).
func matchSeatsFromTemplate(seats []GameModeSeat, template json.RawMessage) ([]GameModeSeat, error) {
	if len(template) == 0 {
		return seats, nil
	}
	specs, err := seattemplate.PathSpecs(template)
	if err != nil {
		return nil, err
	}
	byPath := make(map[string][]GameModeSeat)
	for _, seat := range seats {
		path := seatQueuePathValue(seat)
		byPath[path] = append(byPath[path], seat)
	}

	out := make([]GameModeSeat, 0, len(seats))
	for _, spec := range specs {
		bucket := byPath[spec.QueuePath]
		need := spec.PlayersToStart()
		if need > len(bucket) {
			need = len(bucket)
		}
		out = append(out, bucket[:need]...)
	}
	if len(out) == 0 {
		return seats, nil
	}
	return out, nil
}

func nullQueuePathColumn(value string) any {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

// queuePathColumnValue returns a value safe for NOT NULL queue_path columns (fifo uses "").
func queuePathColumnValue(value string) string {
	return strings.TrimSpace(value)
}

func nullUUIDColumn(value *uuid.UUID) any {
	if value == nil {
		return nil
	}
	return *value
}

func sameQueuePath(existing *string, requested string) bool {
	return entryQueuePathValue(QueueEntry{QueuePath: existing}) == strings.TrimSpace(requested)
}
