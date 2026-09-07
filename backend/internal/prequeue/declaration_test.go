package prequeue

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseAbsentDeclarationIsNil(t *testing.T) {
	decl, err := Parse(nil)
	if err != nil {
		t.Fatalf("Parse(nil): %v", err)
	}
	if decl != nil {
		t.Fatalf("Parse(nil) = %+v, want nil", decl)
	}
}

func TestParseReadsGroups(t *testing.T) {
	raw := json.RawMessage(`{"groups":[
		{"key":"helpers","kind":"Loadout","label":"Choose your two helpers","min":2,"max":2}
	]}`)

	decl, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if decl == nil || len(decl.Groups) != 1 {
		t.Fatalf("Parse = %+v, want one group", decl)
	}
	g := decl.Groups[0]
	if g.Key != "helpers" || g.Kind != KindLoadout || g.Label != "Choose your two helpers" {
		t.Fatalf("group = %+v, want helpers/Loadout/label", g)
	}
	if g.Min != 2 || g.Max != 2 {
		t.Fatalf("group bounds = %d..%d, want 2..2", g.Min, g.Max)
	}
}

func TestParseDefaultsBoundsToExactlyOne(t *testing.T) {
	raw := json.RawMessage(`{"groups":[{"key":"champion","kind":"Character","label":"Choose your champion"}]}`)

	decl, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	g := decl.Groups[0]
	if g.Min != 1 || g.Max != 1 {
		t.Fatalf("group bounds = %d..%d, want 1..1", g.Min, g.Max)
	}
}

func TestParseRejectsUnknownKind(t *testing.T) {
	raw := json.RawMessage(`{"groups":[{"key":"x","kind":"Pet","label":"Pick a pet"}]}`)

	if _, err := Parse(raw); err == nil || !strings.Contains(err.Error(), "kind") {
		t.Fatalf("Parse err = %v, want a kind error", err)
	}
}

func TestParseRejectsDuplicateGroupKeys(t *testing.T) {
	raw := json.RawMessage(`{"groups":[
		{"key":"kit","kind":"Loadout","label":"Kit"},
		{"key":"kit","kind":"Loadout","label":"Kit again"}
	]}`)

	if _, err := Parse(raw); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("Parse err = %v, want a duplicate-key error", err)
	}
}

func TestParseRejectsEmptyGroupList(t *testing.T) {
	if _, err := Parse(json.RawMessage(`{"groups":[]}`)); err == nil {
		t.Fatal("Parse({groups:[]}) = nil error, want an error: declaring preQueue with no groups is a mistake")
	}
}

func TestParseRejectsMaxBelowMin(t *testing.T) {
	raw := json.RawMessage(`{"groups":[{"key":"kit","kind":"Loadout","label":"Kit","min":2,"max":1}]}`)

	if _, err := Parse(raw); err == nil || !strings.Contains(err.Error(), "max") {
		t.Fatalf("Parse err = %v, want a max/min error", err)
	}
}

func TestParseRejectsMissingLabel(t *testing.T) {
	raw := json.RawMessage(`{"groups":[{"key":"kit","kind":"Loadout"}]}`)

	if _, err := Parse(raw); err == nil || !strings.Contains(err.Error(), "label") {
		t.Fatalf("Parse err = %v, want a label error", err)
	}
}
