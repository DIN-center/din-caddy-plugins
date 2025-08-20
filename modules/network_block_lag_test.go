package modules

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	networklib "github.com/DIN-center/din-caddy-plugins/lib/network"
	prom "github.com/DIN-center/din-caddy-plugins/lib/prometheus"
	"github.com/DIN-center/din-caddy-plugins/lib/utils"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestRoundUpToInterval tests the rounding logic
func TestRoundUpToInterval(t *testing.T) {
	tests := []struct {
		name     string
		value    int64
		interval int64
		expected int64
	}{
		{
			name:     "already multiple of 5",
			value:    45,
			interval: 5,
			expected: 45,
		},
		{
			name:     "round 2 to 5",
			value:    2,
			interval: 5,
			expected: 5,
		},
		{
			name:     "round 39 to 40",
			value:    39,
			interval: 5,
			expected: 40,
		},
		{
			name:     "round 131 to 135",
			value:    131,
			interval: 5,
			expected: 135,
		},
		{
			name:     "zero stays zero",
			value:    0,
			interval: 5,
			expected: 0,
		},
		{
			name:     "round 1 to 5",
			value:    1,
			interval: 5,
			expected: 5,
		},
		{
			name:     "large number",
			value:    999,
			interval: 5,
			expected: 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := roundUpToInterval(tt.value, tt.interval)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCalculateDynamicBlockLagLimit tests the dynamic block lag calculation
func TestCalculateDynamicBlockLagLimit(t *testing.T) {
	tests := []struct {
		name              string
		providerRates     map[string]blockRateSimulator
		expectedMinLimit  int64
		expectedMaxLimit  int64
		expectNoChange    bool
	}{
		{
			name: "BSC-like network (3s blocks)",
			providerRates: map[string]blockRateSimulator{
				"provider1": {initialBlock: 1000, blocksPerSecond: 0.33}, // ~3s per block
				"provider2": {initialBlock: 2000, blocksPerSecond: 0.33},
			},
			expectedMinLimit: 5,  // In 1s: ~0.33 blocks, 13000ms / 3000ms = 4.33, rounds to 5
			expectedMaxLimit: 5,
		},
		{
			name: "Ethereum-like network (12s blocks)",
			providerRates: map[string]blockRateSimulator{
				"provider1": {initialBlock: 1000, blocksPerSecond: 0.083}, // ~12s per block
				"provider2": {initialBlock: 2000, blocksPerSecond: 0.083},
			},
			expectedMinLimit: 5,  // In 1s: ~0.083 blocks, 13000ms / 12000ms = 1.08, rounds to 5
			expectedMaxLimit: 5,
		},
		{
			name: "Mixed speed providers - fastest wins",
			providerRates: map[string]blockRateSimulator{
				"slow":   {initialBlock: 1000, blocksPerSecond: 0.1},  // 10s per block
				"medium": {initialBlock: 2000, blocksPerSecond: 0.5},  // 2s per block
				"fast":   {initialBlock: 3000, blocksPerSecond: 1.0},  // 1s per block
			},
			expectedMinLimit: 15, // In 1s: 1 block, 13000ms / 1000ms = 13, rounds to 15
			expectedMaxLimit: 15,
		},
		{
			name: "Very fast network (100ms blocks)",
			providerRates: map[string]blockRateSimulator{
				"provider1": {initialBlock: 1000, blocksPerSecond: 10}, // 100ms per block
			},
			expectedMinLimit: 130, // In 1s: 10 blocks, 13000ms / 100ms = 130
			expectedMaxLimit: 130,
		},
		{
			name: "One provider fails - use working provider",
			providerRates: map[string]blockRateSimulator{
				"working": {initialBlock: 1000, blocksPerSecond: 0.5}, // 2s per block
				"failing": {initialBlock: 0, shouldFail: true},
			},
			expectedMinLimit: 5, // In 1s: 0.5 blocks might round, could get 5 or 10
			expectedMaxLimit: 10,
		},
		{
			name: "All providers fail - keep default",
			providerRates: map[string]blockRateSimulator{
				"fail1": {shouldFail: true},
				"fail2": {shouldFail: true},
			},
			expectNoChange: true,
		},
		{
			name: "Stalled provider (no new blocks)",
			providerRates: map[string]blockRateSimulator{
				"stalled": {initialBlock: 1000, blocksPerSecond: 0}, // No progress
				"working": {initialBlock: 2000, blocksPerSecond: 0.2}, // 5s per block
			},
			expectedMinLimit: 5, // 13000ms / 5000ms = 2.6, rounds to 5
			expectedMaxLimit: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server for providers
			server := createMockBlockServer(tt.providerRates)
			defer server.Close()

			// Create network with providers
			n := createTestNetworkWithProviders(t, server.URL, tt.providerRates)

			// Store original limit
			originalLimit := n.BlockLagLimit

			// Run the calculation with shorter duration for tests (1 second instead of 10)
			n.calculateDynamicBlockLagLimit(1)

			// Check results
			if tt.expectNoChange {
				assert.Equal(t, originalLimit, n.BlockLagLimit,
					"Block lag limit should not change when all providers fail")
			} else {
				n.blockLagLimitMu.RLock()
				actualLimit := n.BlockLagLimit
				n.blockLagLimitMu.RUnlock()

				assert.GreaterOrEqual(t, actualLimit, tt.expectedMinLimit,
					"Block lag limit should be at least %d, got %d", tt.expectedMinLimit, actualLimit)
				assert.LessOrEqual(t, actualLimit, tt.expectedMaxLimit,
					"Block lag limit should be at most %d, got %d", tt.expectedMaxLimit, actualLimit)
			}
		})
	}
}

// TestCalculateDynamicBlockLagLimitConcurrency tests thread safety
func TestCalculateDynamicBlockLagLimitConcurrency(t *testing.T) {
	// Create a network with mock providers
	providerRates := map[string]blockRateSimulator{
		"provider1": {initialBlock: 1000, blocksPerSecond: 1.0},
	}
	server := createMockBlockServer(providerRates)
	defer server.Close()

	n := createTestNetworkWithProviders(t, server.URL, providerRates)

	// Start the calculation with 1 second measurement for faster tests
	go n.calculateDynamicBlockLagLimit(1)

	// Concurrently read the BlockLagLimit many times
	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			// Simulate concurrent reads during health checks
			for j := 0; j < 10; j++ {
				n.blockLagLimitMu.RLock()
				limit := n.BlockLagLimit
				n.blockLagLimitMu.RUnlock()
				
				if limit < 0 {
					errors <- fmt.Errorf("invalid block lag limit: %d", limit)
				}
				
				time.Sleep(time.Millisecond * 10)
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Errorf("Concurrent access error: %v", err)
	}
}

// TestCalculateDynamicBlockLagLimitIntegration tests with actual network simulation
func TestCalculateDynamicBlockLagLimitIntegration(t *testing.T) {
	// Skip in short mode as this test takes 10+ seconds
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a more realistic simulation with faster blocks to ensure limit changes
	providerRates := map[string]blockRateSimulator{
		"provider1": {initialBlock: 100000, blocksPerSecond: 0.5},  // 2s blocks
		"provider2": {initialBlock: 100500, blocksPerSecond: 1.0},  // 1s blocks - fastest
		"provider3": {initialBlock: 99800, blocksPerSecond: 0.33},  // 3s blocks
	}

	server := createMockBlockServer(providerRates)
	defer server.Close()

	n := createTestNetworkWithProviders(t, server.URL, providerRates)
	
	// Store initial limit
	initialLimit := n.BlockLagLimit

	// Start health checking which triggers the calculation
	// Override the calculation with shorter duration for testing
	go n.calculateDynamicBlockLagLimit(1)
	n.healthCheck()
	
	// Wait for the dynamic calculation to complete (it takes ~1 second)
	time.Sleep(2 * time.Second)

	// Verify the result
	n.blockLagLimitMu.RLock()
	finalLimit := n.BlockLagLimit
	n.blockLagLimitMu.RUnlock()

	// Verify that the limit changed from initial value
	assert.NotEqual(t, initialLimit, finalLimit, "Block lag limit should have been updated")
	
	// For 1s blocks (fastest provider), we expect: 13000ms / 1000ms = 13, rounds to 15
	assert.Equal(t, int64(15), finalLimit, "Expected block lag limit for network with 1s blocks")
}

// Helper types and functions

type blockRateSimulator struct {
	initialBlock    int64
	blocksPerSecond float64
	shouldFail      bool
	startTime       time.Time
}

func createMockBlockServer(simulators map[string]blockRateSimulator) *httptest.Server {
	// Use a mutex to protect the shared start time map
	var mu sync.Mutex
	providerStartTimes := make(map[string]time.Time)

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract provider name from URL path
		providerName := r.URL.Path[1:] // Remove leading /
		
		sim, exists := simulators[providerName]
		if !exists || sim.shouldFail {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Thread-safe access to start times
		mu.Lock()
		startTime, hasStart := providerStartTimes[providerName]
		if !hasStart {
			// First request from this provider - record start time
			startTime = time.Now()
			providerStartTimes[providerName] = startTime
		}
		mu.Unlock()

		// Calculate current block based on time elapsed since this provider's first request
		elapsed := time.Since(startTime).Seconds()
		currentBlock := sim.initialBlock + int64(elapsed*sim.blocksPerSecond)

		// Return JSON-RPC response
		response := din_http.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`1`),
			Result:  json.RawMessage(fmt.Sprintf(`"0x%x"`, currentBlock)),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
}

func createTestNetworkWithProviders(t *testing.T, serverURL string, providers map[string]blockRateSimulator) *network {
	n, err := NewNetwork("test-network", EVMHandler, utils.EnvTest, "8080")
	require.NoError(t, err)

	// Initialize network dependencies
	n.logger = logger.NewLoggerClient(zap.NewNop(), utils.EnvTest)
	n.HttpClient = din_http.NewHTTPClient(5 * time.Second)
	n.BlockLagLimit = DefaultBlockLagLimit
	n.HCInterval = 5
	n.RequestAttemptCount = 1
	n.quit = make(chan struct{})
	
	// Initialize PrometheusClient mock to avoid nil pointer in health check
	ctrl := gomock.NewController(t)
	mockProm := prom.NewMockIPrometheusClient(ctrl)
	// Set up expectations for the mock - allow any number of calls
	mockProm.EXPECT().HandleHealthCheckMetric(gomock.Any()).AnyTimes()
	mockProm.EXPECT().HandleNetworkHealthCheckMetric(gomock.Any()).AnyTimes()
	mockProm.EXPECT().HandleRequestMetrics(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	n.PrometheusClient = mockProm

	// Initialize handler
	handler := networklib.NewEVMHandler(&networklib.NetworkConfig{
		ChainID: "1",
		Logger:  n.logger,
	})
	n.SetHandler(handler)

	// Add providers
	n.Providers = make(map[string]*provider)
	for name := range providers {
		p, err := NewProvider(fmt.Sprintf("%s/%s", serverURL, name))
		require.NoError(t, err)
		p.logger = n.logger
		n.Providers[name] = p
	}

	return n
}

// TestDynamicBlockLagWithNoProviders tests behavior when no providers are available
func TestDynamicBlockLagWithNoProviders(t *testing.T) {
	n, err := NewNetwork("test-network", EVMHandler, utils.EnvTest, "8080")
	require.NoError(t, err)

	n.logger = logger.NewLoggerClient(zap.NewNop(), utils.EnvTest)
	n.BlockLagLimit = DefaultBlockLagLimit
	n.Providers = make(map[string]*provider) // Empty providers
	
	// Initialize handler to avoid nil pointer
	handler := networklib.NewEVMHandler(&networklib.NetworkConfig{
		ChainID: "1",
		Logger:  n.logger,
	})
	n.SetHandler(handler)

	originalLimit := n.BlockLagLimit
	n.calculateDynamicBlockLagLimit(1) // Use 1 second for faster test

	assert.Equal(t, originalLimit, n.BlockLagLimit,
		"Block lag limit should remain unchanged with no providers")
}

// TestDynamicBlockLagErrorRecovery tests graceful handling of errors
func TestDynamicBlockLagErrorRecovery(t *testing.T) {
	// Create a server that returns errors initially then succeeds
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		
		// Fail first 2 requests, succeed on 3rd and beyond
		if count <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Return a valid block number
		response := din_http.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`1`),
			Result:  json.RawMessage(fmt.Sprintf(`"0x%x"`, 1000+count*10)),
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	n := createTestNetworkWithProviders(t, server.URL, map[string]blockRateSimulator{
		"provider1": {}, // Will use server behavior
	})

	// Should handle errors gracefully (use 1 second for faster test)
	n.calculateDynamicBlockLagLimit(1)

	// Verify it either kept default or calculated based on partial data
	n.blockLagLimitMu.RLock()
	limit := n.BlockLagLimit
	n.blockLagLimitMu.RUnlock()

	assert.Greater(t, limit, int64(0), "Block lag limit should be positive")
}