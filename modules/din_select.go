package modules

import (
	"bytes"
	"io"
	"net/http"
	"net/url"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
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
	selector := &reverseproxy.HeaderHashSelection{Field: "Din-Session-Id"}
	selector.Provision(context)
	d.selector = selector
	return nil
}

// Select() is called by Caddy reverse proxy dynamic upstream selecting process to select an upstream based on the request.
// It is called for each request.
func (d *DinSelect) Select(pool reverseproxy.UpstreamPool, r *http.Request, rw http.ResponseWriter) *reverseproxy.Upstream {
	// Get providers from context
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
	var providers map[string]*provider
	if v, ok := repl.Get(DinUpstreamsContextKey); ok {
		providers = v.(map[string]*provider)
	}
	// Select upstream based on request
	selectedUpstream := d.selector.Select(pool, r, rw)

	for _, provider := range providers {
		// If the upstream is found in the providers, set the path and headers for the request
		if selectedUpstream == provider.upstream {
			r.URL.RawPath = provider.path
			r.URL.Path, _ = url.PathUnescape(r.URL.RawPath)
			for k, v := range provider.Headers {
				r.Header.Add(k, v)
			}
			break
		}
	}

	// if the request body is nil, return without setting the context for request metrics
	if r.Body == nil {
		return selectedUpstream
	}

	// Read request body for passing to metric middleware
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil
	}
	repl.Set(RequestProviderKey, selectedUpstream.Dial)
	repl.Set(RequestBodyKey, bodyBytes)

	// Set request body back to original state
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	return selectedUpstream
}

func (d *DinSelect) UnmarshalCaddyfile(dispenser *caddyfile.Dispenser) error {
	return nil
}
