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

	prom "github.com/openrelayxyz/din-caddy-plugins/lib/prometheus"
)

type DinSelect struct {
	selector reverseproxy.Selector
}

// CaddyModule returns the Caddy module information.
func (DinSelect) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.dinupstreams",
		New: func() caddy.Module { return new(DinSelect) },
	}
}

func (d *DinSelect) Provision(context caddy.Context) error {
	selector := &reverseproxy.HeaderHashSelection{Field: "Din-Session-Id"}
	selector.Provision(context)
	d.selector = selector
	return nil
}

func (d *DinSelect) Select(pool reverseproxy.UpstreamPool, r *http.Request, rw http.ResponseWriter) *reverseproxy.Upstream {
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
	var mus []*metaUpstream
	if v, ok := repl.Get("din.internal.upstreams"); ok {
		mus = v.([]*metaUpstream)
	}

	res := d.selector.Select(pool, r, rw)
	for _, mu := range mus {
		if res == mu.upstream {
			r.URL.RawPath = mu.path
			r.URL.Path, _ = url.PathUnescape(r.URL.RawPath)
			for k, v := range mu.Headers {
				r.Header.Add(k, v)
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
	go d.handleRequestMetric(body, r.RequestURI, res.Dial)

	// Set request body back to original state
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	return res
}

// Increments prometheus metric based on request data passed in
func (d *DinSelect) handleRequestMetric(bodyBytes []byte, service string, provider string) {
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
	prom.DinRequestCount.WithLabelValues(service, method, provider).Inc()
}

func (d *DinSelect) UnmarshalCaddyfile(dispenser *caddyfile.Dispenser) error {
	return nil
}
