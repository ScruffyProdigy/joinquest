package graph

import (
	"context"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/graph/model"
	"github.com/scruffyprodigy/joinquest/internal/lfg"
)

func toGraphQLPathGaps(gaps []lfg.PathGap) []*model.QueuePathGap {
	if len(gaps) == 0 {
		return []*model.QueuePathGap{}
	}
	out := make([]*model.QueuePathGap, len(gaps))
	for i, g := range gaps {
		out[i] = &model.QueuePathGap{
			QueuePath:   g.QueuePath,
			DisplayName: g.DisplayName,
			Assigned:    g.Assigned,
			Needed:      g.Needed,
		}
	}
	return out
}

func neededPathGaps(gaps []lfg.PathGap) []*model.QueuePathGap {
	all := toGraphQLPathGaps(gaps)
	out := make([]*model.QueuePathGap, 0, len(all))
	for _, g := range all {
		if g.Needed > 0 {
			out = append(out, g)
		}
	}
	if out == nil {
		return []*model.QueuePathGap{}
	}
	return out
}

func (r *Resolver) formingGapsForModeQueue(ctx context.Context, modeQueueID uuid.UUID) ([]*model.QueuePathGap, error) {
	st, err := r.requireStore()
	if err != nil {
		return nil, err
	}
	fm, err := st.GetFillingFormingMatchByModeQueueID(ctx, modeQueueID)
	if err != nil {
		return nil, err
	}
	if fm == nil {
		return []*model.QueuePathGap{}, nil
	}
	gaps, err := st.FormingPathGaps(ctx, fm)
	if err != nil {
		return nil, err
	}
	return neededPathGaps(gaps), nil
}

func (r *Resolver) tableFormingGaps(ctx context.Context, tableID uuid.UUID) ([]*model.QueuePathGap, bool, error) {
	st, err := r.requireStore()
	if err != nil {
		return nil, false, err
	}
	gaps, active, err := st.TableFormingGaps(ctx, tableID)
	if err != nil {
		return nil, false, err
	}
	return neededPathGaps(gaps), active, nil
}

func (r *myTableSeatResolver) resolveTableSeatGaps(ctx context.Context, tableID string) ([]*model.QueuePathGap, bool, error) {
	if tableID == "" {
		return []*model.QueuePathGap{}, false, nil
	}
	tid, err := parseUUID(tableID, "table id")
	if err != nil {
		return nil, false, err
	}
	return r.tableFormingGaps(ctx, tid)
}
