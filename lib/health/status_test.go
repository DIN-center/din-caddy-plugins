package health

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealthStatusValues(t *testing.T) {
	assert.Equal(t, HealthStatus(0), Healthy)
	assert.Equal(t, HealthStatus(1), Warning)
	assert.Equal(t, HealthStatus(2), Unhealthy)
}

func TestHealthStatusString(t *testing.T) {
	tests := []struct {
		status   HealthStatus
		expected string
	}{
		{Healthy, "healthy"},
		{Warning, "warning"},
		{Unhealthy, "unhealthy"},
		{HealthStatus(99), "unknown"},
		{HealthStatus(-1), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}
