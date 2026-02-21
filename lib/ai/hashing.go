package ai

import (
	"hash/fnv"
	"sort"
)

// SelectBySessionHash deterministically selects a provider from a sorted list
// using FNV-32a hashing of sessionID + tierName. This is stateless and produces
// consistent results across all proxy instances given the same healthy provider pool.
//
// Providers are sorted by name before hashing to ensure consistent ordering.
// Returns the index of the selected provider in the sorted list.
func SelectBySessionHash(sessionID, tierName string, providerNames []string) int {
	if len(providerNames) == 0 {
		return -1
	}

	// Sort names for deterministic ordering across instances.
	sorted := make([]string, len(providerNames))
	copy(sorted, providerNames)
	sort.Strings(sorted)

	h := fnv.New32a()
	h.Write([]byte(sessionID + ":" + tierName))
	index := int(h.Sum32()) % len(sorted)

	return index
}

// SelectByTTFTWeight selects a provider using inverse-TTFT weighted random selection.
// Providers with lower average TTFT (faster responses) get proportionally more traffic.
//
// weights is a slice of average TTFT durations in nanoseconds, one per provider.
// randVal is a random float64 in [0, 1) used for selection.
// Returns the index of the selected provider.
func SelectByTTFTWeight(weights []int64, randVal float64) int {
	if len(weights) == 0 {
		return -1
	}
	if len(weights) == 1 {
		return 0
	}

	// Convert TTFT to inverse weights (lower TTFT = higher weight).
	// Use 1/ttft as weight. If ttft is 0, treat as very fast (weight = max).
	inverseWeights := make([]float64, len(weights))
	for i, w := range weights {
		if w <= 0 {
			inverseWeights[i] = 1e9 // very high weight for unmeasured/instant
		} else {
			inverseWeights[i] = 1.0 / float64(w)
		}
	}

	// Calculate total weight.
	var totalWeight float64
	for _, w := range inverseWeights {
		totalWeight += w
	}

	// Weighted random selection.
	target := randVal * totalWeight
	var cumulative float64
	for i, w := range inverseWeights {
		cumulative += w
		if target <= cumulative {
			return i
		}
	}

	// Fallback to last provider (should not reach here with valid input).
	return len(weights) - 1
}
