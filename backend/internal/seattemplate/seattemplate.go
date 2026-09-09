// Package seattemplate expands game seatTemplate trees into leaf seat keys.
package seattemplate

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Leaf is one expanded seat from a template.
type Leaf struct {
	SeatKey     string
	AffinityKey string
	QueuePath   string
	// NamePath identifies the seat's equivalence class: the segment kinds with
	// instance indices and explicit names removed, joined by "/". Two seats are
	// interchangeable exactly when their NamePaths match — the same fact
	// matchmaking relies on when it treats seat 1 and seat 5 as one slot. A flat
	// template yields "" for every seat: one class, a symmetric mode.
	NamePath string
}

var reservedKeys = map[string]struct{}{
	"count": {}, "name": {}, "displayName": {}, "min": {}, "max": {}, "sizePolicy": {},
}

// Expand parses a seatTemplate JSON object into ordered leaves.
func Expand(raw json.RawMessage) ([]Leaf, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("seattemplate: seatTemplate is required")
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("seattemplate: decode: %w", err)
	}
	if len(root) == 0 {
		return nil, fmt.Errorf("seattemplate: seatTemplate must not be empty")
	}
	if _, ok := root["seats"]; ok {
		return nil, fmt.Errorf("seattemplate: flat seats[] is not supported; use seatTemplate")
	}

	queuePaths, err := queuePaths(root, nil)
	if err != nil {
		return nil, err
	}
	defaultQueue := ""
	if len(queuePaths) == 1 {
		defaultQueue = queuePaths[0]
	}

	leaves, err := expandRoot(root)
	if err != nil {
		return nil, err
	}
	if len(leaves) == 0 {
		return nil, fmt.Errorf("seattemplate: template expands to zero seats")
	}

	out := make([]Leaf, len(leaves))
	for i, lb := range leaves {
		qp := defaultQueue
		if len(queuePaths) > 1 {
			qp = lb.queuePath
		}
		out[i] = Leaf{
			SeatKey:     strings.Join(lb.segments, "-"),
			AffinityKey: lb.affinityKey,
			QueuePath:   qp,
			NamePath:    strings.Join(lb.names, "/"),
		}
	}
	return out, nil
}

type leafBuild struct {
	segments    []string
	names       []string
	queuePath   string
	affinityKey string
}

func expandRoot(root map[string]any) ([]leafBuild, error) {
	pascal := pascalKeys(root)
	if len(pascal) == 0 {
		return expandRootCount(root)
	}
	var out []leafBuild
	for _, kind := range pascal {
		child, ok := root[kind].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("seattemplate: %q must be an object", kind)
		}
		sub, err := expandDimension(kind, child, nil, nil, "")
		if err != nil {
			return nil, err
		}
		if len(pascal) > 1 {
			for i := range sub {
				if sub[i].queuePath == "" {
					sub[i].queuePath = kind
				}
			}
		}
		out = append(out, sub...)
	}
	return out, nil
}

func expandRootCount(root map[string]any) ([]leafBuild, error) {
	n, err := nodeCount(root)
	if err != nil {
		return nil, err
	}
	out := make([]leafBuild, n)
	for i := 1; i <= n; i++ {
		out[i-1] = leafBuild{segments: []string{strconv.Itoa(i)}}
	}
	return out, nil
}

func expandDimension(kind string, node map[string]any, prefix []string, namePrefix []string, sideAffinity string) ([]leafBuild, error) {
	pascal := pascalKeys(node)
	if len(pascal) == 0 {
		return expandLeafDimension(kind, node, prefix, namePrefix, sideAffinity)
	}

	if len(pascal) > 1 {
		if hasCountKey(node) {
			count, err := nodeCount(node)
			if err != nil {
				return nil, err
			}
			newNamePrefix := append(append([]string{}, namePrefix...), kind)
			var out []leafBuild
			for i := 1; i <= count; i++ {
				seg := formatSegment(kind, i, count, node)
				newPrefix := append(append([]string{}, prefix...), seg)
				aff := sideAffinityForInstance(kind, i, count, sideAffinity)
				sub, err := expandMultiChildren(pascal, node, newPrefix, newNamePrefix, aff)
				if err != nil {
					return nil, err
				}
				out = append(out, sub...)
			}
			return out, nil
		}
		return expandMultiChildren(pascal, node, append(append([]string{}, prefix...), kind), append(append([]string{}, namePrefix...), kind), sideAffinity)
	}

	childKind := pascal[0]
	child, ok := node[childKind].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("seattemplate: %q must be an object", childKind)
	}

	if !hasCountKey(node) {
		return expandDimension(childKind, child, append(prefix, kind), append(append([]string{}, namePrefix...), kind), sideAffinity)
	}

	count, err := nodeCount(node)
	if err != nil {
		return nil, err
	}

	newNamePrefix := append(append([]string{}, namePrefix...), kind)
	var out []leafBuild
	for i := 1; i <= count; i++ {
		seg := formatSegment(kind, i, count, node)
		newPrefix := append(append([]string{}, prefix...), seg)
		aff := sideAffinityForInstance(kind, i, count, sideAffinity)
		sub, err := expandDimension(childKind, child, newPrefix, newNamePrefix, aff)
		if err != nil {
			return nil, err
		}
		out = append(out, sub...)
	}
	return out, nil
}

func expandMultiChildren(pascal []string, node map[string]any, prefix []string, namePrefix []string, sideAffinity string) ([]leafBuild, error) {
	var out []leafBuild
	for _, childKind := range pascal {
		child, ok := node[childKind].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("seattemplate: %q must be an object", childKind)
		}
		sub, err := expandDimension(childKind, child, prefix, namePrefix, sideAffinity)
		if err != nil {
			return nil, err
		}
		for i := range sub {
			sub[i].queuePath = childKind
		}
		out = append(out, sub...)
	}
	return out, nil
}

func expandLeafDimension(kind string, node map[string]any, prefix []string, namePrefix []string, sideAffinity string) ([]leafBuild, error) {
	if names, ok := nodeNames(node); ok {
		// Explicit names are instance labels, like indices: only the kind joins
		// the equivalence class, never the name itself.
		classNames := append(append([]string{}, namePrefix...), kind)
		out := make([]leafBuild, len(names))
		for i, name := range names {
			segments := append(append([]string{}, prefix...), kind, name)
			aff := sideAffinity
			if aff == "" {
				aff = kind + ":" + name
			}
			out[i] = leafBuild{segments: segments, names: classNames, affinityKey: aff}
		}
		return out, nil
	}

	n, err := nodeCount(node)
	if err != nil {
		return nil, err
	}

	// A bare index (kind == "") carries no name of its own: it's the flat
	// {"count": n} case, so the equivalence class is whatever the caller
	// already built up.
	classNames := namePrefix
	if kind != "" {
		classNames = append(append([]string{}, namePrefix...), kind)
	}

	var out []leafBuild
	for i := 1; i <= n; i++ {
		var seg string
		if kind == "" {
			seg = strconv.Itoa(i)
		} else {
			seg = formatSegment(kind, i, n, node)
		}
		segments := append(append([]string{}, prefix...), seg)
		out = append(out, leafBuild{segments: segments, names: classNames, affinityKey: sideAffinity})
	}
	return out, nil
}

func sideAffinityForInstance(kind string, index, total int, inherited string) string {
	if inherited != "" {
		return inherited
	}
	if total <= 1 {
		return kind + ":1"
	}
	return kind + ":" + strconv.Itoa(index)
}

func formatSegment(kind string, index, total int, node map[string]any) string {
	if total == 1 && isEmptyObject(node) {
		return kind
	}
	if total == 1 {
		return kind + "-1"
	}
	return kind + "-" + strconv.Itoa(index)
}

func hasCountKey(node map[string]any) bool {
	_, ok := node["count"]
	return ok
}

func nodeNames(node map[string]any) ([]string, bool) {
	raw, ok := node["name"]
	if !ok {
		return nil, false
	}
	items, ok := raw.([]any)
	if !ok || len(items) == 0 {
		return nil, false
	}
	names := make([]string, len(items))
	for i, item := range items {
		s, ok := item.(string)
		if !ok || strings.TrimSpace(s) == "" {
			return nil, false
		}
		names[i] = strings.TrimSpace(s)
	}
	return names, true
}

func nodeCount(node map[string]any) (int, error) {
	if _, ok := nodeNames(node); ok {
		return 0, fmt.Errorf("seattemplate: node uses name and must not also use count")
	}
	raw, ok := node["count"]
	if !ok {
		if isEmptyObject(node) {
			return 1, nil
		}
		return 0, fmt.Errorf("seattemplate: node requires count, name, or must be {}")
	}
	switch v := raw.(type) {
	case float64:
		n := int(v)
		if float64(n) != v || n < 1 {
			return 0, fmt.Errorf("seattemplate: count must be a positive integer")
		}
		return n, nil
	case json.Number:
		n, err := v.Int64()
		if err != nil || n < 1 {
			return 0, fmt.Errorf("seattemplate: count must be a positive integer")
		}
		return int(n), nil
	default:
		return 0, fmt.Errorf("seattemplate: count must be a positive integer")
	}
}

func isEmptyObject(node map[string]any) bool {
	for k := range node {
		if k == "count" {
			return false
		}
		if _, reserved := reservedKeys[k]; !reserved {
			return false
		}
	}
	return true
}

func pascalKeys(node map[string]any) []string {
	var keys []string
	for k := range node {
		if _, reserved := reservedKeys[k]; reserved {
			continue
		}
		if !isPascalKey(k) {
			return nil
		}
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

func isPascalKey(k string) bool {
	if k == "" {
		return false
	}
	return unicode.IsUpper(rune(k[0]))
}

func sortStrings(s []string) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

func queuePaths(node map[string]any, path []string) ([]string, error) {
	pascal := pascalKeys(node)
	switch len(pascal) {
	case 0:
		key := strings.Join(path, ".")
		return []string{key}, nil
	case 1:
		child, ok := node[pascal[0]].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("seattemplate: %q must be an object", pascal[0])
		}
		return queuePaths(child, path)
	default:
		var out []string
		for _, kind := range pascal {
			child, ok := node[kind].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("seattemplate: %q must be an object", kind)
			}
			sub, err := queuePaths(child, append(path, kind))
			if err != nil {
				return nil, err
			}
			out = append(out, sub...)
		}
		return out, nil
	}
}
