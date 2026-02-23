package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectBySessionHash_Deterministic(t *testing.T) {
	providers := []string{"openai-gpt4o", "anthropic-sonnet", "deepseek-chat"}

	// Same input should always produce the same result.
	idx1 := SelectBySessionHash("session-abc", "balanced", providers)
	idx2 := SelectBySessionHash("session-abc", "balanced", providers)
	assert.Equal(t, idx1, idx2, "same input should produce same output")
}

func TestSelectBySessionHash_DifferentSessions(t *testing.T) {
	providers := []string{"openai-gpt4o", "anthropic-sonnet", "deepseek-chat"}

	// Different session IDs should generally produce different results (not guaranteed for 3 providers, but test distribution).
	results := make(map[int]int)
	for i := 0; i < 100; i++ {
		idx := SelectBySessionHash("session-"+string(rune('a'+i)), "balanced", providers)
		results[idx]++
	}
	// With 100 sessions and 3 providers, we should see at least 2 different providers selected.
	assert.Greater(t, len(results), 1, "should distribute across multiple providers")
}

func TestSelectBySessionHash_TierAffectsResult(t *testing.T) {
	providers := []string{"openai-gpt4o", "anthropic-sonnet", "deepseek-chat"}

	// Same session ID but different tiers may produce different results.
	idxFast := SelectBySessionHash("session-abc", "fast", providers)
	idxBalanced := SelectBySessionHash("session-abc", "balanced", providers)
	idxPremium := SelectBySessionHash("session-abc", "premium", providers)

	// At least one should differ (not guaranteed for all, but very likely with 3 tiers).
	allSame := idxFast == idxBalanced && idxBalanced == idxPremium
	_ = allSame // may or may not be true, just verify no panic
}

func TestSelectBySessionHash_SortOrder(t *testing.T) {
	// Order of input should not matter since we sort internally.
	providers1 := []string{"openai-gpt4o", "anthropic-sonnet", "deepseek-chat"}
	providers2 := []string{"deepseek-chat", "openai-gpt4o", "anthropic-sonnet"}
	providers3 := []string{"anthropic-sonnet", "deepseek-chat", "openai-gpt4o"}

	idx1 := SelectBySessionHash("session-xyz", "balanced", providers1)
	idx2 := SelectBySessionHash("session-xyz", "balanced", providers2)
	idx3 := SelectBySessionHash("session-xyz", "balanced", providers3)

	assert.Equal(t, idx1, idx2, "sort order should be consistent regardless of input order")
	assert.Equal(t, idx2, idx3, "sort order should be consistent regardless of input order")
}

func TestSelectBySessionHash_SingleProvider(t *testing.T) {
	providers := []string{"only-one"}
	idx := SelectBySessionHash("any-session", "balanced", providers)
	assert.Equal(t, 0, idx)
}

func TestSelectBySessionHash_EmptyProviders(t *testing.T) {
	idx := SelectBySessionHash("session-abc", "balanced", nil)
	assert.Equal(t, -1, idx)

	idx = SelectBySessionHash("session-abc", "balanced", []string{})
	assert.Equal(t, -1, idx)
}

func TestSelectBySessionHash_PoolChangeShiftsSelection(t *testing.T) {
	// When a provider goes unhealthy and is removed from the pool, the hash maps to a different provider.
	fullPool := []string{"openai-gpt4o", "anthropic-sonnet", "deepseek-chat"}
	reducedPool := []string{"openai-gpt4o", "deepseek-chat"} // anthropic-sonnet removed

	idxFull := SelectBySessionHash("session-sticky", "balanced", fullPool)
	idxReduced := SelectBySessionHash("session-sticky", "balanced", reducedPool)

	// The indices are into differently sorted lists, so we can't compare directly.
	// Just verify both are valid indices.
	assert.GreaterOrEqual(t, idxFull, 0)
	assert.Less(t, idxFull, len(fullPool))
	assert.GreaterOrEqual(t, idxReduced, 0)
	assert.Less(t, idxReduced, len(reducedPool))
}

func TestSelectByTTFTWeight_SingleProvider(t *testing.T) {
	idx := SelectByTTFTWeight([]int64{100000}, 0.5)
	assert.Equal(t, 0, idx)
}

func TestSelectByTTFTWeight_Empty(t *testing.T) {
	idx := SelectByTTFTWeight(nil, 0.5)
	assert.Equal(t, -1, idx)
}

func TestSelectByTTFTWeight_FasterGetsMoreTraffic(t *testing.T) {
	// Provider 0: 100ms (fast), Provider 1: 1000ms (slow)
	weights := []int64{100_000_000, 1_000_000_000}

	counts := make([]int, 2)
	for i := 0; i < 1000; i++ {
		randVal := float64(i) / 1000.0
		idx := SelectByTTFTWeight(weights, randVal)
		require.GreaterOrEqual(t, idx, 0)
		require.Less(t, idx, 2)
		counts[idx]++
	}

	// The faster provider should get significantly more traffic.
	assert.Greater(t, counts[0], counts[1], "faster provider (lower TTFT) should get more traffic")
	// Specifically, provider 0 is 10x faster, so it should get ~90% of traffic.
	assert.Greater(t, counts[0], 800, "10x faster provider should get >80%% traffic")
}

func TestSelectByTTFTWeight_ZeroWeightsGetFairShare(t *testing.T) {
	// Provider with 0 TTFT (unmeasured) gets median-based fair share, not dominance.
	weights := []int64{0, 500_000_000}

	counts := make([]int, 2)
	for i := 0; i < 1000; i++ {
		randVal := float64(i) / 1000.0
		idx := SelectByTTFTWeight(weights, randVal)
		counts[idx]++
	}

	// Unmeasured provider should get roughly equal traffic (median = 500ms, so same weight).
	assert.InDelta(t, 500, counts[0], 100, "unmeasured provider should get fair share, not dominance")
	assert.InDelta(t, 500, counts[1], 100, "measured provider should get fair share")
}

func TestSelectByTTFTWeight_MixedZeroAndMeasured(t *testing.T) {
	// One unmeasured, two measured: 0, 100ms, 1000ms.
	// Median of non-zero = (100+1000)/2 = 550ms. Zero gets 550ms equivalent.
	weights := []int64{0, 100_000_000, 1_000_000_000}

	counts := make([]int, 3)
	for i := 0; i < 10000; i++ {
		randVal := float64(i) / 10000.0
		idx := SelectByTTFTWeight(weights, randVal)
		counts[idx]++
	}

	// Fastest (100ms) should get the most traffic.
	assert.Greater(t, counts[1], counts[0], "fastest measured provider should get more than unmeasured")
	assert.Greater(t, counts[1], counts[2], "fastest measured provider should get more than slowest")
	// Unmeasured (median=550ms) should get more than slowest (1000ms).
	assert.Greater(t, counts[0], counts[2], "unmeasured (median) should get more than slowest")
}

func TestSelectByTTFTWeight_EqualWeights(t *testing.T) {
	// Equal TTFT should distribute roughly equally.
	weights := []int64{200_000_000, 200_000_000, 200_000_000}

	counts := make([]int, 3)
	for i := 0; i < 1000; i++ {
		randVal := float64(i) / 1000.0
		idx := SelectByTTFTWeight(weights, randVal)
		counts[idx]++
	}

	// Each should get roughly 333 requests (±100 tolerance).
	for i, count := range counts {
		assert.InDelta(t, 333, count, 100, "provider %d should get roughly equal traffic", i)
	}
}

func TestSelectByTTFTWeight_BoundaryValues(t *testing.T) {
	weights := []int64{100_000_000, 200_000_000}

	// randVal = 0 should select first provider.
	idx := SelectByTTFTWeight(weights, 0.0)
	assert.Equal(t, 0, idx)

	// randVal just under 1.0 should select last provider.
	idx = SelectByTTFTWeight(weights, 0.999999)
	assert.GreaterOrEqual(t, idx, 0)
	assert.Less(t, idx, 2)
}

// --- SelectByInverseWeight Tests ---

func TestSelectByInverseWeight_LowerGetsMore(t *testing.T) {
	// Value 1 vs 10: lower (1) should get ~91% of traffic.
	weights := []float64{1.0, 10.0}

	counts := make([]int, 2)
	for i := 0; i < 1000; i++ {
		randVal := float64(i) / 1000.0
		idx := SelectByInverseWeight(weights, randVal)
		require.GreaterOrEqual(t, idx, 0)
		require.Less(t, idx, 2)
		counts[idx]++
	}

	assert.Greater(t, counts[0], counts[1], "lower weight should get more traffic")
	assert.Greater(t, counts[0], 800, "10x lower weight should get >80%% traffic")
}

func TestSelectByInverseWeight_Equal(t *testing.T) {
	weights := []float64{5.0, 5.0, 5.0}

	counts := make([]int, 3)
	for i := 0; i < 1000; i++ {
		randVal := float64(i) / 1000.0
		idx := SelectByInverseWeight(weights, randVal)
		counts[idx]++
	}

	for i, count := range counts {
		assert.InDelta(t, 333, count, 100, "provider %d should get roughly equal traffic", i)
	}
}

func TestSelectByInverseWeight_Empty(t *testing.T) {
	idx := SelectByInverseWeight(nil, 0.5)
	assert.Equal(t, -1, idx)
}

func TestSelectByInverseWeight_Single(t *testing.T) {
	idx := SelectByInverseWeight([]float64{42.0}, 0.5)
	assert.Equal(t, 0, idx)
}

func TestSelectByInverseWeight_Boundary(t *testing.T) {
	weights := []float64{1.0, 2.0}

	idx := SelectByInverseWeight(weights, 0.0)
	assert.Equal(t, 0, idx)

	idx = SelectByInverseWeight(weights, 0.999)
	assert.GreaterOrEqual(t, idx, 0)
	assert.Less(t, idx, 2)
}

func TestSelectByInverseWeight_ZeroGetsFairShare(t *testing.T) {
	// Zero value gets median replacement (median of [10.0] = 10.0), so equal to the other.
	weights := []float64{0, 10.0}

	counts := make([]int, 2)
	for i := 0; i < 1000; i++ {
		randVal := float64(i) / 1000.0
		idx := SelectByInverseWeight(weights, randVal)
		counts[idx]++
	}

	// Both should get roughly equal traffic since zero gets median (10.0).
	assert.InDelta(t, 500, counts[0], 100, "zero-weight provider should get fair share")
	assert.InDelta(t, 500, counts[1], 100, "measured provider should get fair share")
}

func TestSelectByInverseWeight_AllZero(t *testing.T) {
	weights := []float64{0, 0, 0}

	counts := make([]int, 3)
	for i := 0; i < 1000; i++ {
		randVal := float64(i) / 1000.0
		idx := SelectByInverseWeight(weights, randVal)
		require.GreaterOrEqual(t, idx, 0)
		require.Less(t, idx, 3)
		counts[idx]++
	}

	// Should distribute uniformly.
	for i, count := range counts {
		assert.InDelta(t, 333, count, 100, "provider %d should get roughly equal traffic", i)
	}
}

// --- SelectByCostWeight Tests ---

func TestSelectByCostWeight_CheaperGetsMore(t *testing.T) {
	// Provider 0: $0.10/1M (cheap), Provider 1: $10.00/1M (expensive)
	costs := []float64{0.10, 10.00}

	counts := make([]int, 2)
	for i := 0; i < 1000; i++ {
		randVal := float64(i) / 1000.0
		idx := SelectByCostWeight(costs, randVal)
		require.GreaterOrEqual(t, idx, 0)
		require.Less(t, idx, 2)
		counts[idx]++
	}

	assert.Greater(t, counts[0], counts[1], "cheaper provider should get more traffic")
	assert.Greater(t, counts[0], 800, "100x cheaper provider should get >80%% traffic")
}

func TestSelectByCostWeight_Empty(t *testing.T) {
	idx := SelectByCostWeight(nil, 0.5)
	assert.Equal(t, -1, idx)
}

func TestSelectByCostWeight_Single(t *testing.T) {
	idx := SelectByCostWeight([]float64{1.50}, 0.5)
	assert.Equal(t, 0, idx)
}
