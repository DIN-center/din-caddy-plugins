package oauth2

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	"go.uber.org/zap"
)

type OAuth2Config struct {
	ClientID           string
	ClientSecret       string
	TokenURL           string
	RefreshIntervalSec int // Default: 240 seconds
	Scope              string
}

type OAuth2Client struct {
	config     OAuth2Config
	httpClient *http.Client
	logger     *zap.Logger

	mu           sync.RWMutex
	currentToken *tokenResponse
	tokenExpiry  time.Time

	stopCh chan struct{}
	done   chan struct{}
	err    error
}

type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	Scope            string `json:"scope,omitempty"`
	IDToken          string `json:"id_token,omitempty"`
}

// NewOAuth2Client creates a new OAuth2 client with the given configuration
func NewOAuth2Client(config OAuth2Config) *OAuth2Client {
	if config.RefreshIntervalSec == 0 {
		config.RefreshIntervalSec = 240 // Default to 4 minutes
	}

	if config.Scope == "" {
		config.Scope = "openid"
	}

	return &OAuth2Client{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		stopCh: make(chan struct{}),
		done:   make(chan struct{}),
	}
}

// Start begins the token refresh loop
func (c *OAuth2Client) Start(logger *zap.Logger) error {
	c.logger = logger

	// Get initial token
	if err := c.refreshToken(); err != nil {
		return fmt.Errorf("failed to get initial token: %w", err)
	}

	// Start refresh goroutine
	go c.refreshLoop()

	return nil
}

// Error returns any current error state
func (c *OAuth2Client) Error() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.err
}

// GetToken returns the current auth token information
func (c *OAuth2Client) GetToken(params map[string]interface{}) (auth.AuthToken, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.err != nil {
		return auth.AuthToken{}, c.err
	}

	if c.currentToken == nil {
		return auth.AuthToken{}, errors.New("no token available")
	}

	// Check if token is still valid
	if time.Now().After(c.tokenExpiry) {
		return auth.AuthToken{}, auth.ErrSessionExpired
	}

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", c.currentToken.AccessToken),
	}

	expiry := auth.UnixTime(c.tokenExpiry)

	return auth.AuthToken{
		Headers:    headers,
		Expiration: &expiry,
	}, nil
}

// Sign adds the OAuth2 Bearer token to the request
func (c *OAuth2Client) Sign(req *http.Request) error {
	token, err := c.GetToken(nil)
	if err != nil {
		return err
	}

	// Check if token is available for use
	if err := token.Use(); err != nil {
		return err
	}

	// Add headers to request
	for key, value := range token.Headers {
		req.Header.Set(key, value)
	}

	return nil
}

// Stop gracefully shuts down the OAuth2 client
func (c *OAuth2Client) Stop() {
	close(c.stopCh)
	<-c.done
}

// refreshLoop continuously refreshes the token at the configured interval
func (c *OAuth2Client) refreshLoop() {
	defer close(c.done)

	// Calculate refresh interval with 30 second buffer
	refreshInterval := time.Duration(c.config.RefreshIntervalSec-30) * time.Second
	if refreshInterval < 30*time.Second {
		refreshInterval = 30 * time.Second
	}

	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := c.refreshToken(); err != nil {
				c.logger.Error("failed to refresh OAuth2 token", zap.Error(err))
				c.mu.Lock()
				c.err = err
				c.mu.Unlock()
			}
		case <-c.stopCh:
			return
		}
	}
}

// refreshToken fetches a new access token using client credentials
func (c *OAuth2Client) refreshToken() error {
	// Prepare request body
	data := url.Values{}
	data.Set("client_id", c.config.ClientID)
	data.Set("client_secret", c.config.ClientSecret)
	data.Set("grant_type", "client_credentials")
	data.Set("scope", c.config.Scope)

	req, err := http.NewRequest("POST", c.config.TokenURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute token request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var tokenResp tokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return fmt.Errorf("failed to parse token response: %w", err)
	}

	// Validate response
	if tokenResp.AccessToken == "" {
		return errors.New("received empty access token")
	}

	// Update stored token
	c.mu.Lock()
	c.currentToken = &tokenResp
	// Use configured refresh interval instead of token's expires_in
	c.tokenExpiry = time.Now().Add(time.Duration(c.config.RefreshIntervalSec) * time.Second)
	c.err = nil
	c.mu.Unlock()

	if c.logger != nil {
		c.logger.Debug("OAuth2 token refreshed successfully",
			zap.String("token_type", tokenResp.TokenType),
			zap.Time("expiry", c.tokenExpiry))
	}

	return nil
}
