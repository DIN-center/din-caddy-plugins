package modules

import (
	"net/http"
	"net/url"

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

	// Get network object to determine if this is a REST API
	var networkObj *network
	if v, ok := repl.Get("network_object"); ok {
		networkObj = v.(*network)
	}

	// Select upstream based on request using the header hash selector
	selectedUpstream := d.selector.Select(pool, r, rw)

	// Apply provider-specific configuration (path, headers, auth)
	for _, provider := range providers {
		// If the upstream is found in the providers, set the path and headers for the request
		if selectedUpstream == provider.upstream {
			d.applyProviderConfiguration(provider, r, rw, repl, networkObj)
			break
		}
	}

	return selectedUpstream
}

// applyProviderConfiguration applies provider-specific settings to the request
func (d *DinSelect) applyProviderConfiguration(provider *provider, r *http.Request, rw http.ResponseWriter, repl *caddy.Replacer, networkObj *network) {
	// Use the network handler to configure the request path
	if networkObj != nil && networkObj.handler != nil {
		networkName := networkObj.Name
		if err := networkObj.handler.ConfigureRequestPath(r, provider.path, networkName); err != nil {
			d.logger.Error("Failed to configure request path", 
				zap.String("network", networkName),
				zap.String("provider_path", provider.path),
				zap.Error(err))
		}
	} else {
		// Fallback for cases where handler is not available
		if provider.path != "" {
			r.URL.RawPath = provider.path
			r.URL.Path, _ = url.PathUnescape(r.URL.RawPath)
		}
	}

	// Apply headers
	for k, v := range provider.Headers {
		r.Header.Add(k, v)
	}

	// Apply authentication (supports both SIWE and OAuth2)
	if authClient := provider.AuthClient(); authClient != nil {
		if err := authClient.Sign(r); err != nil {
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
