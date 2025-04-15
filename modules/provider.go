package modules

import (
	"container/list"
	"net/url"
	"sync"
	"time"

	"errors"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	"github.com/DIN-center/din-caddy-plugins/lib/auth/siwe"
	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
)

type provider struct {
	HttpUrl    string
	path       string
	host       string
	Headers    map[string]string
	upstream   *reverseproxy.Upstream
	httpClient *din_http.HTTPClient
	logger     *logger.LoggerClient
	Priority   int

	// Registry Configuration Values
	Methods map[string]struct{}  `json:"methods"`
	Auth    *siwe.SIWEClientAuth `json:"auth"`

	consecutiveUnhealthyChecks int
	blockHistory               *list.List
	mu                         sync.RWMutex
}

type blockHistoryEntry struct {
	blockNumber  int64
	healthStatus HealthStatus
	timestamp    *time.Time
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
		HttpUrl:      urlStr,
		host:         url.Host,
		Headers:      make(map[string]string),
		blockHistory: list.New(),
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
	if latestBlockEntry.healthStatus == Healthy {
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
	if latestBlockEntry.healthStatus == Warning {
		return true
	} else {
		return false
	}
}

// BlockHistory returns a copy of the provider's block history
func (p *provider) BlockHistory() []blockHistoryEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Keep this critical nil check
	if p.blockHistory == nil {
		return []blockHistoryEntry{}
	}

	history := make([]blockHistoryEntry, 0, p.blockHistory.Len())

	for e := p.blockHistory.Front(); e != nil; e = e.Next() {
		entry := e.Value.(blockHistoryEntry)
		// Create a deep copy of the entry
		entryCopy := blockHistoryEntry{
			blockNumber:  entry.blockNumber,
			healthStatus: entry.healthStatus,
		}
		// Only copy the timestamp if it's not nil
		if entry.timestamp != nil {
			timeCopy := *entry.timestamp
			entryCopy.timestamp = &timeCopy
		}
		history = append(history, entryCopy)
	}

	return history
}

// AddBlockEntry adds a new block entry to the history, maintaining the configured history size
func (p *provider) AddBlockEntry(block int64, status HealthStatus, blockHistorySize int) {
	if p == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.blockHistory == nil {
		p.blockHistory = list.New()
		// Exit early if initialization failed
		if p.blockHistory == nil {
			return
		}
	}

	now := time.Now()
	entry := blockHistoryEntry{
		blockNumber:  block,
		healthStatus: status,
		timestamp:    &now,
	}

	if p.blockHistory != nil {
		p.blockHistory.PushBack(entry)

		// Trim the list if it exceeds the history size
		for p.blockHistory.Len() > blockHistorySize {
			if p.blockHistory.Front() != nil {
				p.blockHistory.Remove(p.blockHistory.Front())
			}
		}
	}
}

func (p *provider) getLatestHealthyBlockEntry() *blockHistoryEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Keep critical nil checks
	if p.blockHistory == nil {
		return nil
	}

	if p.blockHistory.Len() == 0 {
		return nil
	}

	// Start from the back (most recent) and find the first healthy entry
	for e := p.blockHistory.Back(); e != nil; e = e.Prev() {
		entry := e.Value.(blockHistoryEntry)
		if entry.healthStatus == Healthy {
			return &entry
		}
	}
	return nil
}

func (p *provider) getLatestBlockEntry() *blockHistoryEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Keep critical nil checks
	if p.blockHistory == nil {
		return nil
	}

	if p.blockHistory.Len() == 0 {
		return nil
	}

	entry := p.blockHistory.Back().Value.(blockHistoryEntry)
	return &entry
}
