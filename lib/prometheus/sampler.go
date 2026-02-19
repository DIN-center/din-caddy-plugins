package prometheus

import (
	"fmt"
	"hash/fnv"
	"math"
	"time"
)

// HybridSampler combines hash-based deterministic sampling with error boosting
// to ensure fair sampling across all label combinations while prioritizing errors.
type HybridSampler struct {
	baseRate  float64 // Base sampling rate for normal requests
	errorRate float64 // Higher sampling rate for errors
	nowFunc   func() time.Time
}

// NewHybridSampler creates a new hybrid sampler with specified rates.
// baseRate: sampling rate for normal requests (e.g., 0.25 for 25%)
// errorRate: sampling rate for errors (e.g., 1.0 for 100%)
// Both rates must be within [0.0, 1.0]. Values outside this range will be clamped.
func NewHybridSampler(baseRate, errorRate float64) *HybridSampler {
	// Enforce rate boundaries [0.0, 1.0]
	// Rates < 0.0 or > 1.0 have no valid meaning
	if baseRate < 0.0 {
		baseRate = 0.0
	} else if baseRate > 1.0 {
		baseRate = 1.0
	}

	if errorRate < 0.0 {
		errorRate = 0.0
	} else if errorRate > 1.0 {
		errorRate = 1.0
	}

	return &HybridSampler{
		baseRate:  baseRate,
		errorRate: errorRate,
		nowFunc:   time.Now,
	}
}

// ShouldSample determines if a metric should be sampled.
// Now includes per-request uniqueness to ensure sampling happens WITHIN label groups,
// not all-or-nothing per group.
//
// isError: whether this is an error condition that should be sampled at higher rate
// labels: the label values that identify this metric series
func (hs *HybridSampler) ShouldSample(isError bool, labels ...string) bool {
	// Select appropriate sampling rate
	rate := hs.baseRate
	if isError {
		rate = hs.errorRate
	}

	// Always sample if rate is 1.0 (100% sampling)
	// Since we enforce boundaries in the constructor, rate will never exceed 1.0
	if rate >= 1.0 {
		return true
	}

	// Never sample if rate is 0 (0% sampling)
	// Since we enforce boundaries in the constructor, rate will never be negative
	if rate <= 0 {
		return false
	}

	// Hash labels AND current timestamp to ensure each request gets its own decision
	h := fnv.New64a()
	for _, label := range labels {
		h.Write([]byte(label))
		h.Write([]byte("|"))
	}

	// Add nanosecond timestamp to make each request unique
	// This ensures we sample 25% of requests within each label group,
	// not 0% or 100% for the entire group
	timestamp := hs.nowFunc().UnixNano()
	h.Write([]byte(fmt.Sprintf("%d", timestamp)))

	hashValue := h.Sum64()

	// Convert rate to threshold value
	// We use the full uint64 range for maximum precision
	threshold := uint64(float64(math.MaxUint64) * rate)

	// Decision based on hash that includes timestamp
	return hashValue < threshold
}

// ShouldSampleRequest determines if a request metric should be sampled.
// It checks for error conditions based on HTTP status and health status.
func (hs *HybridSampler) ShouldSampleRequest(responseStatus int, healthStatus string, labels ...string) bool {
	// Consider it an error if:
	// - HTTP status is 4xx or 5xx
	// - Health status is not "Healthy" (capitalized, from HealthStatus.String())
	// Note: healthStatus comes from the enum's String() method which returns "Healthy", "Warning", or "Unhealthy"
	isError := responseStatus >= 400 || (healthStatus != "Healthy" && healthStatus != "")
	return hs.ShouldSample(isError, labels...)
}

// ShouldSampleHealthCheck determines if a health check metric should be sampled.
// It checks for error conditions based on HTTP status.
func (hs *HybridSampler) ShouldSampleHealthCheck(responseStatus int, labels ...string) bool {
	// Consider it an error if HTTP status is 4xx or 5xx
	isError := responseStatus >= 400
	return hs.ShouldSample(isError, labels...)
}
