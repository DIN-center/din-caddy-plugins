package modules

import (
	"testing"

	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
)

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name   string
		urlstr string
		output *provider
		hasErr bool
	}{
		{
			name:   "passing localhost",
			urlstr: "http://localhost:8080",
			output: &provider{
				HttpUrl:  "http://localhost:8080",
				host:     "localhost:8080",
				path:     "",
				Headers:  make(map[string]string),
				Priority: 0,
			},
			hasErr: false,
		},
		{
			name:   "passing fullurl with key",
			urlstr: "https://eth.rpc.test.cloud:443/key",
			output: &provider{
				HttpUrl:  "https://eth.rpc.test.cloud:443/key",
				host:     "eth.rpc.test.cloud:443",
				Headers:  make(map[string]string),
				Priority: 0,
			},
			hasErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewProvider(tt.urlstr)
			if err != nil && !tt.hasErr {
				t.Errorf("urlToProviderObject() = %v, want %v", err, tt.hasErr)
			}
			if provider.HttpUrl != tt.output.HttpUrl {
				t.Errorf("HttpUrl = %v, want %v", provider.HttpUrl, tt.output.HttpUrl)
			}
			if provider.host != tt.output.host {
				t.Errorf("host = %v, want %v", provider.host, tt.output.host)
			}
			if provider.path != tt.output.path {
				t.Errorf("path = %v, want %v", provider.path, tt.output.path)
			}
			if len(provider.Headers) != len(tt.output.Headers) {
				t.Errorf("Headers length = %v, want %v", len(provider.Headers), len(tt.output.Headers))
			}
			if provider.Priority != tt.output.Priority {
				t.Errorf("Priority = %v, want %v", provider.Priority, tt.output.Priority)
			}
		})
	}
}

func TestAvailable(t *testing.T) {
	tests := []struct {
		name     string
		provider *provider
		output   bool
	}{
		{
			name: "Available with healthy upstream",
			provider: &provider{
				healthStatus: Healthy,
				upstream: &reverseproxy.Upstream{
					Dial: "localhost:8080",
				},
			},
			output: true,
		},
		{
			name: "Available with unhealthy upstream",
			provider: &provider{
				healthStatus: Unhealthy,
				upstream: &reverseproxy.Upstream{
					Dial: "localhost:8080",
				},
			},
			output: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.provider.Available() != tt.output {
				t.Errorf("Available() = %v, want %v", tt.provider.Available(), tt.output)
			}
		})
	}
}

func TestMarkPingFailure(t *testing.T) {
	tests := []struct {
		name     string
		hcThresh int
		provider *provider
		output   HealthStatus
	}{
		{
			name: "markPingFailure with 0 threshold",
			provider: &provider{
				failures:     0,
				successes:    0,
				healthStatus: Healthy,
			},
			hcThresh: 0,
			output:   Unhealthy,
		},
		{
			name: "markPingFailure with 1 threshold",
			provider: &provider{
				failures:     0,
				successes:    0,
				healthStatus: Healthy,
			},
			hcThresh: 1,
			output:   Healthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.provider.markPingFailure(tt.hcThresh)
			if tt.provider.healthStatus != tt.output {
				t.Errorf("markPingFailure() = %v, want %v", tt.provider.healthStatus, tt.output)
			}
		})
	}
}

func TestMarkPingSuccess(t *testing.T) {
	tests := []struct {
		name     string
		hcThresh int
		provider *provider
		output   HealthStatus
	}{
		{
			name: "markPingSuccess with 0 threshold",
			provider: &provider{
				failures:     0,
				successes:    0,
				healthStatus: Unhealthy,
			},
			hcThresh: 0,
			output:   Healthy,
		},
		{
			name: "markPingSuccess with 1 threshold",
			provider: &provider{
				failures:     0,
				successes:    0,
				healthStatus: Unhealthy,
			},
			hcThresh: 1,
			output:   Unhealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.provider.markPingSuccess(tt.hcThresh)
			if tt.provider.healthStatus != tt.output {
				t.Errorf("markPingSuccess() = %v, want %v", tt.provider.healthStatus, tt.output)
			}
		})
	}
}
