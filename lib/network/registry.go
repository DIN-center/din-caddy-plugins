// lib/network/registry.go
package network

import (
	"fmt"
	"sync"
)

// HandlerFactory is a function that creates a new handler instance
type HandlerFactory func(config *NetworkConfig) (NetworkHandler, error)

// HandlerRegistry manages available network handlers
type HandlerRegistry struct {
	mu        sync.RWMutex
	factories map[string]HandlerFactory
	handlers  map[string]NetworkHandler
}

// NewHandlerRegistry creates a new handler registry
func NewHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{
		factories: make(map[string]HandlerFactory),
		handlers:  make(map[string]NetworkHandler),
	}
}

// RegisterHandler registers a new handler factory for a network type
func (r *HandlerRegistry) RegisterHandler(networkType string, factory HandlerFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[networkType]; exists {
		// Handler already registered, this is safe to ignore
		return nil
	}

	r.factories[networkType] = factory
	return nil
}

// GetHandler retrieves or creates a handler for the given network type
func (r *HandlerRegistry) GetHandler(networkType string, config *NetworkConfig) (NetworkHandler, error) {
	r.mu.RLock()
	// Check if handler already exists
	if handler, exists := r.handlers[config.Name]; exists {
		r.mu.RUnlock()
		return handler, nil
	}
	r.mu.RUnlock()

	// Create new handler
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if handler, exists := r.handlers[config.Name]; exists {
		return handler, nil
	}

	factory, exists := r.factories[networkType]
	if !exists {
		return nil, fmt.Errorf("no handler registered for network type '%s'", networkType)
	}

	handler, err := factory(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create handler for network type '%s': %w", networkType, err)
	}

	if err := handler.Initialize(config); err != nil {
		return nil, fmt.Errorf("failed to initialize handler for network type '%s': %w", networkType, err)
	}

	r.handlers[config.Name] = handler
	return handler, nil
}

// ListHandlers returns all registered handler types
func (r *HandlerRegistry) ListHandlers() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.factories))
	for handlerType := range r.factories {
		types = append(types, handlerType)
	}
	return types
}

// ShutdownAll shuts down all active handlers
func (r *HandlerRegistry) ShutdownAll() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errors []error
	for name, handler := range r.handlers {
		if err := handler.Shutdown(); err != nil {
			errors = append(errors, fmt.Errorf("failed to shutdown handler '%s': %w", name, err))
		}
	}

	// Clear the handlers map
	r.handlers = make(map[string]NetworkHandler)

	if len(errors) > 0 {
		return fmt.Errorf("multiple shutdown errors: %v", errors)
	}
	return nil
}

// HandlerInfo provides metadata about a registered handler
type HandlerInfo struct {
	Type        string
	Name        string
	Version     string
	Description string
}

// GetHandlerInfo returns metadata about a handler
func (r *HandlerRegistry) GetHandlerInfo(networkType string) (*HandlerInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, exists := r.factories[networkType]
	if !exists {
		return nil, fmt.Errorf("no handler registered for network type '%s'", networkType)
	}

	// Create a temporary handler to get its metadata
	tempHandler, err := factory(&NetworkConfig{Type: networkType})
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary handler: %w", err)
	}

	return &HandlerInfo{
		Type:    tempHandler.GetType(),
		Name:    tempHandler.GetName(),
		Version: tempHandler.GetVersion(),
	}, nil
}

// Global handler registry instance
var DefaultRegistry = NewHandlerRegistry()
