package ai

import (
	"crypto/rand"
	"encoding/binary"
	"math"
	"sort"

	libai "github.com/DIN-center/din-caddy-plugins/lib/ai"
)

// Tier represents a quality tier (fast, balanced, premium) containing multiple AI providers.
type Tier struct {
	Name      string        `json:"name"`
	Providers []*AIProvider `json:"providers"`
}

// GetAvailableProviders returns providers that can serve requests, prioritizing healthy over warning.
// Returns healthy providers first. If none are healthy, falls back to warning providers.
// Returns nil if all providers are unhealthy.
func (t *Tier) GetAvailableProviders() []*AIProvider {
	var healthy, warning []*AIProvider
	for _, p := range t.Providers {
		switch p.HealthStatus() {
		case Healthy:
			healthy = append(healthy, p)
		case Warning:
			warning = append(warning, p)
		}
	}

	if len(healthy) > 0 {
		return healthy
	}
	if len(warning) > 0 {
		return warning
	}
	return nil
}

// SelectProvider picks a provider from the tier based on session stickiness or optimization mode.
// If sessionID is non-empty, uses deterministic hashing for session stickiness.
// Otherwise, selects based on optimizeMode: "latency" (TTFT-weighted), "cost" (inverse-cost),
// or "balanced" (combined 60% cost + 40% latency scoring).
func (t *Tier) SelectProvider(sessionID string, optimizeMode string) *AIProvider {
	available := t.GetAvailableProviders()
	if len(available) == 0 {
		return nil
	}

	if len(available) == 1 {
		return available[0]
	}

	// Session stickiness via deterministic hashing — always takes priority.
	if sessionID != "" {
		names := make([]string, len(available))
		for i, p := range available {
			names[i] = p.Name
		}

		idx := libai.SelectBySessionHash(sessionID, t.Name, names)
		if idx < 0 {
			return nil
		}

		// Find the provider by the sorted name.
		sorted := make([]string, len(names))
		copy(sorted, names)
		sort.Strings(sorted)
		selectedName := sorted[idx]

		for _, p := range available {
			if p.Name == selectedName {
				return p
			}
		}
	}

	randVal := cryptoRandFloat64()

	switch optimizeMode {
	case OptimizeCost:
		costs := make([]float64, len(available))
		for i, p := range available {
			costs[i] = (p.InputCostPer1M + p.OutputCostPer1M) / 2
		}
		idx := libai.SelectByCostWeight(costs, randVal)
		if idx < 0 {
			return available[0]
		}
		return available[idx]

	case OptimizeBalanced:
		return t.selectBalanced(available, randVal)

	default: // OptimizeLatency
		weights := make([]int64, len(available))
		for i, p := range available {
			weights[i] = int64(p.AvgTTFT())
		}
		idx := libai.SelectByTTFTWeight(weights, randVal)
		if idx < 0 {
			return available[0]
		}
		return available[idx]
	}
}

// selectBalanced picks a provider using combined cost + latency scoring.
// Score = 0.6 * costNorm + 0.4 * latencyNorm, fed into inverse weighting.
func (t *Tier) selectBalanced(available []*AIProvider, randVal float64) *AIProvider {
	// Find max cost and max TTFT for normalization.
	var maxCost float64
	var maxTTFT float64
	hasTTFT := false
	for _, p := range available {
		avgCost := (p.InputCostPer1M + p.OutputCostPer1M) / 2
		if avgCost > maxCost {
			maxCost = avgCost
		}
		ttft := float64(p.AvgTTFT())
		if ttft > 0 {
			hasTTFT = true
			if ttft > maxTTFT {
				maxTTFT = ttft
			}
		}
	}

	// If no TTFT data, fall back to cost-only.
	if !hasTTFT || maxTTFT == 0 {
		costs := make([]float64, len(available))
		for i, p := range available {
			costs[i] = (p.InputCostPer1M + p.OutputCostPer1M) / 2
		}
		idx := libai.SelectByCostWeight(costs, randVal)
		if idx < 0 {
			return available[0]
		}
		return available[idx]
	}

	// Compute combined scores.
	scores := make([]float64, len(available))
	for i, p := range available {
		costNorm := ((p.InputCostPer1M + p.OutputCostPer1M) / 2) / maxCost
		ttft := float64(p.AvgTTFT())
		if ttft <= 0 {
			ttft = maxTTFT // Use max as fallback for unmeasured providers.
		}
		ttftNorm := ttft / maxTTFT
		scores[i] = 0.6*costNorm + 0.4*ttftNorm
	}

	// Lower score = better (cheaper + faster), so use inverse weighting.
	idx := libai.SelectByInverseWeight(scores, randVal)
	if idx < 0 {
		return available[0]
	}
	return available[idx]
}

// cryptoRandFloat64 generates a random float64 in [0, 1) using crypto/rand.
func cryptoRandFloat64() float64 {
	var b [8]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return 0.5 // fallback
	}
	// Convert to uint64, mask to 53 bits (float64 mantissa), divide by 2^53.
	n := binary.LittleEndian.Uint64(b[:])
	return float64(n>>11) / math.Exp2(53)
}
