package queuewait

import (
	"testing"
	"time"
)

func TestFlowConsumptionRateIsFillsOverTheWindow(t *testing.T) {
	flow := Flow{Fills: 6, Window: 2 * time.Minute}

	if got := flow.ConsumptionRate(); got != 0.05 {
		t.Errorf("got %v players/sec, want 0.05", got)
	}
}

func TestFlowArrivalRateIsArrivalsOverTheWindow(t *testing.T) {
	flow := Flow{Arrivals: 3, Window: time.Minute}

	if got := flow.ArrivalRate(); got != 0.05 {
		t.Errorf("got %v players/sec, want 0.05", got)
	}
}

// A line nobody has matched in cannot divide by its own emptiness: the guard
// belongs in the type, so no caller has to remember it.
func TestFlowConsumptionRateIsZeroWithNoFills(t *testing.T) {
	flow := Flow{Fills: 0, Window: time.Minute}

	if got := flow.ConsumptionRate(); got != 0 {
		t.Errorf("got %v, want 0", got)
	}
}

// A zero window is a caller's bug, not a panic: reporting no rate lets the
// crossover send the estimate to the historical fallback instead.
func TestFlowRatesAreZeroWhenTheWindowIsZero(t *testing.T) {
	flow := Flow{Arrivals: 5, Fills: 5, Window: 0}

	if got := flow.ConsumptionRate(); got != 0 {
		t.Errorf("ConsumptionRate: got %v, want 0", got)
	}
	if got := flow.ArrivalRate(); got != 0 {
		t.Errorf("ArrivalRate: got %v, want 0", got)
	}
}

func TestFlowRatesAreZeroWhenTheWindowIsNegative(t *testing.T) {
	flow := Flow{Arrivals: 5, Fills: 5, Window: -time.Minute}

	if got := flow.ConsumptionRate(); got != 0 {
		t.Errorf("ConsumptionRate: got %v, want 0", got)
	}
	if got := flow.ArrivalRate(); got != 0 {
		t.Errorf("ArrivalRate: got %v, want 0", got)
	}
}
