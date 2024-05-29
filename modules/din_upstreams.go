package modules

import (
	"fmt"
	"time"
	"net"
	"net/http"
	"strings"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
)

var (
	// Initializations of extended Caddy Module Interface Guards
	// https://caddyserver.com/docs/extending-caddy

	// Din Upstream Module
	_ caddy.Module                = (*DinUpstreams)(nil)
	_ reverseproxy.UpstreamSource = (*DinUpstreams)(nil)
)

type DinUpstreams struct{}

type upstreamWrapper struct {
	HttpUrl   string `json:"http.url"`
	path      string
	Headers   map[string]string
	upstream  *reverseproxy.Upstream
	Priority  int
	HCRPCMethod string `json:"healthcheck.rpc.method"`
	failures  int
	successes int
	healthy   bool
	quit      chan struct{}
}

func (u *upstreamWrapper) Available() bool {
	return u.upstream.Available() && u.Healthy()
}

func (u *upstreamWrapper) StartHealthchecks() {
	u.quit = make(chan struct{})
	c := &http.Client{Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConnsPerHost:   16,
		MaxIdleConns:          16,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}}
	u.healthy = true
	ticker := time.NewTicker(time.Second * 5)
	go func() {
		for i :=0 ; ; i++ {
			select {
			case <-u.quit:
				ticker.Stop()
				return
			case <-ticker.C:
				rpcCallString := fmt.Sprintf(`{"jsonrpc":"2.0","id":%v,"method":"%v","params":[]}`, i, u.HCRPCMethod)
				req, err := http.NewRequest("POST", u.HttpUrl, strings.NewReader(rpcCallString))
				if err != nil {
					u.markFailure(err.Error())
					continue
				}
				for k, v := range u.Headers {
					req.Header.Add(k, v)
				}
				req.Header.Add("Content-Type", "application/json")
				res, err := c.Do(req)
				if err != nil {
					u.markFailure(err.Error())
					continue
				}
				if res != nil && res.Body != nil {
					res.Body.Close()
				}
				if res.StatusCode > 399 {
					u.markFailure(fmt.Sprintf("status: %v", res.StatusCode))
					continue
				}
				u.markSuccess()
			}
		}
	}()
}

func (u *upstreamWrapper) markFailure(reason string) {
	u.failures++
	u.successes = 0
	if u.failures > 2 && u.healthy {
		u.healthy = false
		// fmt.Printf("Marking %v as unhealthy: %v\n", u.HttpUrl, reason)
	}
}
func (u *upstreamWrapper) markSuccess() {
	u.successes++
	if u.successes > 2 && !u.healthy {
		u.failures = 0
		u.healthy = true
		// fmt.Printf("Marking %v as healthy\n", u.HttpUrl)
	}
}

func (u *upstreamWrapper) Healthy() bool {
	return u.healthy
}

// CaddyModule returns the Caddy module information.
func (DinUpstreams) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.upstreams.din_reverse_proxy_policy",
		New: func() caddy.Module { return new(DinUpstreams) },
	}
}

// GetUpstreams returns the possible upstreams for the request.
func (d *DinUpstreams) GetUpstreams(r *http.Request) ([]*reverseproxy.Upstream, error) {
	var upstreamWrappers []*upstreamWrapper

	// Get upstreams from the replacer context
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
	if v, ok := repl.Get(DinUpstreamsContextKey); ok {
		upstreamWrappers = v.([]*upstreamWrapper)
	}

	res := make([]*reverseproxy.Upstream, 0, len(upstreamWrappers))

	// Select upstream based on priority. If no upstreams are available, pass along all upstreams
	for priority := 0; priority < len(upstreamWrappers); priority++ {
		for _, u := range upstreamWrappers {
			if u.Priority == priority && u.Available() {
				res = append(res, u.upstream)
			}
		}
		if len(res) > 0 {
			break
		}
	}
	if len(res) == 0 {
		// Didn't find any based on priority, available, pass along all upstreams
		for _, u := range upstreamWrappers {
			res = append(res, u.upstream)
		}
	}
	return res, nil
}

func (d *DinUpstreams) UnmarshalCaddyfile(dispenser *caddyfile.Dispenser) error {
	return nil
}
