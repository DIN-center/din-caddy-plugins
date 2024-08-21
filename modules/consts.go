package modules

type HealthStatus int

const (
	// Health status enums
	Healthy HealthStatus = iota
	Warning
	Unhealthy

	// Module Context Key constants
	DinUpstreamsContextKey = "din.internal.upstreams"
	RequestProviderKey     = "request_provider"
	RequestBodyKey         = "request_body"
	HealthStatusKey        = "health_status"
	BlockNumberKey         = "block_number"

	// Health check constants
	DefaultHCMethod            = "eth_blockNumber"
	DefaultHCThreshold         = 2
	DefaultHCInterval          = 5
	DefaultBlockLagLimit       = int64(5)
	DefaultRequestAttemptCount = 5

	// Additional Status Codes
	StatusOriginUnreachable = 523
)

// String method to convert MyEnum to string
func (h HealthStatus) String() string {
	switch h {
	case Healthy:
		return "Healthy"
	case Warning:
		return "Warning"
	case Unhealthy:
		return "Unhealthy"
	default:
		return "Unknown"
	}
}
