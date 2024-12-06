package modules

import (
	"net/url"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	"github.com/DIN-center/din-caddy-plugins/lib/auth/siwe"
	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"go.uber.org/zap"
)

type provider struct {
	HttpUrl      string
	path         string
	host         string
	Headers      map[string]string
	upstream     *reverseproxy.Upstream
	httpClient   *din_http.HTTPClient
	logger       *zap.Logger
	failures     int
	successes    int
	healthStatus HealthStatus // 0 = Healthy, 1 = Warning, 2 = Unhealthy
	Priority     int
	quit         chan struct{}

	// Registry Configuration Values
	Methods []*string            `json:"methods"`
	Auth    *siwe.SIWEClientAuth `json:"auth"`

	consecutiveHealthyChecks int
}

func NewProvider(urlStr string) (*provider, error) {
	url, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	p := &provider{
		HttpUrl:                  urlStr,
		host:                     url.Host,
		Headers:                  make(map[string]string),
		consecutiveHealthyChecks: 0,
	}
	return p, nil
}

// Available indicates whether the Caddy upstream is available, and
// whether the provider's healthchecks indicate the upstream is healthy.
func (p *provider) Available() bool {
	return p.upstream.Available() && p.Healthy()
}

func (p *provider) IsAvailableWithWarning() bool {
	return p.upstream.Available() && p.Warning()
}

func (p *provider) AuthClient() auth.IAuthClient {
	if p.Auth == nil {
		return nil
	}
	return p.Auth
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

func (p *provider) markPingWarning() {
	p.successes = 0
	p.failures = 0
	p.healthStatus = Warning
}

// markPingSuccess records a successful healthcheck, and if the success count exceeds the healthcheck
// threshold marks the upstream as healthy
func (p *provider) markPingSuccess(hcThreshold int) {
	p.successes++
	if p.healthStatus == Unhealthy && p.successes > hcThreshold {
		p.failures = 0
		p.healthStatus = Healthy
	}
}

func (p *provider) markHealthy(hcThreshold int) {
	if p.healthStatus == Unhealthy {
		p.consecutiveHealthyChecks++
		if p.consecutiveHealthyChecks > hcThreshold {
			p.healthStatus = Healthy
			p.consecutiveHealthyChecks = 0
		}
		return
	}
	p.consecutiveHealthyChecks = 0
	p.healthStatus = Healthy
}

func (p *provider) markWarning() {
	p.healthStatus = Warning
	p.consecutiveHealthyChecks = 0
}

func (p *provider) markUnhealthy() {
	p.healthStatus = Unhealthy
	p.consecutiveHealthyChecks = 0
}

// Healthy returns True if the node is passing healthchecks, False otherwise
func (p *provider) Healthy() bool {
	if p.healthStatus == Healthy {
		return true
	} else {
		return false
	}
}

// Warning returns True if the node is returning warning in healthchecks, False otherwise
func (p *provider) Warning() bool {
	if p.healthStatus == Warning {
		return true
	} else {
		return false
	}
}
