package modules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"go.uber.org/zap"

	prom "github.com/openrelayxyz/din-caddy-plugins/lib/prometheus"
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
	logger *zap.Logger
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
	// Get upstreamWrappers from context
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
	var upstreamWrappers []*upstreamWrapper
	if v, ok := repl.Get(DinUpstreamsContextKey); ok {
		upstreamWrappers = v.([]*upstreamWrapper)
	}

	// Select upstream based on request
	res := d.selector.Select(pool, r, rw)
	for _, upstreamWrapper := range upstreamWrappers {
		// If the upstream is found in the upstreamWrappers, set the path and headers for the request
		if res == upstreamWrapper.upstream {
			r.URL.RawPath = upstreamWrapper.path
			r.URL.Path, _ = url.PathUnescape(r.URL.RawPath)
			for k, v := range upstreamWrapper.Headers {
				r.Header.Add(k, v)
			}
			if upstreamWrapper.Auth != nil {
				if err := upstreamWrapper.Auth.Sign(r); err != nil {
					d.logger.Error("error signing request", zap.String("err", err.Error()))
				}
			}
			break
		}
	}

	// Read request body for passing to metric middleware
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil
	}
	// Increment prometheus metric based on request data
	// Ran as a go routine to reduce latency on the client request to the provider
	go d.handleRequestMetric(body, r.RequestURI, r.Host, res.Dial)

	// Set request body back to original state
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	return res
}

// handleRequestMetric increments prometheus metric based on request data passed in
func (d *DinSelect) handleRequestMetric(bodyBytes []byte, service string, hostName string, provider string) {
	fmt.Println(hostName)
	// First extract method data from body
	// define struct to hold request data
	var requestBody struct {
		Method string `json:"method,omitempty"`
	}
	err := json.Unmarshal(bodyBytes, &requestBody)
	if err != nil {
		fmt.Printf("Error decoding request body: %v", http.StatusBadRequest)
	}
	var method string
	if requestBody.Method != "" {
		method = requestBody.Method
	}
	service = strings.TrimPrefix(service, "/")

	// Increment prometheus metric based on request data
	prom.DinRequestCount.WithLabelValues(service, method, provider, hostName).Inc()
}

func (d *DinSelect) UnmarshalCaddyfile(dispenser *caddyfile.Dispenser) error {
	return nil
}
