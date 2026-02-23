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
// Lower values get higher selection probability. Zero/negative values are replaced with the
// median of non-zero values to give unmeasured entries a fair share (not dominant share).
// If all values are zero, distributes uniformly.
// randVal is a random float64 in [0, 1) used for selection.
// Returns the index of the selected element, or -1 if weights is empty.
func SelectByInverseWeight(weights []float64, randVal float64) int {
	if len(weights) == 0 {
		return -1
	}
	if len(weights) == 1 {
		return 0
	}

	// Collect non-zero values to compute median replacement for zeros.
	var nonZero []float64
	for _, w := range weights {
		if w > 0 {
			nonZero = append(nonZero, w)
		}
	}

	// All zero: uniform distribution.
	if len(nonZero) == 0 {
		return int(randVal * float64(len(weights)))
	}

	// Compute median of non-zero values.
	sort.Float64s(nonZero)
	var median float64
	n := len(nonZero)
	if n%2 == 0 {
		median = (nonZero[n/2-1] + nonZero[n/2]) / 2
	} else {
		median = nonZero[n/2]
	}

	// Convert to inverse weights (lower value = higher weight).
	// Zero values get the median, giving them fair share instead of dominance.
	inverseWeights := make([]float64, len(weights))
	for i, w := range weights {
		if w <= 0 {
			inverseWeights[i] = 1.0 / median
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
	return SelectByInverseWeight(floatWeights, randVal)
}

// SelectByCostWeight selects a provider using inverse-cost weighted random selection.
// Cheaper providers (lower cost) get proportionally more traffic.
// costs is a slice of cost values (e.g., average of input + output cost per 1M tokens).
// randVal is a random float64 in [0, 1) used for selection.
func SelectByCostWeight(costs []float64, randVal float64) int {
	return SelectByInverseWeight(costs, randVal)
}
