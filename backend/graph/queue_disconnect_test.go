package graph

import "testing"

// Removal is the only outcome this ticket ships. This test exists so that adding a
// second one is a deliberate change to a named decision rather than a quiet edit to
// an if-statement.
func TestDecideDisconnectOutcomeIsRemoveToday(t *testing.T) {
	if got := decideDisconnectOutcome(); got != DisconnectOutcomeRemove {
		t.Fatalf("outcome: got %v, want DisconnectOutcomeRemove", got)
	}
}
