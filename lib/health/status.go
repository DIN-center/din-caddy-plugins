package health

// HealthStatus represents the health state of a provider.
type HealthStatus int

const (
	Healthy   HealthStatus = iota
	Warning
	Unhealthy
)

// String returns the lowercase string representation of the health status.
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
