package gemini

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestApplyEstimatedCostGemini31FlashLite(t *testing.T) {
	inputTokens, outputTokens := int64(1000), int64(200)
	usage := Usage{
		Model:        "gemini-3.1-flash-lite",
		InputTokens:  &inputTokens,
		OutputTokens: &outputTokens,
	}

	applyEstimatedCost(&usage, time.Time{})

	if usage.CostUSD == nil || math.Abs(*usage.CostUSD-0.00055) > 1e-12 {
		t.Fatalf("cost estimate = %v, want $0.00055", usage.CostUSD)
	}
	if !strings.Contains(usage.CostBasis, "Standard paid list rate") || !strings.Contains(usage.CostBasis, "2026-11-12") {
		t.Fatalf("cost basis is not explicit and dated: %q", usage.CostBasis)
	}
}
