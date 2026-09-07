package lfg

import (
	"strings"

	"github.com/scruffyprodigy/joinquest/internal/seattemplate"
)

// Assignment is a placed player on a forming match seat map.
type Assignment struct {
	SeatKey     string
	UserID      string // empty when unassigned
	QueuePath   string
	AffinityKey string
}

// PathGap is remaining need for one queue path bucket.
type PathGap struct {
	QueuePath   string
	DisplayName string
	Assigned    int
	Needed      int
}

// ComputePathGaps returns per-path assigned vs required counts from PathSpecs.
func ComputePathGaps(specs []seattemplate.PathSpec, assignments []Assignment) []PathGap {
	assignedByPath := map[string]int{}
	for _, a := range assignments {
		if strings.TrimSpace(a.UserID) == "" {
			continue
		}
		path := strings.TrimSpace(a.QueuePath)
		assignedByPath[path]++
	}

	out := make([]PathGap, len(specs))
	for i, spec := range specs {
		path := strings.TrimSpace(spec.QueuePath)
		assigned := assignedByPath[path]
		needed := spec.PlayersToStart() - assigned
		if needed < 0 {
			needed = 0
		}
		display := spec.DisplayName
		if display == "" {
			display = path
		}
		out[i] = PathGap{
			QueuePath:   path,
			DisplayName: display,
			Assigned:    assigned,
			Needed:      needed,
		}
	}
	return out
}

// ReadyToFire is true when every path bucket meets PlayersToStart.
func ReadyToFire(gaps []PathGap) bool {
	if len(gaps) == 0 {
		return false
	}
	for _, g := range gaps {
		if g.Needed > 0 {
			return false
		}
	}
	return true
}

// TargetPlayerCount returns the fire size implied by PathSpecs.
func TargetPlayerCount(specs []seattemplate.PathSpec) int {
	total := 0
	for _, spec := range specs {
		total += spec.PlayersToStart()
	}
	return total
}
