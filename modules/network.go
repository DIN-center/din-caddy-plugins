package modules

import (
	internalnetwork "github.com/DIN-center/din-caddy-plugins/internal/network"
)

// network is a type alias for backward compatibility with existing code.
// The canonical implementation is in internal/network.Network.
type network = internalnetwork.Network

// NewNetwork creates a new network with the given name and handler type.
// This is a convenience function that delegates to internal/network.NewNetwork.
var NewNetwork = internalnetwork.NewNetwork

// caddyfileConfigFlags is a type alias for backward compatibility.
type caddyfileConfigFlags = internalnetwork.CaddyfileConfigFlags

// RoundUpToInterval is a wrapper for testing.
var RoundUpToInterval = internalnetwork.RoundUpToInterval
