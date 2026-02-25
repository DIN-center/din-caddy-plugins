package provider

import "github.com/DIN-center/din-caddy-plugins/lib/health"

// Provider defines the minimal interface shared by all proxy provider types.
type Provider interface {
	GetName() string
	GetURL() string
	GetHeaders() map[string]string
	GetHealthStatus() health.HealthStatus
	IsAvailable() bool
	IsHealthy() bool
}
