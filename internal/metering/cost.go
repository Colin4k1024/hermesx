package metering

import "log/slog"

// ModelCost defines cost per 1K tokens for a given model.
type ModelCost struct {
	InputPer1K  float64
	OutputPer1K float64
}

// modelCosts maps model names to their per-1K token costs (USD).
var modelCosts = map[string]ModelCost{
	"gpt-4o":                             {InputPer1K: 0.0025, OutputPer1K: 0.01},
	"gpt-4o-mini":                        {InputPer1K: 0.00015, OutputPer1K: 0.0006},
	"claude-sonnet-4-20250514":           {InputPer1K: 0.003, OutputPer1K: 0.015},
	"claude-haiku-4-20250414":            {InputPer1K: 0.0008, OutputPer1K: 0.004},
	"anthropic/claude-sonnet-4-20250514": {InputPer1K: 0.003, OutputPer1K: 0.015},
	"anthropic/claude-haiku-4-20250414":  {InputPer1K: 0.0008, OutputPer1K: 0.004},
}

// highestTierCost is used as fallback for unknown models.
var highestTierCost = ModelCost{InputPer1K: 0.003, OutputPer1K: 0.015}

// CalculateCost returns the estimated cost in USD for given token counts and model.
func CalculateCost(model string, inputTokens, outputTokens int) float64 {
	cost, ok := modelCosts[model]
	if !ok {
		slog.Warn("unknown model for cost calculation, using highest tier", "model", model)
		cost = highestTierCost
	}
	return (float64(inputTokens)/1000.0)*cost.InputPer1K +
		(float64(outputTokens)/1000.0)*cost.OutputPer1K
}
