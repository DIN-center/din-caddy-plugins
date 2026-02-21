package ai

// HealthStatus represents the health state of an AI provider.
type HealthStatus int

const (
	Healthy   HealthStatus = 0
	Warning   HealthStatus = 1
	Unhealthy HealthStatus = 2
)

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

func (h HealthStatus) String() string {
	switch h {
	case Healthy:
		return "healthy"
	case Warning:
		return "warning"
	case Unhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}
