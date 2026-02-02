package auth

import (
	"fmt"
	"sync"
)

// AuthConfig holds the configuration for creating an auth client.
// Keys are auth-type specific (e.g., "url", "client_id", "client_secret" for OIDC).
type AuthConfig map[string]interface{}

// AuthFactory creates auth clients of a specific type.
type AuthFactory interface {
	// Type returns the auth type name (e.g., "siwe", "oidc")
	Type() string
	// Create creates a new auth client with the given configuration
	Create(cfg AuthConfig) (IAuthClient, error)
	// ValidateConfig validates the configuration before creation
	ValidateConfig(cfg AuthConfig) error
}

var (
	factoryMu sync.RWMutex
	factories = make(map[string]AuthFactory)
)

// Register registers an auth factory. Typically called from init().
func Register(f AuthFactory) {
	factoryMu.Lock()
	defer factoryMu.Unlock()
	factories[f.Type()] = f
}

// Create creates an auth client of the specified type with the given configuration.
func Create(authType string, cfg AuthConfig) (IAuthClient, error) {
	factoryMu.RLock()
	f, ok := factories[authType]
	factoryMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown auth type: %s (registered: %v)", authType, RegisteredTypes())
	}

	if err := f.ValidateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid %s config: %w", authType, err)
	}

	return f.Create(cfg)
}

// RegisteredTypes returns a list of registered auth type names.
func RegisteredTypes() []string {
	factoryMu.RLock()
	defer factoryMu.RUnlock()

	types := make([]string, 0, len(factories))
	for t := range factories {
		types = append(types, t)
	}
	return types
}

// GetFactory returns the factory for a given auth type, or nil if not registered.
func GetFactory(authType string) AuthFactory {
	factoryMu.RLock()
	defer factoryMu.RUnlock()
	return factories[authType]
}
