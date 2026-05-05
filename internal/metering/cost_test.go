package metering

import (
	"math"
	"testing"
)

func TestCalculateCost_KnownModel(t *testing.T) {
	// gpt-4o: input $0.0025/1K, output $0.01/1K
	cost := CalculateCost("gpt-4o", 1000, 500)
	expected := 0.0025 + 0.005
	if math.Abs(cost-expected) > 0.000001 {
		t.Errorf("expected cost %.6f, got %.6f", expected, cost)
	}
}

func TestCalculateCost_UnknownModel(t *testing.T) {
	// Unknown model should use highest tier: input $0.003/1K, output $0.015/1K
	cost := CalculateCost("unknown-model-xyz", 2000, 1000)
	expected := 0.006 + 0.015
	if math.Abs(cost-expected) > 0.000001 {
		t.Errorf("expected cost %.6f, got %.6f", expected, cost)
	}
}

func TestCalculateCost_ZeroTokens(t *testing.T) {
	cost := CalculateCost("gpt-4o", 0, 0)
	if cost != 0 {
		t.Errorf("expected zero cost for zero tokens, got %.6f", cost)
	}
}
