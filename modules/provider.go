package modules

import (
	"net/url"

	internalnetwork "github.com/DIN-center/din-caddy-plugins/internal/network"
)

// provider is a type alias for backward compatibility with existing code.
// The canonical implementation is in internal/network.Provider.
type provider = internalnetwork.Provider

// NewProvider creates a new provider from a URL string.
// This is a convenience function that delegates to internal/network.NewProvider.
var NewProvider = internalnetwork.NewProvider

// blockHistoryEntry is a type alias for backward compatibility.
type blockHistoryEntry = internalnetwork.BlockHistoryEntry

// BlockHistoryEntry is the exported type alias for test visibility.
type BlockHistoryEntry = internalnetwork.BlockHistoryEntry

// safeExtractMainDomainWithPSL is a wrapper that delegates to internal/network.
func safeExtractMainDomainWithPSL(u *url.URL) string {
	return internalnetwork.SafeExtractMainDomainWithPSL(u)
}
