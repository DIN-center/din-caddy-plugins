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

// cachedPool holds a snapshot of the upstream pool along with the health check
// version at which it was built. A stale version signals the pool must be rebuilt.
type cachedPool struct {
	version uint64
	pool    []*reverseproxy.Upstream
}

type DinUpstreams struct {
	logger    *zap.Logger
	poolCache sync.Map // map[string]*cachedPool, keyed by network name
}

// CaddyModule returns the Caddy module information.
func (*DinUpstreams) CaddyModule() caddy.ModuleInfo {
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

	// Check for providers excluded due to method-not-found errors on prior attempts.
	var excludedProviders map[string]struct{}
	if v, ok := repl.Get(DinExcludedProvidersContextKey); ok {
		excludedProviders = v.(map[string]struct{})
	}

	// Use the cached upstream pool when:
	//   - DinNetworkContextKey is set (only set when no method filter is active)
	//   - No providers are excluded (retries must bypass cache for proper failover)
	//
	// Known trade-off: if Caddy's internal circuit breaker trips a backend between
	// health check cycles, the cached pool may still include that upstream. Caddy's
	// selection policy re-checks Available() per upstream and will skip it, burning
	// a retry attempt. This is acceptable given the ~5-second health check window.
	if v, ok := repl.Get(DinNetworkContextKey); ok && len(excludedProviders) == 0 {
		net := v.(*network)
		currentVersion := net.HealthCheckVersion()

		if entry, loaded := d.poolCache.Load(net.Name); loaded {
			cached := entry.(*cachedPool)
			if cached.version == currentVersion {
				return cached.pool, nil
			}
		}

		pool := d.buildUpstreamPool(providers, nil)
		// Only cache non-empty pools. An empty pool means all providers are unhealthy;
		// Caddy's circuit breaker may recover before the next health check cycle, so
		// we must not cache the empty state.
		if len(pool) > 0 {
			d.poolCache.Store(net.Name, &cachedPool{version: currentVersion, pool: pool})
		}
		return pool, nil
	}

	return d.buildUpstreamPool(providers, excludedProviders), nil
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
