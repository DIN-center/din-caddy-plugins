package network

import (
	"container/list"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"golang.org/x/net/publicsuffix"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	ws "github.com/DIN-center/din-caddy-plugins/lib/watcherscore"
)

// Provider represents an RPC provider endpoint with health tracking and authentication.
type Provider struct {
	HttpUrl  string
	Path     string
	Query    string // URL query string (e.g., "apikey=xxx&foo=bar")
	Host     string
	Headers  map[string]string
	Upstream *reverseproxy.Upstream
	Logger   *logger.LoggerClient
	Priority int
	Name     string

	// Registry Configuration Values
	Methods map[string]struct{} `json:"methods"`

	// Auth client (SIWE, OIDC, or other auth types via factory)
	authClient auth.IAuthClient

	// Watcher Score
	score   *ws.Score    // Immutable score object
	scoreMu sync.RWMutex // Mutex to protect the access to the score object

	ConsecutiveUnhealthyChecks int
	BlockHistory               *list.List
	Mu                         sync.RWMutex
}

// BlockHistoryEntry tracks block data for a provider
type BlockHistoryEntry struct {
	BlockNumber  int64
	BlockHash    string
	HealthStatus HealthStatus
	Timestamp    *time.Time
}

// NewProvider creates a new provider from a URL string
func NewProvider(urlStr string) (*Provider, error) {
	if urlStr == "" {
		return nil, errors.New("empty URL")
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	// Additional validation
	if parsedURL.Host == "" {
		return nil, errors.New("invalid URL: missing host")
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, errors.New("invalid URL: scheme must be http or https")
	}

	p := &Provider{
		HttpUrl:      urlStr,
		Host:         parsedURL.Host,
		Path:         parsedURL.Path,
		Query:        parsedURL.RawQuery,
		Name:         SafeExtractMainDomainWithPSL(parsedURL),
		Headers:      make(map[string]string),
		BlockHistory: list.New(),
		score:        ws.EmptyScore,
	}
	return p, nil
}

// GetHttpUrl returns the provider's HTTP URL
func (p *Provider) GetHttpUrl() string {
	return p.HttpUrl
}

// GetHost returns the provider's host
func (p *Provider) GetHost() string {
	return p.Host
}

// GetName returns the provider's display name
func (p *Provider) GetName() string {
	return p.Name
}

// GetPriority returns the provider's priority
func (p *Provider) GetPriority() int {
	return p.Priority
}

// GetUpstream returns the Caddy reverse proxy upstream
func (p *Provider) GetUpstream() *reverseproxy.Upstream {
	return p.Upstream
}

// Available indicates whether the Caddy upstream is available, and
// whether the provider's healthchecks indicate the upstream is healthy.
func (p *Provider) Available() bool {
	return p.Upstream.Available() && p.Healthy()
}

// IsAvailableWithWarning returns true if available but in warning state
func (p *Provider) IsAvailableWithWarning() bool {
	return p.Upstream.Available() && p.Warning()
}

// AuthClient returns the auth client for this provider
func (p *Provider) AuthClient() auth.IAuthClient {
	return p.authClient
}

// SetAuthClient sets the generic auth client for this provider
func (p *Provider) SetAuthClient(authClient auth.IAuthClient) {
	p.authClient = authClient
}

// Healthy returns True if the node is passing healthchecks, False otherwise
func (p *Provider) Healthy() bool {
	latestBlockEntry := p.GetLatestBlockEntry()
	if latestBlockEntry == nil {
		return false
	}
	return latestBlockEntry.HealthStatus == Healthy
}

// Warning returns True if the node is returning warning in healthchecks, False otherwise
func (p *Provider) Warning() bool {
	latestBlockEntry := p.GetLatestBlockEntry()
	if latestBlockEntry == nil {
		return false
	}
	return latestBlockEntry.HealthStatus == Warning
}

// GetBlockHistory returns a copy of the provider's block history
func (p *Provider) GetBlockHistory() []BlockHistoryEntry {
	p.Mu.RLock()
	defer p.Mu.RUnlock()

	// Keep this critical nil check
	if p.BlockHistory == nil {
		return []BlockHistoryEntry{}
	}

	history := make([]BlockHistoryEntry, 0, p.BlockHistory.Len())

	for e := p.BlockHistory.Front(); e != nil; e = e.Next() {
		entry := e.Value.(BlockHistoryEntry)
		// Create a deep copy of the entry
		entryCopy := BlockHistoryEntry{
			BlockNumber:  entry.BlockNumber,
			HealthStatus: entry.HealthStatus,
		}
		// Only copy the timestamp if it's not nil
		if entry.Timestamp != nil {
			timeCopy := *entry.Timestamp
			entryCopy.Timestamp = &timeCopy
		}
		history = append(history, entryCopy)
	}

	return history
}

// AddBlockEntry adds a new block entry to the history, maintaining the configured history size
func (p *Provider) AddBlockEntry(block int64, status HealthStatus, blockHistorySize int) {
	if p == nil {
		return
	}

	p.Mu.Lock()
	defer p.Mu.Unlock()

	if p.BlockHistory == nil {
		p.BlockHistory = list.New()
		// Exit early if initialization failed
		if p.BlockHistory == nil {
			return
		}
	}

	now := time.Now()
	entry := BlockHistoryEntry{
		BlockNumber:  block,
		HealthStatus: status,
		Timestamp:    &now,
	}

	if p.BlockHistory != nil {
		p.BlockHistory.PushBack(entry)

		// Trim the list if it exceeds the history size
		for p.BlockHistory.Len() > blockHistorySize {
			if p.BlockHistory.Front() != nil {
				p.BlockHistory.Remove(p.BlockHistory.Front())
			}
		}
	}
}

// GetLatestHealthyBlockEntry returns the most recent healthy block entry
func (p *Provider) GetLatestHealthyBlockEntry() *BlockHistoryEntry {
	p.Mu.RLock()
	defer p.Mu.RUnlock()

	// Keep critical nil checks
	if p.BlockHistory == nil {
		return nil
	}

	if p.BlockHistory.Len() == 0 {
		return nil
	}

	// Start from the back (most recent) and find the first healthy entry
	for e := p.BlockHistory.Back(); e != nil; e = e.Prev() {
		entry := e.Value.(BlockHistoryEntry)
		if entry.HealthStatus == Healthy {
			return &entry
		}
	}
	return nil
}

// GetLatestBlockEntry returns the most recent block entry
func (p *Provider) GetLatestBlockEntry() *BlockHistoryEntry {
	p.Mu.RLock()
	defer p.Mu.RUnlock()

	// Keep critical nil checks
	if p.BlockHistory == nil {
		return nil
	}

	if p.BlockHistory.Len() == 0 {
		return nil
	}

	entry := p.BlockHistory.Back().Value.(BlockHistoryEntry)
	return &entry
}

// SafeGetScore returns the current score for the provider with a read lock to prevent reading while writing.
func (p *Provider) SafeGetScore() *ws.Score {
	p.scoreMu.RLock()
	defer p.scoreMu.RUnlock()
	return p.score
}

// SafeUpdateScore updates the score for the provider with a write lock to prevent writing while reading.
func (p *Provider) SafeUpdateScore(score *ws.Score) {
	p.scoreMu.Lock()
	defer p.scoreMu.Unlock()
	p.score = score
}

// SafeExtractMainDomainWithPSL extracts the main domain from the URL using the Public Suffix List.
// If this fails, it returns the hostname.
func SafeExtractMainDomainWithPSL(u *url.URL) string {
	// Get the eTLD+1 (effective TLD plus one label)
	domain, err := publicsuffix.EffectiveTLDPlusOne(u.Hostname())
	if err != nil {
		return u.Hostname()
	}

	// Split and get the main part (before the TLD)
	parts := strings.Split(domain, ".")
	if len(parts) > 0 {
		return parts[0]
	}

	return domain
}
