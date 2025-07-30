package oauth2

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestOAuth2Client_NewClient(t *testing.T) {
	config := OAuth2Config{
		ClientID:           "test-client",
		ClientSecret:       "test-secret",
		TokenURL:           "https://auth.example.com/token",
		RefreshIntervalSec: 300,
		Scope:              "openid",
	}

	client := NewOAuth2Client(config)
	assert.NotNil(t, client)
	assert.Equal(t, config.ClientID, client.config.ClientID)
	assert.Equal(t, config.ClientSecret, client.config.ClientSecret)
	assert.Equal(t, config.TokenURL, client.config.TokenURL)
	assert.Equal(t, 300, client.config.RefreshIntervalSec)
}

func TestOAuth2Client_DefaultRefreshInterval(t *testing.T) {
	config := OAuth2Config{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		TokenURL:     "https://auth.example.com/token",
		// RefreshIntervalSec not set, should default to 240
	}

	client := NewOAuth2Client(config)
	assert.Equal(t, 240, client.config.RefreshIntervalSec)
}

func TestOAuth2Client_TokenRefresh(t *testing.T) {
	// Create a test server that returns a token
	tokenResponse := tokenResponse{
		AccessToken: "test-access-token",
		TokenType:   "Bearer",
		ExpiresIn:   300,
		Scope:       "openid",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		// Parse form data
		err := r.ParseForm()
		require.NoError(t, err)
		assert.Equal(t, "test-client", r.Form.Get("client_id"))
		assert.Equal(t, "test-secret", r.Form.Get("client_secret"))
		assert.Equal(t, "client_credentials", r.Form.Get("grant_type"))
		assert.Equal(t, "openid", r.Form.Get("scope"))

		// Return token
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tokenResponse)
	}))
	defer server.Close()

	config := OAuth2Config{
		ClientID:           "test-client",
		ClientSecret:       "test-secret",
		TokenURL:           server.URL,
		RefreshIntervalSec: 240,
		Scope:              "openid",
	}

	client := NewOAuth2Client(config)
	logger := zap.NewNop()

	// Start the client
	err := client.Start(logger)
	require.NoError(t, err)
	defer client.Stop()

	// Get token
	token, err := client.GetToken(nil)
	require.NoError(t, err)
	assert.Equal(t, "Bearer test-access-token", token.Headers["Authorization"])
	assert.NotNil(t, token.Expiration)
}

func TestOAuth2Client_Sign(t *testing.T) {
	// Create a test server that returns a token
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenResp := tokenResponse{
			AccessToken: "test-token-123",
			TokenType:   "Bearer",
			ExpiresIn:   300,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tokenResp)
	}))
	defer server.Close()

	config := OAuth2Config{
		ClientID:           "test-client",
		ClientSecret:       "test-secret",
		TokenURL:           server.URL,
		RefreshIntervalSec: 240,
	}

	client := NewOAuth2Client(config)
	logger := zap.NewNop()

	err := client.Start(logger)
	require.NoError(t, err)
	defer client.Stop()

	// Create a request to sign
	req, err := http.NewRequest("GET", "https://api.example.com/data", nil)
	require.NoError(t, err)

	// Sign the request
	err = client.Sign(req)
	require.NoError(t, err)

	// Check the Authorization header
	assert.Equal(t, "Bearer test-token-123", req.Header.Get("Authorization"))
}

func TestOAuth2Client_TokenExpiry(t *testing.T) {
	config := OAuth2Config{
		ClientID:           "test-client",
		ClientSecret:       "test-secret",
		TokenURL:           "https://auth.example.com/token",
		RefreshIntervalSec: 1, // 1 second for testing
	}

	client := NewOAuth2Client(config)
	
	// Manually set a token that's already expired
	client.currentToken = &tokenResponse{
		AccessToken: "expired-token",
		TokenType:   "Bearer",
	}
	client.tokenExpiry = time.Now().Add(-1 * time.Hour) // Expired 1 hour ago

	// Try to get token
	_, err := client.GetToken(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session expired")
}

func TestOAuth2Client_ErrorHandling(t *testing.T) {
	// Test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "invalid_client", "error_description": "Client authentication failed"}`))
	}))
	defer server.Close()

	config := OAuth2Config{
		ClientID:           "invalid-client",
		ClientSecret:       "invalid-secret",
		TokenURL:           server.URL,
		RefreshIntervalSec: 240,
	}

	client := NewOAuth2Client(config)
	logger := zap.NewNop()

	// Start should fail due to authentication error
	err := client.Start(logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token request failed with status 401")
}