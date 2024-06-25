package modules

import (
	"net/url"

	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	din_http "github.com/openrelayxyz/din-caddy-plugins/lib/http"
)

type provider struct {
	HttpUrl    string `json:"http.url"`
	path       string
	host       string
	Headers    map[string]string
	upstream   *reverseproxy.Upstream
	httpClient *din_http.HTTPClient
	Priority   int

	failures     int
	successes    int
	healthStatus HealthStatus // 0 = Healthy, 1 = Warning, 2 = Unhealthy
	quit         chan struct{}
}

func NewProvider(urlStr string) (*provider, error) {
	url, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	p := &provider{
		HttpUrl: urlStr,
		host:    url.Host,
		Headers: make(map[string]string),
	}
	return p, nil
}

// Available indicates whether the Caddy upstream is available, and
// whether the provider's healthchecks indicate the upstream is healthy.
func (p *provider) Available() bool {
	return p.upstream.Available() && p.Healthy()
}

// markPingFailure records the failure, and if the failure count exceeds the healthcheck threshold
// marks the upstream as unhealthy
func (p *provider) markPingFailure(hcThreshold int) {
	p.failures++
	p.successes = 0
	if p.healthStatus == Healthy && p.failures > hcThreshold {
		p.healthStatus = Unhealthy
	}
}

// markPingSuccess records a successful healthcheck, and if the success count exceeds the healthcheck
// threshold marks the upsteram as healthy
func (p *provider) markPingSuccess(hcThreshold int) {
	p.successes++
	if p.healthStatus == Unhealthy && p.successes > hcThreshold {
		p.failures = 0
		p.healthStatus = Healthy
	}
}

func (p *provider) markHealthy() {
	p.healthStatus = Healthy
}

func (p *provider) markUnhealthy() {
	p.healthStatus = Unhealthy
}

// Healthy returns True if the node is passing healthchecks, False otherwise
func (p *provider) Healthy() bool {
	if p.healthStatus == Healthy {
		return true
	} else {
		return false
	}
}
