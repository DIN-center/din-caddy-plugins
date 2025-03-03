package modules

import (
	"net/url"
	"sync"
	"time"

	"errors"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	"github.com/DIN-center/din-caddy-plugins/lib/auth/siwe"
	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"go.uber.org/zap"
)

type provider struct {
	HttpUrl    string
	path       string
	host       string
	Headers    map[string]string
	upstream   *reverseproxy.Upstream
	httpClient *din_http.HTTPClient
	logger     *zap.Logger
	Priority   int

	// Registry Configuration Values
	Methods []*string            `json:"methods"`
	Auth    *siwe.SIWEClientAuth `json:"auth"`

	consecutiveUnhealthyChecks int
	blockHistory               []blockHistoryEntry
	mu                         sync.RWMutex
}

type blockHistoryEntry struct {
	blockNumber int64
	statusCode  HealthStatus
	timestamp   *time.Time
}

func NewProvider(urlStr string) (*provider, error) {
	if urlStr == "" {
		return nil, errors.New("empty URL")
	}

	url, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	// Additional validation
	if url.Host == "" {
		return nil, errors.New("invalid URL: missing host")
	}

	if url.Scheme != "http" && url.Scheme != "https" {
		return nil, errors.New("invalid URL: scheme must be http or https")
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

func (p *provider) IsAvailableWithWarning() bool {
	return p.upstream.Available() && p.Warning()
}

func (p *provider) AuthClient() auth.IAuthClient {
	if p.Auth == nil {
		return nil
	}
	return p.Auth
}

// Healthy returns True if the node is passing healthchecks, False otherwise
func (p *provider) Healthy() bool {
	latestBlockEntry := p.getLatestBlockEntry()
	if latestBlockEntry == nil {
		return false
	}
	if latestBlockEntry.statusCode == Healthy {
		return true
	} else {
		return false
	}
}

// Warning returns True if the node is returning warning in healthchecks, False otherwise
func (p *provider) Warning() bool {
	latestBlockEntry := p.getLatestBlockEntry()
	if latestBlockEntry == nil {
		return false
	}
	if latestBlockEntry.statusCode == Warning {
		return true
	} else {
		return false
	}
}

// BlockHistory returns a copy of the provider's block history
func (p *provider) BlockHistory() []blockHistoryEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()
	history := make([]blockHistoryEntry, len(p.blockHistory))
	copy(history, p.blockHistory)
	return history
}

// AddBlockEntry adds a new block entry to the history, maintaining the configured history size
func (p *provider) AddBlockEntry(block int64, status HealthStatus, blockHistorySize int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	entry := blockHistoryEntry{
		blockNumber: block,
		statusCode:  status,
		timestamp:   &now,
	}
	p.blockHistory = append(p.blockHistory, entry)
	if len(p.blockHistory) > blockHistorySize {
		p.blockHistory = p.blockHistory[1:]
	}
}

func (p *provider) getLatestHealthyBlockEntry() *blockHistoryEntry {
	if len(p.blockHistory) == 0 {
		return nil
	}
	for i := len(p.blockHistory) - 1; i >= 0; i-- {
		if p.blockHistory[i].statusCode == Healthy {
			return &p.blockHistory[i]
		}
	}
	return nil
}

func (p *provider) getLatestBlockEntry() *blockHistoryEntry {
	if len(p.blockHistory) == 0 {
		return nil
	}
	return &p.blockHistory[len(p.blockHistory)-1]
}
