package graph

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/scruffyprodigy/joinquest/graph/model"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

// TestRegroupClientErrorCodes is the backend half of the JQ-176 contract: each regroup
// sentinel the client branches on must arrive with its RegroupErrorCode in the `code`
// extension. Dropping a case here degrades that player's experience to the generic "try
// again" — silently, because the message text would still look right.
//
// The frontend half lives in frontend/src/test/regroupErrorCodes.test.js, which reads the
// same enum out of the schema.
func TestRegroupClientErrorCodes(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code model.RegroupErrorCode
	}{
		{"no mode to regroup into", store.ErrNoRegroupMode, model.RegroupErrorCodeNoRegroupMode},
		{"match still running", store.ErrSessionNotFinished, model.RegroupErrorCodeSessionNotFinished},
		{"every seat taken", store.ErrTableFull, model.RegroupErrorCodeTableFull},
	}

	seen := map[model.RegroupErrorCode]bool{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seen[tc.code] = true

			// Wrapped, because the store rarely returns a bare sentinel.
			for _, err := range []error{tc.err, fmt.Errorf("claim regroup table: %w", tc.err)} {
				var gqlErr *gqlerror.Error
				if !errors.As(regroupClientError(err), &gqlErr) {
					t.Fatalf("regroupClientError(%v) is not a *gqlerror.Error, so it carries no extensions", err)
				}
				if got := gqlErr.Extensions["code"]; got != string(tc.code) {
					t.Errorf("code extension = %v, want %q", got, tc.code)
				}
				if strings.Contains(gqlErr.Message, "store:") {
					t.Errorf("message leaks internal text: %q", gqlErr.Message)
				}
			}
		})
	}

	// Every value the schema declares must actually be emitted by something. A code the
	// frontend can switch on but the backend never sends is a dead branch.
	for _, code := range model.AllRegroupErrorCode {
		if !seen[code] {
			t.Errorf("schema declares RegroupErrorCode.%s but no store sentinel maps to it", code)
		}
	}
}

// TestRegroupClientErrorHidesEverythingElse pins the fourth acceptance criterion: no raw
// "store: ..." text reaches a player from this path, including the ErrNotFound case that
// used to be flattened into "you did not play in this match".
func TestRegroupClientErrorHidesEverythingElse(t *testing.T) {
	for _, err := range []error{
		store.ErrNotFound,
		errors.New("store: pq: deadlock detected"),
		fmt.Errorf("claim regroup table: %w", store.ErrNotFound),
	} {
		got := regroupClientError(err)
		if got.Error() != regroupGenericMessage {
			t.Errorf("regroupClientError(%v) = %q, want the generic %q", err, got.Error(), regroupGenericMessage)
		}
		var gqlErr *gqlerror.Error
		if errors.As(got, &gqlErr) && gqlErr.Extensions["code"] != nil {
			t.Errorf("regroupClientError(%v) carries code %v; only the three actionable failures may", err, gqlErr.Extensions["code"])
		}
	}
}
