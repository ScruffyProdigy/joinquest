package seattemplate

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// PathSpec describes one lobby join bucket from a seatTemplate node.
type PathSpec struct {
	QueuePath    string
	DisplayName  string
	Min          int
	Max          int
	SizeForQueue int
	SeatCapacity int
}

// PlayersToStart returns how many players from this path are required to fire a match.
func (s PathSpec) PlayersToStart() int {
	if s.SizeForQueue > 0 {
		return s.SizeForQueue
	}
	return s.SeatCapacity
}

// PathSpecs derives per-queue-path metadata and seat capacities from a template.
func PathSpecs(raw json.RawMessage) ([]PathSpec, error) {
	leaves, err := Expand(raw)
	if err != nil {
		return nil, err
	}
	capByPath := map[string]int{}
	for _, leaf := range leaves {
		capByPath[leaf.QueuePath]++
	}

	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("seattemplate: decode: %w", err)
	}

	metaByPath := map[string]map[string]any{}
	if err := walkPathMeta(root, "", metaByPath); err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(capByPath))
	for path := range capByPath {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	out := make([]PathSpec, len(paths))
	for i, path := range paths {
		node := metaByPath[path]
		spec := PathSpec{
			QueuePath:    path,
			SeatCapacity: capByPath[path],
		}
		if node != nil {
			spec.DisplayName = stringField(node, "displayName")
			spec.Min = intField(node, "min")
			spec.Max = intField(node, "max")
			spec.SizeForQueue = intField(node, "sizeForQueue")
		}
		if spec.DisplayName == "" {
			spec.DisplayName = path
		}
		if spec.Max <= 0 {
			spec.Max = spec.SeatCapacity
		}
		out[i] = spec
	}
	return out, nil
}

// ValidateDistinctDisplayNames rejects templates whose paths share the same displayName.
func ValidateDistinctDisplayNames(specs []PathSpec) error {
	seen := map[string]string{}
	for _, spec := range specs {
		key := strings.ToLower(strings.TrimSpace(spec.DisplayName))
		if key == "" {
			continue
		}
		if other, ok := seen[key]; ok && other != spec.QueuePath {
			return fmt.Errorf("duplicate displayName %q on paths %q and %q", spec.DisplayName, other, spec.QueuePath)
		}
		seen[key] = spec.QueuePath
	}
	return nil
}

// TotalPlayersToStart sums PlayersToStart across all paths.
func TotalPlayersToStart(specs []PathSpec) int {
	total := 0
	for _, spec := range specs {
		total += spec.PlayersToStart()
	}
	return total
}

// DerivedPlayerBoundsFromPaths sums each path's own min/max, falling back to
// its full seat capacity when a path doesn't declare its own min.
func DerivedPlayerBoundsFromPaths(specs []PathSpec) (int, int) {
	if len(specs) == 0 {
		return 0, 0
	}
	var min, max int
	for _, spec := range specs {
		pathMin := spec.Min
		if pathMin <= 0 {
			pathMin = spec.SeatCapacity
		}
		min += pathMin
		max += spec.Max
	}
	return min, max
}

func walkPathMeta(node map[string]any, inheritedPath string, out map[string]map[string]any) error {
	pascal := pascalKeys(node)
	if len(pascal) == 0 {
		out[inheritedPath] = node
		return nil
	}

	rootMulti := inheritedPath == "" && len(pascal) > 1
	for _, kind := range pascal {
		child, ok := node[kind].(map[string]any)
		if !ok {
			return fmt.Errorf("seattemplate: %q must be an object", kind)
		}
		if err := walkPathMetaDimension(kind, child, inheritedPath, rootMulti, out); err != nil {
			return err
		}
	}
	return nil
}

func walkPathMetaDimension(kind string, node map[string]any, inheritedPath string, rootSiblings bool, out map[string]map[string]any) error {
	pascal := pascalKeys(node)
	if len(pascal) == 0 {
		path := inheritedPath
		if rootSiblings {
			path = kind
		}
		out[path] = node
		return nil
	}

	if len(pascal) > 1 {
		for _, childKind := range pascal {
			child, ok := node[childKind].(map[string]any)
			if !ok {
				return fmt.Errorf("seattemplate: %q must be an object", childKind)
			}
			if err := walkPathMetaDimension(childKind, child, childKind, false, out); err != nil {
				return err
			}
		}
		return nil
	}

	childKind := pascal[0]
	child, ok := node[childKind].(map[string]any)
	if !ok {
		return fmt.Errorf("seattemplate: %q must be an object", childKind)
	}

	if !hasCountKey(node) {
		nextPath := inheritedPath
		if rootSiblings {
			nextPath = kind
		}
		return walkPathMetaDimension(childKind, child, nextPath, false, out)
	}

	count, err := nodeCount(node)
	if err != nil {
		return err
	}
	nextPath := inheritedPath
	if rootSiblings {
		nextPath = kind
	}
	for i := 0; i < count; i++ {
		_ = i
		if err := walkPathMetaDimension(childKind, child, nextPath, false, out); err != nil {
			return err
		}
	}
	return nil
}

func stringField(node map[string]any, key string) string {
	raw, ok := node[key]
	if !ok {
		return ""
	}
	s, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func intField(node map[string]any, key string) int {
	raw, ok := node[key]
	if !ok {
		return 0
	}
	switch v := raw.(type) {
	case float64:
		n := int(v)
		if float64(n) == v && n >= 0 {
			return n
		}
	case json.Number:
		n, err := v.Int64()
		if err == nil && n >= 0 {
			return int(n)
		}
	}
	return 0
}
