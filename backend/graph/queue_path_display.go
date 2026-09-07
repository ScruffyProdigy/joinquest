package graph

import (
	"encoding/json"
	"strings"

	"github.com/scruffyprodigy/joinquest/internal/seattemplate"
)

func queuePathDisplayName(seatTemplate json.RawMessage, queuePath *string) *string {
	if queuePath == nil {
		return nil
	}
	path := strings.TrimSpace(*queuePath)
	if path == "" {
		return nil
	}

	specs, err := seattemplate.PathSpecs(seatTemplate)
	if err != nil {
		return &path
	}
	for _, spec := range specs {
		if spec.QueuePath == path {
			label := strings.TrimSpace(spec.DisplayName)
			if label != "" {
				return &label
			}
			return &path
		}
	}
	return &path
}
