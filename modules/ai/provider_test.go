package ai

import (
	"sync"
	"testing"
	"time"

	"github.com/DIN-center/din-caddy-plugins/lib/health"
	"github.com/stretchr/testify/assert"
)

func TestNewAIProvider(t *testing.T) {
	tests := []struct {
		name    string
		pName   string
		url     string
		wantErr bool
		host    string
		path    string
	}{
		{
			name:  "valid openai url",
			pName: "openai-gpt4o",
			url:   "https://api.openai.com/v1/chat/completions",
			host:  "api.openai.com",
			path:  "/v1/chat/completions",
		},
		{
			name:  "valid anthropic url",
			pName: "anthropic-sonnet",
			url:   "https://api.anthropic.com/v1/messages",
			host:  "api.anthropic.com",
			path:  "/v1/messages",
		},
		{
			name:    "invalid url",
			pName:   "bad",
			url:     "://invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewAIProvider(tt.pName, tt.url)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.pName, p.Name)
			assert.Equal(t, tt.host, p.Host)
			assert.Equal(t, tt.path, p.Path)
			assert.Equal(t, health.Healthy, p.healthStatus)
			assert.NotNil(t, p.Headers)
			assert.Equal(t, DefaultTTFTWindowSize, p.ttftWindowSize)
		})
	}
}

func TestHealthStatusString(t *testing.T) {
	assert.Equal(t, "healthy", health.Healthy.String())
	assert.Equal(t, "warning", health.Warning.String())
	assert.Equal(t, "unhealthy", health.Unhealthy.String())
	assert.Equal(t, "unknown", health.HealthStatus(99).String())
}

func TestProviderHealthTransitions(t *testing.T) {
	t.Run("ping failure transitions to unhealthy after threshold", func(t *testing.T) {
		p := mustProvider(t)
		assert.Equal(t, health.Healthy, p.HealthStatus())

		// Failures at threshold should NOT transition
		for i := 0; i <= DefaultHCThreshold; i++ {
			p.MarkPingFailure()
		}
		assert.Equal(t, health.Unhealthy, p.HealthStatus())
	})

	t.Run("ping success recovers from unhealthy after threshold", func(t *testing.T) {
		p := mustProvider(t)
		setProviderHealth(p, health.Unhealthy)
		assert.Equal(t, health.Unhealthy, p.HealthStatus())

		for i := 0; i <= DefaultHCThreshold; i++ {
			p.MarkPingSuccess()
		}
		assert.Equal(t, health.Healthy, p.HealthStatus())
	})

	t.Run("ping warning sets warning state", func(t *testing.T) {
		p := mustProvider(t)
		p.MarkPingWarning()
		assert.Equal(t, health.Warning, p.HealthStatus())
	})

	t.Run("ping success from warning is immediate", func(t *testing.T) {
		p := mustProvider(t)
		setProviderHealth(p, health.Warning)
		assert.Equal(t, health.Warning, p.HealthStatus())
		p.MarkPingSuccess()
		assert.Equal(t, health.Healthy, p.HealthStatus())
	})
}

func TestProviderAvailability(t *testing.T) {
	p := mustProvider(t)

	assert.True(t, p.IsHealthy())
	assert.True(t, p.IsAvailable())

	setProviderHealth(p, health.Warning)
	assert.False(t, p.IsHealthy())
	assert.True(t, p.IsAvailable())

	setProviderHealth(p, health.Unhealthy)
	assert.False(t, p.IsHealthy())
	assert.False(t, p.IsAvailable())
}

func TestProviderTTFT(t *testing.T) {
	t.Run("avg ttft with no measurements returns 0", func(t *testing.T) {
		p := mustProvider(t)
		assert.Equal(t, time.Duration(0), p.AvgTTFT())
		assert.Equal(t, 0, p.TTFTCount())
	})

	t.Run("avg ttft with measurements", func(t *testing.T) {
		p := mustProvider(t)
		p.RecordTTFT(100 * time.Millisecond)
		p.RecordTTFT(200 * time.Millisecond)
		p.RecordTTFT(300 * time.Millisecond)

		assert.Equal(t, 3, p.TTFTCount())
		assert.Equal(t, 200*time.Millisecond, p.AvgTTFT())
	})

	t.Run("ttft window rolls over", func(t *testing.T) {
		p := mustProvider(t)
		p.ttftWindowSize = 3

		p.RecordTTFT(100 * time.Millisecond)
		p.RecordTTFT(200 * time.Millisecond)
		p.RecordTTFT(300 * time.Millisecond)
		// Window full, next record should evict oldest
		p.RecordTTFT(400 * time.Millisecond)

		assert.Equal(t, 3, p.TTFTCount())
		// (200 + 300 + 400) / 3 = 300ms
		assert.Equal(t, 300*time.Millisecond, p.AvgTTFT())
	})
}

func TestProviderConcurrentAccess(t *testing.T) {
	p := mustProvider(t)
	var wg sync.WaitGroup
	iterations := 100

	// Concurrent health state writes
	wg.Add(3)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			p.MarkPingSuccess()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			p.MarkPingFailure()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			p.RecordTTFT(100 * time.Millisecond)
		}
	}()

	// Concurrent reads
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = p.HealthStatus()
			_ = p.IsHealthy()
			_ = p.IsAvailable()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = p.AvgTTFT()
			_ = p.TTFTCount()
		}
	}()

	wg.Wait()
	// If we get here without a race detector panic, the mutex is working
}

func TestProviderHealthThresholdBoundary(t *testing.T) {
	t.Run("exactly threshold failures stays healthy, threshold+1 transitions to unhealthy", func(t *testing.T) {
		p := mustProvider(t)
		assert.Equal(t, health.Healthy, p.HealthStatus())

		// DefaultHCThreshold (3) failures: condition is `failures > hcThreshold`,
		// so exactly 3 should NOT transition.
		for i := 0; i < DefaultHCThreshold; i++ {
			p.MarkPingFailure()
		}
		assert.Equal(t, health.Healthy, p.HealthStatus(), "exactly %d failures should remain Healthy", DefaultHCThreshold)

		// The (threshold+1)th failure crosses the boundary.
		p.MarkPingFailure()
		assert.Equal(t, health.Unhealthy, p.HealthStatus(), "%d failures should transition to Unhealthy", DefaultHCThreshold+1)
	})

	t.Run("exactly threshold successes stays unhealthy, threshold+1 transitions to healthy", func(t *testing.T) {
		p := mustProvider(t)
		setProviderHealth(p, health.Unhealthy)
		assert.Equal(t, health.Unhealthy, p.HealthStatus())

		// DefaultHCThreshold (3) successes: condition is `successes > hcThreshold`,
		// so exactly 3 should NOT recover.
		for i := 0; i < DefaultHCThreshold; i++ {
			p.MarkPingSuccess()
		}
		assert.Equal(t, health.Unhealthy, p.HealthStatus(), "exactly %d successes should remain Unhealthy", DefaultHCThreshold)

		// The (threshold+1)th success crosses the boundary.
		p.MarkPingSuccess()
		assert.Equal(t, health.Healthy, p.HealthStatus(), "%d successes should transition to Healthy", DefaultHCThreshold+1)
	})
}

func mustProvider(t *testing.T) *AIProvider {
	t.Helper()
	p, err := NewAIProvider("test-provider", "https://api.example.com/v1/chat/completions")
	assert.NoError(t, err)
	return p
}
