package ai

// Tier names.
const (
	TierFast     = "fast"
	TierBalanced = "balanced"
	TierPremium  = "premium"
)

// Default configuration values.
const (
	DefaultHCInterval         = 30              // seconds between health checks
	DefaultHCThreshold        = 3               // consecutive checks before state transition
	DefaultRequestAttemptCount = 3              // max retries per request
	DefaultTTFTWindowSize     = 20              // rolling window size for TTFT measurements
)

// Adapter type names used in Caddyfile config.
const (
	AdapterOpenAI    = "openai"
	AdapterAnthropic = "anthropic"
)

// Optimization modes controlled by X-DIN-Optimize header.
const (
	OptimizeLatency  = "latency"  // default: TTFT-weighted selection
	OptimizeCost     = "cost"     // inverse-cost-weighted selection
	OptimizeBalanced = "balanced" // combined cost + latency scoring
)
