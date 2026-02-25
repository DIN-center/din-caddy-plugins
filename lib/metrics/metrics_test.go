package metrics

import "testing"

// testRecorder is a simple test implementation of HealthCheckRecorder.
type testRecorder struct {
	calls []struct {
		provider     string
		statusCode   string
		healthStatus string
	}
}

func (r *testRecorder) RecordHealthCheck(provider, statusCode, healthStatus string) {
	r.calls = append(r.calls, struct {
		provider     string
		statusCode   string
		healthStatus string
	}{provider, statusCode, healthStatus})
}

// Compile-time check.
var _ HealthCheckRecorder = (*testRecorder)(nil)

func TestHealthCheckRecorder_Interface(t *testing.T) {
	r := &testRecorder{}
	r.RecordHealthCheck("my-provider", "200", "healthy")

	if len(r.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(r.calls))
	}
	if r.calls[0].provider != "my-provider" {
		t.Errorf("expected provider 'my-provider', got '%s'", r.calls[0].provider)
	}
	if r.calls[0].statusCode != "200" {
		t.Errorf("expected statusCode '200', got '%s'", r.calls[0].statusCode)
	}
	if r.calls[0].healthStatus != "healthy" {
		t.Errorf("expected healthStatus 'healthy', got '%s'", r.calls[0].healthStatus)
	}
}
