package modules

import (
	"net/http"
	"net/url"
	"strings"

	networklib "github.com/DIN-center/din-caddy-plugins/lib/network"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"go.uber.org/zap"
	// prom "github.com/DIN-center/din-caddy-plugins/lib/prometheus"
)

var (
	// Initializations of extended Caddy Module Interface Guards
	// https://caddyserver.com/docs/extending-caddy

	// Din Select Module
	_ caddy.Module      = (*DinSelect)(nil)
	_ caddy.Provisioner = (*DinSelect)(nil)
)

type DinSelect struct {
	selector reverseproxy.Selector
	logger   *zap.Logger
}

// CaddyModule returns the Caddy module information.
func (DinSelect) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.din_reverse_proxy_policy",
		New: func() caddy.Module { return new(DinSelect) },
	}
}

// Provision() is called by Caddy to prepare the selector for use.
// It is called only once, when the server is starting.
func (d *DinSelect) Provision(context caddy.Context) error {
	d.logger = context.Logger(d)

	selector := &reverseproxy.HeaderHashSelection{Field: "Din-Session-Id"}
	selector.Provision(context)
	d.selector = selector
	return nil
}

// Select() is called by Caddy reverse proxy dynamic upstream selecting process to select an upstream based on the request.
// It is called for each request.
func (d *DinSelect) Select(pool reverseproxy.UpstreamPool, r *http.Request, rw http.ResponseWriter) *reverseproxy.Upstream {
	// Check if the selector is nil
	if d.selector == nil {
		d.logger.Error("selector is nil")
		return nil
	}

	// Get providers and request context
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
	var providers map[string]*provider
	if v, ok := repl.Get(DinUpstreamsContextKey); ok {
		providers = v.(map[string]*provider)
	}

	// Get request context to determine if this is a REST API
	var reqContext *RequestProcessor
	if v, ok := repl.Get(RequestProcessorKey); ok {
		reqContext = v.(*RequestProcessor)
	}

	// Select upstream based on request using the header hash selector
	selectedUpstream := d.selector.Select(pool, r, rw)

	// Apply provider-specific configuration (path, headers, auth)
	for _, provider := range providers {
		// If the upstream is found in the providers, set the path and headers for the request
		if selectedUpstream == provider.upstream {
			d.applyProviderConfiguration(provider, r, rw, repl, reqContext)
			break
		}
	}

	return selectedUpstream
}

// TODO: Can we clean this up?
// applyProviderConfiguration applies provider-specific settings to the request
func (d *DinSelect) applyProviderConfiguration(provider *provider, r *http.Request, rw http.ResponseWriter, repl *caddy.Replacer, reqContext *RequestProcessor) {
	// Handle path configuration with generic REST API support
	currentPath := r.URL.Path

	// For REST APIs, strip the network prefix from the path
	// e.g., "/eth-beacon-mainnet/eth/v1/beacon/genesis" -> "/eth/v1/beacon/genesis"
	// This works for any REST API handler (beacon chain, future REST APIs, etc.)
	if reqContext != nil && reqContext.Type == networklib.RequestTypeREST {
		pathSegments := strings.Split(strings.TrimPrefix(currentPath, "/"), "/")
		if len(pathSegments) > 1 {
			// Remove the first segment (network name) and rebuild path
			strippedPath := "/" + strings.Join(pathSegments[1:], "/")
			currentPath = strippedPath
		}
	}

	// Set the final path based on request type and provider configuration
	if reqContext != nil && reqContext.Type == networklib.RequestTypeREST {
		// Parse the original provider URL to extract components
		if _, err := url.Parse(provider.HttpUrl); err == nil {
			// We can still parse and use the URL components for other purposes
			// but we should NOT set scheme/host/user on the request URL
			// as this confuses Caddy's reverse proxy

			// Combine provider base path with processed request path
			if provider.path != "" && provider.path != "/" {
				combinedPath := strings.TrimSuffix(provider.path, "/") + currentPath
				r.URL.Path = combinedPath
			} else {
				r.URL.Path = currentPath
			}
			r.URL.RawPath = ""
		} else {
			// Fallback to standard path handling if URL parsing fails
			d.logger.Error("Failed to parse provider URL", zap.String("provider_url", provider.HttpUrl), zap.Error(err))
			r.URL.Path = currentPath
		}
	} else if provider.path != "" {
		// For RPC providers with configured paths, use the configured provider path
		r.URL.RawPath = provider.path
		r.URL.Path, _ = url.PathUnescape(r.URL.RawPath)
	} else {
		// For RPC providers without configured paths, use the current path
		r.URL.Path = currentPath
		r.URL.RawPath = ""
	}

	// Apply headers
	for k, v := range provider.Headers {
		r.Header.Add(k, v)
	}

	// Apply authentication
	if provider.Auth != nil {
		if err := provider.Auth.Sign(r); err != nil {
			d.logger.Error("error signing request", zap.String("err", err.Error()))
		}
	}

	// Set provider info header
	if v := r.Header.Get(DinProviderInfo); v != "" {
		rw.Header().Set(DinProviderInfo, provider.host)
	}
	repl.Set(RequestProviderKey, provider.host)
}

func (d *DinSelect) UnmarshalCaddyfile(dispenser *caddyfile.Dispenser) error {
	return nil
}
