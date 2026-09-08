package store

import (
	"encoding/json"
	"fmt"

	"github.com/scruffyprodigy/joinquest/internal/prequeue"
)

// encodeQueueOptions renders a player's picks for a queue_options column.
//
// The column is NOT NULL DEFAULT '[]', so no-options is an empty array rather
// than NULL — every read gets a list it can range over without a nil check.
func encodeQueueOptions(selections []prequeue.Selection) ([]byte, error) {
	if len(selections) == 0 {
		return []byte(`[]`), nil
	}
	raw, err := json.Marshal(selections)
	if err != nil {
		return nil, fmt.Errorf("store: encode queue options: %w", err)
	}
	return raw, nil
}

// decodeQueueOptions reads a queue_options column back.
func decodeQueueOptions(raw []byte) ([]prequeue.Selection, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var selections []prequeue.Selection
	if err := json.Unmarshal(raw, &selections); err != nil {
		return nil, fmt.Errorf("store: decode queue options: %w", err)
	}
	return selections, nil
}
