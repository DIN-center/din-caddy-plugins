package ai

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestProvider(name string, health HealthStatus) *AIProvider {
	p, _ := NewAIProvider(name, "https://api.example.com/v1/chat/completions")
	p.InputCostPer1M = 1.00
	p.OutputCostPer1M = 2.00
	p.mu.Lock()
	p.healthStatus = health
	p.mu.Unlock()
	return p
}

// setProviderHealth directly sets a provider's health status for testing.
func setProviderHealth(p *AIProvider, status HealthStatus) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.healthStatus = status
	p.successes = 0
	p.failures = 0
}

func TestTierGetAvailableProviders(t *testing.T) {
	t.Run("returns healthy providers", func(t *testing.T) {
		tier := &Tier{
			Name: TierBalanced,
			Providers: []*AIProvider{
				newTestProvider("p1", Healthy),
				newTestProvider("p2", Healthy),
				newTestProvider("p3", Unhealthy),
			},
		}

		available := tier.GetAvailableProviders()
		assert.Len(t, available, 2)
		assert.Equal(t, "p1", available[0].Name)
		assert.Equal(t, "p2", available[1].Name)
	})

	t.Run("falls back to warning when no healthy", func(t *testing.T) {
		tier := &Tier{
			Name: TierBalanced,
			Providers: []*AIProvider{
				newTestProvider("p1", Unhealthy),
				newTestProvider("p2", Warning),
				newTestProvider("p3", Warning),
			},
		}

		available := tier.GetAvailableProviders()
		assert.Len(t, available, 2)
		assert.Equal(t, "p2", available[0].Name)
		assert.Equal(t, "p3", available[1].Name)
	})

	t.Run("returns nil when all unhealthy", func(t *testing.T) {
		tier := &Tier{
			Name: TierBalanced,
			Providers: []*AIProvider{
				newTestProvider("p1", Unhealthy),
				newTestProvider("p2", Unhealthy),
			},
		}

		available := tier.GetAvailableProviders()
		assert.Nil(t, available)
	})

	t.Run("prefers healthy over warning", func(t *testing.T) {
		tier := &Tier{
			Name: TierBalanced,
			Providers: []*AIProvider{
				newTestProvider("p1", Healthy),
				newTestProvider("p2", Warning),
				newTestProvider("p3", Unhealthy),
			},
		}

		available := tier.GetAvailableProviders()
		assert.Len(t, available, 1)
		assert.Equal(t, "p1", available[0].Name)
	})

	t.Run("empty tier returns nil", func(t *testing.T) {
		tier := &Tier{Name: TierFast, Providers: nil}
		assert.Nil(t, tier.GetAvailableProviders())
	})
}

func TestTierSelectProvider(t *testing.T) {
	t.Run("returns nil when all unhealthy", func(t *testing.T) {
		tier := &Tier{
			Name: TierBalanced,
			Providers: []*AIProvider{
				newTestProvider("p1", Unhealthy),
				newTestProvider("p2", Unhealthy),
			},
		}

		provider := tier.SelectProvider("", OptimizeLatency)
		assert.Nil(t, provider)
	})

	t.Run("returns only available provider", func(t *testing.T) {
		tier := &Tier{
			Name: TierBalanced,
			Providers: []*AIProvider{
				newTestProvider("p1", Healthy),
				newTestProvider("p2", Unhealthy),
			},
		}

		provider := tier.SelectProvider("", OptimizeLatency)
		require.NotNil(t, provider)
		assert.Equal(t, "p1", provider.Name)
	})

	t.Run("session hash is deterministic", func(t *testing.T) {
		tier := &Tier{
			Name: TierBalanced,
			Providers: []*AIProvider{
				newTestProvider("openai-gpt4o", Healthy),
				newTestProvider("anthropic-sonnet", Healthy),
				newTestProvider("deepseek-chat", Healthy),
			},
		}

		// Same session ID should always pick the same provider.
		p1 := tier.SelectProvider("session-123", OptimizeLatency)
		p2 := tier.SelectProvider("session-123", OptimizeLatency)
		p3 := tier.SelectProvider("session-123", OptimizeLatency)

		require.NotNil(t, p1)
		assert.Equal(t, p1.Name, p2.Name)
		assert.Equal(t, p2.Name, p3.Name)
	})

	t.Run("no session uses TTFT-weighted selection", func(t *testing.T) {
		p1 := newTestProvider("fast-provider", Healthy)
		p2 := newTestProvider("slow-provider", Healthy)

		// Give p1 fast TTFT and p2 slow TTFT.
		p1.RecordTTFT(50 * time.Millisecond)
		p2.RecordTTFT(500 * time.Millisecond)

		tier := &Tier{
			Name:      TierBalanced,
			Providers: []*AIProvider{p1, p2},
		}

		// Run many selections and verify the faster provider gets more traffic.
		counts := map[string]int{}
		for i := 0; i < 100; i++ {
			provider := tier.SelectProvider("", OptimizeLatency)
			require.NotNil(t, provider)
			counts[provider.Name]++
		}

		// Faster provider should get significantly more selections.
		assert.Greater(t, counts["fast-provider"], counts["slow-provider"],
			"faster provider should be selected more often")
	})

	t.Run("session stickiness breaks when provider becomes unhealthy", func(t *testing.T) {
		p1 := newTestProvider("openai-gpt4o", Healthy)
		p2 := newTestProvider("anthropic-sonnet", Healthy)
		p3 := newTestProvider("deepseek-chat", Healthy)

		tier := &Tier{
			Name:      TierBalanced,
			Providers: []*AIProvider{p1, p2, p3},
		}

		// Get initial selection for a session.
		initial := tier.SelectProvider("sticky-session", OptimizeLatency)
		require.NotNil(t, initial)

		// Mark the selected provider as unhealthy.
		setProviderHealth(initial, Unhealthy)

		// Session should now route to a different provider.
		after := tier.SelectProvider("sticky-session", OptimizeLatency)
		require.NotNil(t, after)
		assert.NotEqual(t, initial.Name, after.Name,
			"should route to different provider when original is unhealthy")
	})
}

func TestSelectProvider_CostMode_CheaperGetsMoreTraffic(t *testing.T) {
	cheap := newTestProvider("cheap-provider", Healthy)
	cheap.InputCostPer1M = 0.10
	cheap.OutputCostPer1M = 0.30

	expensive := newTestProvider("expensive-provider", Healthy)
	expensive.InputCostPer1M = 10.00
	expensive.OutputCostPer1M = 40.00

	tier := &Tier{
		Name:      TierBalanced,
		Providers: []*AIProvider{cheap, expensive},
	}

	counts := map[string]int{}
	for i := 0; i < 100; i++ {
		p := tier.SelectProvider("", OptimizeCost)
		require.NotNil(t, p)
		counts[p.Name]++
	}

	assert.Greater(t, counts["cheap-provider"], counts["expensive-provider"],
		"cheaper provider should be selected more often in cost mode")
}

func TestSelectProvider_BalancedMode_ConsidersBothCostAndLatency(t *testing.T) {
	// Cheap but slow.
	cheapSlow := newTestProvider("cheap-slow", Healthy)
	cheapSlow.InputCostPer1M = 0.10
	cheapSlow.OutputCostPer1M = 0.30
	cheapSlow.RecordTTFT(500 * time.Millisecond)

	// Expensive but fast.
	expensiveFast := newTestProvider("expensive-fast", Healthy)
	expensiveFast.InputCostPer1M = 10.00
	expensiveFast.OutputCostPer1M = 40.00
	expensiveFast.RecordTTFT(50 * time.Millisecond)

	tier := &Tier{
		Name:      TierBalanced,
		Providers: []*AIProvider{cheapSlow, expensiveFast},
	}

	counts := map[string]int{}
	for i := 0; i < 100; i++ {
		p := tier.SelectProvider("", OptimizeBalanced)
		require.NotNil(t, p)
		counts[p.Name]++
	}

	// Both should get some traffic since balanced considers both factors.
	assert.Greater(t, counts["cheap-slow"], 0, "cheap-slow should get some traffic")
	assert.Greater(t, counts["expensive-fast"], 0, "expensive-fast should get some traffic")
}

func TestSelectProvider_BalancedMode_NoTTFT_FallsToCost(t *testing.T) {
	cheap := newTestProvider("cheap", Healthy)
	cheap.InputCostPer1M = 0.10
	cheap.OutputCostPer1M = 0.30

	expensive := newTestProvider("expensive", Healthy)
	expensive.InputCostPer1M = 10.00
	expensive.OutputCostPer1M = 40.00

	tier := &Tier{
		Name:      TierBalanced,
		Providers: []*AIProvider{cheap, expensive},
	}

	counts := map[string]int{}
	for i := 0; i < 100; i++ {
		p := tier.SelectProvider("", OptimizeBalanced)
		require.NotNil(t, p)
		counts[p.Name]++
	}

	// With no TTFT data, should fall back to cost-only (cheap wins).
	assert.Greater(t, counts["cheap"], counts["expensive"],
		"with no TTFT data, balanced mode should fall back to cost-only")
}

func TestSelectProvider_SessionStickinessOverridesOptimizeMode(t *testing.T) {
	cheap := newTestProvider("cheap", Healthy)
	cheap.InputCostPer1M = 0.10
	cheap.OutputCostPer1M = 0.30

	expensive := newTestProvider("expensive", Healthy)
	expensive.InputCostPer1M = 10.00
	expensive.OutputCostPer1M = 40.00

	tier := &Tier{
		Name:      TierBalanced,
		Providers: []*AIProvider{cheap, expensive},
	}

	// With session ID, should always return the same provider regardless of mode.
	p1 := tier.SelectProvider("fixed-session", OptimizeCost)
	p2 := tier.SelectProvider("fixed-session", OptimizeLatency)
	p3 := tier.SelectProvider("fixed-session", OptimizeBalanced)

	require.NotNil(t, p1)
	assert.Equal(t, p1.Name, p2.Name, "session stickiness should override optimize mode")
	assert.Equal(t, p2.Name, p3.Name, "session stickiness should override optimize mode")
}

func TestSelectProvider_LatencyModeUnchanged(t *testing.T) {
	fast := newTestProvider("fast-provider", Healthy)
	fast.RecordTTFT(50 * time.Millisecond)

	slow := newTestProvider("slow-provider", Healthy)
	slow.RecordTTFT(500 * time.Millisecond)

	tier := &Tier{
		Name:      TierBalanced,
		Providers: []*AIProvider{fast, slow},
	}

	counts := map[string]int{}
	for i := 0; i < 100; i++ {
		p := tier.SelectProvider("", OptimizeLatency)
		require.NotNil(t, p)
		counts[p.Name]++
	}

	assert.Greater(t, counts["fast-provider"], counts["slow-provider"],
		"faster provider should be selected more often in latency mode")
}

func TestTierSelectProviderDistribution(t *testing.T) {
	// Verify that different session IDs distribute across providers.
	tier := &Tier{
		Name: TierBalanced,
		Providers: []*AIProvider{
			newTestProvider("p1", Healthy),
			newTestProvider("p2", Healthy),
			newTestProvider("p3", Healthy),
		},
	}

	counts := map[string]int{}
	for i := 0; i < 300; i++ {
		p := tier.SelectProvider("session-"+string(rune(i)), OptimizeLatency)
		require.NotNil(t, p)
		counts[p.Name]++
	}

	// All 3 providers should be selected at least once.
	assert.Len(t, counts, 3, "all providers should be selected at least once")
}
