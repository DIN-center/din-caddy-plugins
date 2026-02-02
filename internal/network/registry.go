package network

import (
	"sync"
)

// Global registry for sharing network configurations between modules
var (
	globalNetworkRegistry = make(map[string]*Network)
	globalNetworkMutex    sync.RWMutex
)

// RegisterNetwork registers a network configuration in the global registry
func RegisterNetwork(name string, net *Network) {
	globalNetworkMutex.Lock()
	defer globalNetworkMutex.Unlock()
	globalNetworkRegistry[name] = net
}

// GetNetwork retrieves a network configuration from the global registry
func GetNetwork(name string) (*Network, bool) {
	globalNetworkMutex.RLock()
	defer globalNetworkMutex.RUnlock()
	net, exists := globalNetworkRegistry[name]
	return net, exists
}

// UnregisterNetwork removes a network from the global registry (used for testing)
func UnregisterNetwork(name string) {
	globalNetworkMutex.Lock()
	defer globalNetworkMutex.Unlock()
	delete(globalNetworkRegistry, name)
}

// ClearRegistry clears the global network registry (used for testing)
func ClearRegistry() {
	globalNetworkMutex.Lock()
	defer globalNetworkMutex.Unlock()
	globalNetworkRegistry = make(map[string]*Network)
}
