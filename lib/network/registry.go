// lib/network/registry.go
package network

import (
	"fmt"
	"sync"

	"go.uber.org/zap"

	"github.com/DIN-center/din-caddy-plugins/lib/logger"
)

// HandlerConstructor is a function that creates a new handler instance
type HandlerConstructor func(config *NetworkConfig) (NetworkHandler, error)

// HandlerRegistry manages available network handlers
type HandlerRegistry struct {
	mu           sync.RWMutex
	constructors map[string]HandlerConstructor
	handlers     map[string]NetworkHandler
}

// NewHandlerRegistry creates a new handler registry
func NewHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{
		constructors: make(map[string]HandlerConstructor),
		handlers:     make(map[string]NetworkHandler),
	}
}

// RegisterHandler registers a new handler constructor for a network type
func (r *HandlerRegistry) RegisterHandler(networkType string, constructor HandlerConstructor) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.constructors[networkType]; exists {
		// Handler already registered, this is safe to ignore
		return nil
	}

	r.constructors[networkType] = constructor
	return nil
}

// handlerCacheKey creates a compound cache key from network name and type
func handlerCacheKey(networkName, networkType string) string {
	return networkName + ":" + networkType
}

// GetHandler retrieves or creates a handler for the given network type
func (r *HandlerRegistry) GetHandler(networkType string, config *NetworkConfig) (NetworkHandler, error) {
	cacheKey := handlerCacheKey(config.Name, networkType)

	r.mu.RLock()
	// Check if handler already exists
	if handler, exists := r.handlers[cacheKey]; exists {
		if config.Logger != nil {
			config.Logger.Debug("Returning cached handler from registry",
				zap.String("network", config.Name),
				zap.String("type", networkType))
		}
		r.mu.RUnlock()
		return handler, nil
	}
	r.mu.RUnlock()

	// Create new handler
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if handler, exists := r.handlers[cacheKey]; exists {
		if config.Logger != nil {
			config.Logger.Debug("Returning cached handler from registry (after lock)",
				zap.String("network", config.Name),
				zap.String("type", networkType))
		}
		return handler, nil
	}

	constructor, exists := r.constructors[networkType]
	if !exists {
		return nil, fmt.Errorf("no handler registered for network type '%s'", networkType)
	}

	// Ensure logger is available
	if config.Logger == nil {
		// This should not happen in production, but we'll create a default logger
		// to prevent nil pointer dereferences
		zapLogger, _ := zap.NewProduction()
		config.Logger = logger.NewLoggerClient(zapLogger, "production")
	}

	handler, err := constructor(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create handler for network type '%s': %w", networkType, err)
	}

	// Validate ChainID format for the specific network type
	if config.ChainID != "" {
		if err := handler.ValidateChainID(config.ChainID); err != nil {
			return nil, fmt.Errorf("invalid chain ID '%s' for network type '%s': %w", config.ChainID, networkType, err)
		}
	}

	if err := handler.Initialize(config); err != nil {
		return nil, fmt.Errorf("failed to initialize handler for network type '%s': %w", networkType, err)
	}

	r.handlers[cacheKey] = handler

	if config.Logger != nil {
		config.Logger.Debug("Created and cached new handler in registry",
			zap.String("network", config.Name),
			zap.String("type", networkType),
			zap.String("handler_type", handler.GetType()))
	}

	return handler, nil
}

// ListHandlers returns all registered handler types
func (r *HandlerRegistry) ListHandlers() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.constructors))
	for handlerType := range r.constructors {
		types = append(types, handlerType)
	}
	return types
}

// RemoveHandler removes a specific handler from the registry
func (r *HandlerRegistry) RemoveHandler(networkName, networkType string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cacheKey := handlerCacheKey(networkName, networkType)
	delete(r.handlers, cacheKey)
	return nil
}

// ShutdownAll clears all active handlers
func (r *HandlerRegistry) ShutdownAll() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Clear the handlers map
	r.handlers = make(map[string]NetworkHandler)
	return nil
}

// HandlerInfo provides metadata about a registered handler
type HandlerInfo struct {
	Type        string
	Name        string
	Description string
}

// GetHandlerInfo returns metadata about a handler
func (r *HandlerRegistry) GetHandlerInfo(networkType string) (*HandlerInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	constructor, exists := r.constructors[networkType]
	if !exists {
		return nil, fmt.Errorf("no handler registered for network type '%s'", networkType)
	}

	// Create a temporary handler to get its metadata
	tempHandler, err := constructor(&NetworkConfig{Type: networkType})
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary handler: %w", err)
	}

	return &HandlerInfo{
		Type: tempHandler.GetType(),
		Name: tempHandler.GetName(),
	}, nil
}

// Global handler registry instance
var DefaultRegistry = NewHandlerRegistry()
