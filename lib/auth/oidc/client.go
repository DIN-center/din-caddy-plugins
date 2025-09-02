package oidc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
)

// OIDCClient implements the IAuthClient interface for OIDC/OAuth2 authentication
type OIDCClient struct {
	ClientID        string `json:"client_id"`
	ClientSecret    string `json:"client_secret"`
	TokenURL        string `json:"token_url"`
	Scope           string `json:"scope"`
	DurationSeconds int    `json:"duration_seconds,omitempty"` // Optional override for refresh interval

	httpClient *http.Client
	logger     *zap.Logger

	tokenMu        sync.RWMutex
	accessToken    string
	tokenExpiry    time.Time
	tokenExpiresIn int // Store the expires_in value from token response

	stopCh chan struct{}
	done   chan struct{}
	err    error
}

type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in,omitempty"`
	Scope            string `json:"scope,omitempty"`
	IDToken          string `json:"id_token,omitempty"`
}

// NewOIDCClient creates a new OIDC client
func NewOIDCClient(clientID, clientSecret, tokenURL string) *OIDCClient {
	return &OIDCClient{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     tokenURL,
		Scope:        "openid", // default scope
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Start initializes the OIDC client and begins the token refresh loop
func (c *OIDCClient) Start(logger *zap.Logger) error {
	c.logger = logger

	// Initialize HTTP client if not set
	if c.httpClient == nil {
		c.httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	// Set default scope if not provided
	if c.Scope == "" {
		c.Scope = "openid"
	}

	// Get initial token
	token, expiry, expiresIn, err := c.fetchToken()
	if err != nil {
		return fmt.Errorf("failed to get initial token: %w", err)
	}

	c.tokenMu.Lock()
	c.accessToken = token
	c.tokenExpiry = expiry
	c.tokenExpiresIn = expiresIn
	c.tokenMu.Unlock()

	c.stopCh = make(chan struct{})
	c.done = make(chan struct{})

	// Start refresh goroutine
	go c.refreshTokenLoop()

	return nil
}

// Error returns any current error state
func (c *OIDCClient) Error() error {
	c.tokenMu.RLock()
	defer c.tokenMu.RUnlock()
	return c.err
}

// GetToken returns the current auth token information
func (c *OIDCClient) GetToken(params map[string]interface{}) (auth.AuthToken, error) {
	c.tokenMu.RLock()
	defer c.tokenMu.RUnlock()

	if c.err != nil {
		return auth.AuthToken{}, c.err
	}

	if c.accessToken == "" {
		return auth.AuthToken{}, errors.New("no token available")
	}

	// Check if token is still valid
	if time.Now().After(c.tokenExpiry) {
		return auth.AuthToken{}, auth.ErrSessionExpired
	}

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", c.accessToken),
	}

	expiry := auth.UnixTime(c.tokenExpiry)

	return auth.AuthToken{
		Headers:    headers,
		Expiration: &expiry,
	}, nil
}

// Sign adds the OIDC Bearer token to the request
func (c *OIDCClient) Sign(req *http.Request) error {
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

// Stop gracefully shuts down the OIDC client
func (c *OIDCClient) Stop() {
	if c.stopCh != nil {
		close(c.stopCh)
		<-c.done
	}
}

// calculateRefreshTime determines when to refresh the token
func (c *OIDCClient) calculateRefreshTime(tokenExpiresIn int) time.Duration {
	// Default: use token's expires_in minus 1 minute buffer
	defaultRefreshTime := time.Duration(tokenExpiresIn-60) * time.Second

	// If DurationSeconds is configured
	if c.DurationSeconds > 0 {
		configuredRefreshTime := time.Duration(c.DurationSeconds) * time.Second

		// Validate: configured time must be less than token expiry
		if configuredRefreshTime < time.Duration(tokenExpiresIn)*time.Second {
			// Use configured duration minus 1 minute buffer
			refreshTime := configuredRefreshTime - (1 * time.Minute)
			if refreshTime < 30*time.Second {
				return 30 * time.Second // Minimum 30 seconds
			}
			return refreshTime
		} else {
			// Log warning and use default
			if c.logger != nil {
				c.logger.Warn("configured duration_seconds exceeds token expiry, using default",
					zap.Int("configured", c.DurationSeconds),
					zap.Int("token_expires_in", tokenExpiresIn))
			}
		}
	}

	// Use default (token expires_in - 1 minute)
	if defaultRefreshTime < 30*time.Second {
		return 30 * time.Second // Minimum 30 seconds
	}
	return defaultRefreshTime
}

// refreshTokenLoop continuously refreshes the token before it expires
func (c *OIDCClient) refreshTokenLoop() {
	defer close(c.done)

	for {
		// Calculate next refresh time based on current token
		c.tokenMu.RLock()
		expiresIn := c.tokenExpiresIn
		currentExpiry := c.tokenExpiry
		c.tokenMu.RUnlock()

		// Determine effective lifetime to use (configured duration or token expiry)
		effectiveLifetime := expiresIn
		usingConfiguredDuration := false
		if c.DurationSeconds > 0 && c.DurationSeconds < expiresIn {
			effectiveLifetime = c.DurationSeconds
			usingConfiguredDuration = true
		}

		// Simple refresh strategy:
		// - For lifetimes >= 2 minutes: refresh 1 minute before expiry
		// - For shorter lifetimes: refresh at 50% of lifetime
		var refreshBuffer int
		if effectiveLifetime >= 120 {
			refreshBuffer = 60
		} else {
			refreshBuffer = effectiveLifetime / 2
		}

		// Calculate sleep duration
		var sleepDuration time.Duration
		if usingConfiguredDuration {
			// For configured duration, calculate from now
			sleepDuration = time.Duration(effectiveLifetime-refreshBuffer) * time.Second
			if c.logger != nil {
				c.logger.Debug("using configured duration for token refresh",
					zap.Int("duration_seconds", c.DurationSeconds),
					zap.Duration("sleep_duration", sleepDuration))
			}
		} else {
			// For token expiry, calculate from actual expiry time
			sleepDuration = time.Until(currentExpiry.Add(-time.Duration(refreshBuffer) * time.Second))
			if sleepDuration <= 0 {
				sleepDuration = 0 // Refresh immediately if already expired
			}
		}

		// Ensure minimum refresh interval
		if sleepDuration > 0 && sleepDuration < 5*time.Second {
			sleepDuration = 5 * time.Second
		}

		// Log warning if configured duration exceeds token lifetime
		if c.DurationSeconds > 0 && c.DurationSeconds >= expiresIn && c.logger != nil {
			c.logger.Warn("configured duration_seconds exceeds token expiry, using token expiry",
				zap.Int("configured", c.DurationSeconds),
				zap.Int("token_expires_in", expiresIn))
		}

		// Wait until refresh time
		select {
		case <-time.After(sleepDuration):
			// Time to refresh
		case <-c.stopCh:
			return
		}

		// Fetch new token
		token, expiry, expiresIn, err := c.fetchToken()
		if err != nil {
			if c.logger != nil {
				c.logger.Error("failed to refresh OIDC token", zap.Error(err))
			}
			c.tokenMu.Lock()
			c.err = err
			c.tokenMu.Unlock()

			// Retry after 30 seconds on error
			select {
			case <-time.After(30 * time.Second):
				continue
			case <-c.stopCh:
				return
			}
		}

		c.tokenMu.Lock()
		c.accessToken = token
		c.tokenExpiry = expiry
		c.tokenExpiresIn = expiresIn
		c.err = nil
		c.tokenMu.Unlock()

		if c.logger != nil {
			c.logger.Debug("OIDC token refreshed successfully",
				zap.Time("expiry", expiry),
				zap.Int("expires_in", expiresIn))
		}
	}
}

// fetchToken fetches a new access token using client credentials
func (c *OIDCClient) fetchToken() (string, time.Time, int, error) {
	var err error

	// Prepare request body
	data := url.Values{}
	data.Set("client_id", c.ClientID)
	data.Set("client_secret", c.ClientSecret)
	data.Set("grant_type", "client_credentials")
	data.Set("scope", c.Scope)

	req, err := http.NewRequest("POST", c.TokenURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return "", time.Time{}, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, 0, fmt.Errorf("failed to execute token request: %w", err)
	}

	defer func() {
		err = resp.Body.Close()
	}()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", time.Time{}, 0, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return "", time.Time{}, 0, fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var tokenResp tokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", time.Time{}, 0, fmt.Errorf("failed to parse token response: %w", err)
	}

	// Validate response
	if tokenResp.AccessToken == "" {
		return "", time.Time{}, 0, errors.New("received empty access token")
	}

	// Calculate expiry time using the expires_in from response
	expiry := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return tokenResp.AccessToken, expiry, tokenResp.ExpiresIn, err
}

// Ensure OIDCClient implements IAuthClient interface
var _ auth.IAuthClient = (*OIDCClient)(nil)
