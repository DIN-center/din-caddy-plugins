// Package network provides the core network and provider types for the DIN middleware.
// This package contains the Network and Provider structures that manage RPC provider
// health checking, load balancing, and request routing.
package network

import (
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	networklib "github.com/DIN-center/din-caddy-plugins/lib/network"
	ws "github.com/DIN-center/din-caddy-plugins/lib/watcherscore"
)

// Re-export HealthStatus from modules/consts.go for convenience.
// The canonical definition remains in modules/consts.go since it's used
// extensively in Caddyfile parsing and other Caddy-specific code.
// Consumers of this package should use these re-exported values.
type HealthStatus = int

const (
	Healthy   HealthStatus = 0
	Warning   HealthStatus = 1
	Unhealthy HealthStatus = 2
)

// HealthStatusString converts a HealthStatus to its string representation
func HealthStatusString(h HealthStatus) string {
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

// INetwork defines the interface for network operations.
// This interface allows for mocking in tests and dependency injection.
type INetwork interface {
	// GetName returns the network name (e.g., "eth-mainnet")
	GetName() string

	// GetHandlerType returns the handler type string (e.g., "evm", "beacon-chain")
	GetHandlerType() string

	// GetHandler returns the network handler
	GetHandler() networklib.NetworkHandler

	// SetHandler sets the network's handler
	SetHandler(handler networklib.NetworkHandler) error

	// GetProviders returns the map of providers for this network
	GetProviders() map[string]*Provider

	// StartHealthcheck starts the health check goroutine
	StartHealthcheck()

	// Stop signals the network to stop all background goroutines
	Stop()
}

// IProvider defines the interface for provider operations.
// This interface allows for mocking in tests and dependency injection.
type IProvider interface {
	// GetHttpUrl returns the provider's HTTP URL
	GetHttpUrl() string

	// GetHost returns the provider's host (e.g., "provider.example.com:443")
	GetHost() string

	// GetName returns the provider's display name (extracted from domain)
	GetName() string

	// GetPriority returns the provider's priority (0-8, lower is better)
	GetPriority() int

	// Available returns true if the provider is available and healthy
	Available() bool

	// IsAvailableWithWarning returns true if the provider is available but in warning state
	IsAvailableWithWarning() bool

	// Healthy returns true if the provider's latest health check was healthy
	Healthy() bool

	// Warning returns true if the provider's latest health check was warning
	Warning() bool

	// AuthClient returns the authentication client for this provider
	AuthClient() auth.IAuthClient

	// SetAuthClient sets the authentication client for this provider
	SetAuthClient(authClient auth.IAuthClient)

	// GetUpstream returns the Caddy reverse proxy upstream for this provider
	GetUpstream() *reverseproxy.Upstream

	// SafeGetScore returns the current watcher score for the provider
	SafeGetScore() *ws.Score

	// SafeUpdateScore updates the watcher score for the provider
	SafeUpdateScore(score *ws.Score)
}
