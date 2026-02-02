package siwe

import (
	"fmt"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
)

const (
	// ConfigKeyURL is the auth endpoint URL
	ConfigKeyURL = "url"
	// ConfigKeySessions is the number of concurrent sessions
	ConfigKeySessions = "sessions"
	// ConfigKeySigner is the signing configuration
	ConfigKeySigner = "signer"
	// DefaultSessionCount is the default number of sessions
	DefaultSessionCount = 16
)

// Factory creates SIWE auth clients
type Factory struct{}

// Type returns the auth type name
func (f *Factory) Type() string {
	return "siwe"
}

// ValidateConfig validates the SIWE configuration
func (f *Factory) ValidateConfig(cfg auth.AuthConfig) error {
	// URL is required
	if _, ok := cfg[ConfigKeyURL]; !ok {
		return fmt.Errorf("%s is required", ConfigKeyURL)
	}

	// Sessions must be positive if specified
	if sessions, ok := cfg[ConfigKeySessions]; ok {
		switch v := sessions.(type) {
		case int:
			if v <= 0 {
				return fmt.Errorf("%s must be positive", ConfigKeySessions)
			}
		case int64:
			if v <= 0 {
				return fmt.Errorf("%s must be positive", ConfigKeySessions)
			}
		default:
			return fmt.Errorf("%s must be an integer", ConfigKeySessions)
		}
	}

	return nil
}

// Create creates a new SIWE auth client
func (f *Factory) Create(cfg auth.AuthConfig) (auth.IAuthClient, error) {
	url, ok := cfg[ConfigKeyURL].(string)
	if !ok {
		return nil, fmt.Errorf("%s must be a string", ConfigKeyURL)
	}

	sessionCount := DefaultSessionCount
	if sessions, ok := cfg[ConfigKeySessions]; ok {
		switch v := sessions.(type) {
		case int:
			sessionCount = v
		case int64:
			sessionCount = int(v)
		}
	}

	client := &SIWEClientAuth{
		ProviderURL:  url,
		SessionCount: sessionCount,
	}

	// Set signer if provided
	if signer, ok := cfg[ConfigKeySigner].(*SigningConfig); ok {
		client.Signer = signer
	}

	return client, nil
}

func init() {
	auth.Register(&Factory{})
}
