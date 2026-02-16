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
	constructors map[HandlerType]HandlerConstructor
	handlers     map[string]NetworkHandler
}

// NewHandlerRegistry creates a new handler registry
func NewHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{
		constructors: make(map[HandlerType]HandlerConstructor),
		handlers:     make(map[string]NetworkHandler),
	}
}

// RegisterHandler registers a new handler constructor for a network type
func (r *HandlerRegistry) RegisterHandler(networkType HandlerType, constructor HandlerConstructor) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.constructors[networkType]; exists {
		// Handler already registered, this is safe to ignore
		return nil
	}

	r.constructors[networkType] = constructor
	return nil
}

// GetHandler retrieves or creates a handler for the given network type
func (r *HandlerRegistry) GetHandler(networkType HandlerType, config *NetworkConfig) (NetworkHandler, error) {
	r.mu.RLock()
	// Check if handler already exists
	if handler, exists := r.handlers[config.Name]; exists {
		// Validate that the cached handler matches the requested type
		if handler.GetType() != networkType {
			r.mu.RUnlock()
			return nil, fmt.Errorf("cached handler type mismatch for network '%s': cached type '%s', requested type '%s'",
				config.Name, handler.GetType(), networkType)
		}
		if config.Logger != nil {
			config.Logger.Debug("Returning cached handler from registry",
				zap.String("network", config.Name),
				zap.String("type", string(networkType)))
		}
		r.mu.RUnlock()
		return handler, nil
	}
	r.mu.RUnlock()

	// Create new handler
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if handler, exists := r.handlers[config.Name]; exists {
		// Validate that the cached handler matches the requested type
		if handler.GetType() != networkType {
			return nil, fmt.Errorf("cached handler type mismatch for network '%s': cached type '%s', requested type '%s'",
				config.Name, handler.GetType(), networkType)
		}
		if config.Logger != nil {
			config.Logger.Debug("Returning cached handler from registry (after lock)",
				zap.String("network", config.Name),
				zap.String("type", string(networkType)))
		}
		return handler, nil
	}

	constructor, exists := r.constructors[networkType]
	if !exists {
		return nil, fmt.Errorf("no handler registered for network type '%s'", string(networkType))
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

	r.handlers[config.Name] = handler

	if config.Logger != nil {
		config.Logger.Debug("Created and cached new handler in registry",
			zap.String("network", config.Name),
			zap.String("type", string(networkType)),
			zap.String("handler_type", string(handler.GetType())))
	}

	return handler, nil
}

// ListHandlers returns all registered handler types
func (r *HandlerRegistry) ListHandlers() []HandlerType {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]HandlerType, 0, len(r.constructors))
	for handlerType := range r.constructors {
		types = append(types, handlerType)
	}
	return types
}

// RemoveHandler removes a specific handler from the registry
func (r *HandlerRegistry) RemoveHandler(networkName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Just remove the handler from the map
	// No shutdown needed since we removed the Shutdown method
	delete(r.handlers, networkName)
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
	Type        HandlerType
	Name        string
	Description string
}

// GetHandlerInfo returns metadata about a handler
func (r *HandlerRegistry) GetHandlerInfo(networkType HandlerType) (*HandlerInfo, error) {
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
