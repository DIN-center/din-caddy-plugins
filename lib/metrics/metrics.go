package metrics

// HealthCheckRecorder records health check outcomes.
// Both proxy types implement this interface to allow shared health
// check infrastructure to record metrics without coupling to specific
// Prometheus label schemas.
type HealthCheckRecorder interface {
	RecordHealthCheck(provider string, statusCode string, healthStatus string)
}
