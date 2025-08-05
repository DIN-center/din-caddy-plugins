package modules

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBitcoinEsplora_OAuth2Configuration(t *testing.T) {
	// Test that OAuth2 configuration is properly extracted from custom_config
	network := &network{
		Name:        "bitcoin-esplora-mainnet",
		HandlerType: BitcoinEsploraHandler,
		ChainId:     "bitcoin:mainnet",
		CustomConfig: map[string]interface{}{
			"oauth2_client_id":        "test-client-id",
			"oauth2_client_secret":    "test-client-secret",
			"oauth2_token_url":        "https://auth.example.com/token",
			"oauth2_refresh_interval": 180,
		},
	}

	// Create a provider with OAuth2 enabled
	provider := &provider{
		HttpUrl:       "https://enterprise.blockstream.info",
		OAuth2Enabled: true,
		Headers:       make(map[string]string),
		Priority:      0,
	}

	// Configure OAuth2 for the provider
	err := network.configureOAuth2ForProvider(provider)
	require.NoError(t, err)

	// Verify OAuth2 client was created
	assert.NotNil(t, provider.authClient)
}

func TestBitcoinEsplora_OAuth2MissingConfiguration(t *testing.T) {
	testCases := []struct {
		name         string
		customConfig map[string]interface{}
		expectError  string
	}{
		{
			name:         "no_custom_config",
			customConfig: nil,
			expectError:  "OAuth2 enabled but no custom_config found",
		},
		{
			name: "missing_client_id",
			customConfig: map[string]interface{}{
				"oauth2_client_secret": "secret",
				"oauth2_token_url":     "https://auth.example.com/token",
			},
			expectError: "oauth2_client_id not found in custom_config",
		},
		{
			name: "missing_client_secret",
			customConfig: map[string]interface{}{
				"oauth2_client_id": "client",
				"oauth2_token_url": "https://auth.example.com/token",
			},
			expectError: "oauth2_client_secret not found in custom_config",
		},
		{
			name: "missing_token_url",
			customConfig: map[string]interface{}{
				"oauth2_client_id":     "client",
				"oauth2_client_secret": "secret",
			},
			expectError: "oauth2_token_url not found in custom_config",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			network := &network{
				Name:         "bitcoin-esplora-mainnet",
				HandlerType:  BitcoinEsploraHandler,
				ChainId:      "bitcoin:mainnet",
				CustomConfig: tc.customConfig,
			}

			provider := &provider{
				HttpUrl:       "https://enterprise.blockstream.info",
				OAuth2Enabled: true,
				Headers:       make(map[string]string),
			}

			// Configure OAuth2 should fail
			err := network.configureOAuth2ForProvider(provider)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectError)
		})
	}
}

func TestBitcoinEsplora_OAuth2DefaultRefreshInterval(t *testing.T) {
	network := &network{
		Name:        "bitcoin-esplora-mainnet",
		HandlerType: BitcoinEsploraHandler,
		ChainId:     "bitcoin:mainnet",
		CustomConfig: map[string]interface{}{
			"oauth2_client_id":     "test-client-id",
			"oauth2_client_secret": "test-client-secret",
			"oauth2_token_url":     "https://auth.example.com/token",
			// No refresh interval specified, should default to 240
		},
	}

	provider := &provider{
		HttpUrl:       "https://enterprise.blockstream.info",
		OAuth2Enabled: true,
		Headers:       make(map[string]string),
	}

	// Configure OAuth2
	err := network.configureOAuth2ForProvider(provider)
	require.NoError(t, err)

	// Verify OAuth2 client was created
	assert.NotNil(t, provider.authClient)
	// Default refresh interval is 240 seconds (4 minutes)
}

func TestBitcoinEsplora_NoOAuth2ForProvider(t *testing.T) {
	network := &network{
		Name:        "bitcoin-esplora-mainnet",
		HandlerType: BitcoinEsploraHandler,
		ChainId:     "bitcoin:mainnet",
		// No custom config
	}

	provider := &provider{
		HttpUrl:       "https://blockstream.info",
		OAuth2Enabled: false, // OAuth2 not enabled for this provider
		Headers:       make(map[string]string),
	}

	// Should not configure OAuth2 since it's not enabled
	// authClient should remain nil
	assert.Nil(t, provider.authClient)
}