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

// SelectByInverseWeight selects an index using inverse-proportional weighted random selection.
// Lower values get higher selection probability. zeroWeight is used when a value is <= 0.
// randVal is a random float64 in [0, 1) used for selection.
// Returns the index of the selected element, or -1 if weights is empty.
func SelectByInverseWeight(weights []float64, zeroWeight float64, randVal float64) int {
	if len(weights) == 0 {
		return -1
	}
	if len(weights) == 1 {
		return 0
	}

	// Convert to inverse weights (lower value = higher weight).
	inverseWeights := make([]float64, len(weights))
	for i, w := range weights {
		if w <= 0 {
			inverseWeights[i] = zeroWeight
		} else {
			inverseWeights[i] = 1.0 / w
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

	// Fallback to last element (should not reach here with valid input).
	return len(weights) - 1
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
	floatWeights := make([]float64, len(weights))
	for i, w := range weights {
		floatWeights[i] = float64(w)
	}
	return SelectByInverseWeight(floatWeights, 1e9, randVal)
}

// SelectByCostWeight selects a provider using inverse-cost weighted random selection.
// Cheaper providers (lower cost) get proportionally more traffic.
// costs is a slice of cost values (e.g., average of input + output cost per 1M tokens).
// randVal is a random float64 in [0, 1) used for selection.
func SelectByCostWeight(costs []float64, randVal float64) int {
	return SelectByInverseWeight(costs, 1e9, randVal)
}
