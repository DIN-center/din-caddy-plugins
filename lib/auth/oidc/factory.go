package oidc

import (
	"fmt"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
)

const (
	// ConfigKeyURL is the token endpoint URL
	ConfigKeyURL = "url"
	// ConfigKeyClientID is the OAuth2 client ID
	ConfigKeyClientID = "client_id"
	// ConfigKeyClientSecret is the OAuth2 client secret
	ConfigKeyClientSecret = "client_secret"
	// ConfigKeyScope is the OAuth2 scope
	ConfigKeyScope = "scope"
	// ConfigKeyDurationSeconds is the optional refresh interval override
	ConfigKeyDurationSeconds = "duration_seconds"
	// DefaultScope is the default OAuth2 scope
	DefaultScope = "openid"
)

// Factory creates OIDC auth clients
type Factory struct{}

// Type returns the auth type name
func (f *Factory) Type() string {
	return "oidc"
}

// ValidateConfig validates the OIDC configuration
func (f *Factory) ValidateConfig(cfg auth.AuthConfig) error {
	// Required fields
	if _, ok := cfg[ConfigKeyURL]; !ok {
		return fmt.Errorf("%s is required", ConfigKeyURL)
	}
	if _, ok := cfg[ConfigKeyClientID]; !ok {
		return fmt.Errorf("%s is required", ConfigKeyClientID)
	}
	if _, ok := cfg[ConfigKeyClientSecret]; !ok {
		return fmt.Errorf("%s is required", ConfigKeyClientSecret)
	}

	// Validate duration_seconds if specified
	if duration, ok := cfg[ConfigKeyDurationSeconds]; ok {
		switch v := duration.(type) {
		case int:
			if v <= 0 {
				return fmt.Errorf("%s must be positive", ConfigKeyDurationSeconds)
			}
		case int64:
			if v <= 0 {
				return fmt.Errorf("%s must be positive", ConfigKeyDurationSeconds)
			}
		default:
			return fmt.Errorf("%s must be an integer", ConfigKeyDurationSeconds)
		}
	}

	return nil
}

// Create creates a new OIDC auth client
func (f *Factory) Create(cfg auth.AuthConfig) (auth.IAuthClient, error) {
	tokenURL, ok := cfg[ConfigKeyURL].(string)
	if !ok {
		return nil, fmt.Errorf("%s must be a string", ConfigKeyURL)
	}

	clientID, ok := cfg[ConfigKeyClientID].(string)
	if !ok {
		return nil, fmt.Errorf("%s must be a string", ConfigKeyClientID)
	}

	clientSecret, ok := cfg[ConfigKeyClientSecret].(string)
	if !ok {
		return nil, fmt.Errorf("%s must be a string", ConfigKeyClientSecret)
	}

	scope := DefaultScope
	if s, ok := cfg[ConfigKeyScope].(string); ok && s != "" {
		scope = s
	}

	client := &OIDCClient{
		TokenURL:     tokenURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scope:        scope,
	}

	// Set duration_seconds if provided
	if duration, ok := cfg[ConfigKeyDurationSeconds]; ok {
		switch v := duration.(type) {
		case int:
			client.DurationSeconds = v
		case int64:
			client.DurationSeconds = int(v)
		}
	}

	return client, nil
}

func init() {
	auth.Register(&Factory{})
}
