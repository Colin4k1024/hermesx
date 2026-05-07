package metering

import "context"

// CostCalculator computes token costs using the PricingStore (DB-first)
// with hardcoded modelCosts as fallback for unknown models.
type CostCalculator struct {
	pricing *PricingStore
}

func NewCostCalculator(pricing *PricingStore) *CostCalculator {
	return &CostCalculator{pricing: pricing}
}

// Calculate returns the estimated cost in USD.
// Priority: DB pricing rule → hardcoded map → highest tier fallback.
func (cc *CostCalculator) Calculate(ctx context.Context, model string, inputTokens, outputTokens int) float64 {
	if cc.pricing != nil {
		if rule := cc.pricing.GetCost(ctx, model); rule != nil {
			return (float64(inputTokens)/1000.0)*rule.InputPer1K +
				(float64(outputTokens)/1000.0)*rule.OutputPer1K
		}
	}
	return CalculateCost(model, inputTokens, outputTokens)
}
