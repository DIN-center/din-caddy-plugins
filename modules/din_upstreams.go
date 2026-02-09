package modules

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"go.uber.org/zap"
)

// Global registry for sharing network configurations between modules
var (
	globalNetworkRegistry = make(map[string]*network)
	globalNetworkMutex    sync.RWMutex
)

// RegisterNetwork registers a network configuration in the global registry
func RegisterNetwork(name string, net *network) {
	globalNetworkMutex.Lock()
	defer globalNetworkMutex.Unlock()
	globalNetworkRegistry[name] = net
}

// GetNetwork retrieves a network configuration from the global registry
func GetNetwork(name string) (*network, bool) {
	globalNetworkMutex.RLock()
	defer globalNetworkMutex.RUnlock()
	net, exists := globalNetworkRegistry[name]
	return net, exists
}

// Compile-time check for interface implementations
var (
	_ caddy.Provisioner           = (*DinUpstreams)(nil)
	_ reverseproxy.UpstreamSource = (*DinUpstreams)(nil)
	_ caddyfile.Unmarshaler       = (*DinUpstreams)(nil)
)

type DinUpstreams struct {
	logger *zap.Logger
}

// CaddyModule returns the Caddy module information.
func (DinUpstreams) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.upstreams.din_reverse_proxy_policy",
		New: func() caddy.Module { return new(DinUpstreams) },
	}
}

// Provision sets up the upstream source.
func (d *DinUpstreams) Provision(ctx caddy.Context) error {
	d.logger = ctx.Logger(d)
	return nil
}

// GetUpstreams returns the possible upstream endpoints for the request.
func (d *DinUpstreams) GetUpstreams(r *http.Request) ([]*reverseproxy.Upstream, error) {
	var providers map[string]*provider

	// Get upstreams from the replacer context
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
	if v, ok := repl.Get(DinUpstreamsContextKey); ok {
		providers = v.(map[string]*provider)
	}

	if providers == nil {
		// Extract network name from request path
		networkName := d.extractNetworkName(r.URL.Path)
		d.logger.Warn("Providers not available from replacer. Retrieving from network object.",
			zap.String("network", networkName),
		)
		if networkName == "" {
			return nil, fmt.Errorf("no network name found in path")
		}

		// Get network configuration from global registry
		networkConfig, exists := GetNetwork(networkName)
		if !exists {
			return nil, fmt.Errorf("network %s not found in registry", networkName)
		}
		providers = networkConfig.Providers
	}

	// Check for providers excluded due to method-not-found errors on prior attempts
	var excludedProviders map[string]struct{}
	if v, ok := repl.Get(DinExcludedProvidersContextKey); ok {
		excludedProviders = v.(map[string]struct{})
	}

	// Convert providers to upstreams based on priority and health status
	upstreamPool := d.buildUpstreamPool(providers, excludedProviders)

	return upstreamPool, nil
}

// extractNetworkName extracts the network name from the request path
// For JSON-RPC: /optimism-mainnet/... -> "optimism-mainnet"
// For REST: /eth-beacon-mainnet/eth/v1/... -> "eth-beacon-mainnet"
func (d *DinUpstreams) extractNetworkName(path string) string {
	// Remove leading slash and split by "/"
	fullPath := strings.TrimPrefix(path, "/")
	if fullPath == "" {
		return ""
	}

	pathSegments := strings.Split(fullPath, "/")
	if len(pathSegments) == 0 {
		return ""
	}

	// First segment is always the network name
	networkName := pathSegments[0]

	return networkName
}

// buildUpstreamPool converts providers to upstreams with priority + health based selection.
// excludedProviders contains hosts to skip (e.g., from method-not-found retries). May be nil.
func (d *DinUpstreams) buildUpstreamPool(providers map[string]*provider, excludedProviders map[string]struct{}) []*reverseproxy.Upstream {
	upstreamPool := make([]*reverseproxy.Upstream, 0)

	// Select upstream based on priority. If no upstreams are available, pass along all upstreams
	for priority := 0; priority < MaxPriority; priority++ {
		for _, p := range providers {
			if _, excluded := excludedProviders[p.host]; excluded {
				continue
			}
			if p.Priority == priority && p.Available() {
				upstreamPool = append(upstreamPool, p.upstream)
			}
		}
		if len(upstreamPool) > 0 {
			break
		}
	}

	// TODO: set config based on customer's request header to go to a specific provider/upstream
	// Didn't find any based on priority, available, find all upstreams that are in warning status by priority.
	if len(upstreamPool) == 0 {
		for priority := 0; priority < MaxPriority; priority++ {
			for _, p := range providers {
				if _, excluded := excludedProviders[p.host]; excluded {
					continue
				}
				if p.Priority == priority && p.IsAvailableWithWarning() {
					upstreamPool = append(upstreamPool, p.upstream)
				}
			}
			if len(upstreamPool) > 0 {
				break
			}
		}
	}

	return upstreamPool
}

func (d *DinUpstreams) UnmarshalCaddyfile(dispenser *caddyfile.Dispenser) error {
	return nil
}
