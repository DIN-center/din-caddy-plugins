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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
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

// blockTimestampSimulator holds configuration for simulating block timestamps
type blockTimestampSimulator struct {
	latestBlock      int64
	blockTimeSeconds float64 // seconds per block (can be fractional for fast chains)
	baseTimestamp    int64   // Unix timestamp of block 0
	shouldFail       bool
}

// getTimestampForBlock calculates the timestamp for a given block number
func (s *blockTimestampSimulator) getTimestampForBlock(blockNum int64) int64 {
	// timestamp = baseTimestamp + (blockNum * blockTimeSeconds)
	// Using float64 for calculation to handle sub-second block times properly
	return s.baseTimestamp + int64(float64(blockNum)*s.blockTimeSeconds)
}

// TestCalculateDynamicBlockLagLimit tests the dynamic block lag calculation with timestamps
func TestCalculateDynamicBlockLagLimit(t *testing.T) {
	tests := []struct {
		name             string
		simulator        blockTimestampSimulator
		expectedMinLimit int64
		expectedMaxLimit int64
		expectNoChange   bool
	}{
		{
			name: "BSC-like network (3s blocks)",
			simulator: blockTimestampSimulator{
				latestBlock:      2000,
				blockTimeSeconds: 3.0, // 3 seconds per block
				baseTimestamp:    1700000000,
			},
			// 13000ms / 3000ms = 4.33, ceil = 5, round to 5
			expectedMinLimit: 5,
			expectedMaxLimit: 5,
		},
		{
			name: "Ethereum-like network (12s blocks)",
			simulator: blockTimestampSimulator{
				latestBlock:      2000,
				blockTimeSeconds: 12.0, // 12 seconds per block
				baseTimestamp:    1700000000,
			},
			// 13000ms / 12000ms = 1.08, ceil = 2, but min is 5
			expectedMinLimit: 5,
			expectedMaxLimit: 5,
		},
		{
			name: "Fast network (1s blocks)",
			simulator: blockTimestampSimulator{
				latestBlock:      2000,
				blockTimeSeconds: 1.0, // 1 second per block
				baseTimestamp:    1700000000,
			},
			// 13000ms / 1000ms = 13, round to 15
			expectedMinLimit: 15,
			expectedMaxLimit: 15,
		},
		{
			name: "Very fast network (200ms blocks)",
			simulator: blockTimestampSimulator{
				latestBlock:      2000,
				blockTimeSeconds: 0.2, // 200ms per block
				baseTimestamp:    1700000000,
			},
			// 13000ms / 200ms = 65, round to 65
			expectedMinLimit: 65,
			expectedMaxLimit: 65,
		},
		{
			name: "Slow network (30s blocks)",
			simulator: blockTimestampSimulator{
				latestBlock:      2000,
				blockTimeSeconds: 30.0, // 30 seconds per block
				baseTimestamp:    1700000000,
			},
			// 13000ms / 30000ms = 0.43, ceil = 1, but min is 5
			expectedMinLimit: 5,
			expectedMaxLimit: 5,
		},
		{
			name: "Provider fails",
			simulator: blockTimestampSimulator{
				shouldFail: true,
			},
			expectNoChange: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := createMockTimestampServer(tt.simulator)
			defer server.Close()

			// Create network with provider
			n := createTestNetworkWithTimestampProvider(t, server.URL, tt.simulator)

			// Store original limit
			originalLimit := n.BlockLagLimit

			// Run the calculation
			n.calculateDynamicBlockLagLimit()

			// Check results
			if tt.expectNoChange {
				assert.Equal(t, originalLimit, n.BlockLagLimit,
					"Block lag limit should not change when provider fails")
			} else {
				actualLimit := atomic.LoadInt64(&n.BlockLagLimit)
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
	// Create a network with mock provider
	simulator := blockTimestampSimulator{
		latestBlock:      2000,
		blockTimeSeconds: 1.0,
		baseTimestamp:    1700000000,
	}
	server := createMockTimestampServer(simulator)
	defer server.Close()

	n := createTestNetworkWithTimestampProvider(t, server.URL, simulator)

	// Start the calculation
	go n.calculateDynamicBlockLagLimit()

	// Concurrently read the BlockLagLimit many times
	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Simulate concurrent reads during health checks
			for j := 0; j < 10; j++ {
				limit := atomic.LoadInt64(&n.BlockLagLimit)
				if limit < 0 {
					errors <- fmt.Errorf("invalid block lag limit: %d", limit)
				}
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
	n.calculateDynamicBlockLagLimit()

	assert.Equal(t, originalLimit, n.BlockLagLimit,
		"Block lag limit should remain unchanged with no providers")
}

// TestDynamicBlockLagWithUnsupportedHandler tests behavior with handlers that don't support dynamic block lag
func TestDynamicBlockLagWithUnsupportedHandler(t *testing.T) {
	n, err := NewNetwork("test-network", BeaconHandler, utils.EnvTest, "8080")
	require.NoError(t, err)

	n.logger = logger.NewLoggerClient(zap.NewNop(), utils.EnvTest)
	n.BlockLagLimit = DefaultBlockLagLimit

	// Initialize Beacon handler (doesn't support dynamic block lag)
	handler := networklib.NewBeaconChainHandler(&networklib.NetworkConfig{
		ChainID: "1",
		Logger:  n.logger,
	})
	n.SetHandler(handler)

	// Add a provider
	n.Providers = make(map[string]*provider)
	p, _ := NewProvider("http://localhost:8545")
	n.Providers["test"] = p

	originalLimit := n.BlockLagLimit
	n.calculateDynamicBlockLagLimit()

	assert.Equal(t, originalLimit, n.BlockLagLimit,
		"Block lag limit should remain unchanged for unsupported handlers")
}

// TestDeterministicCalculation tests that same block data produces same result
func TestDeterministicCalculation(t *testing.T) {
	// Create identical simulators
	simulator := blockTimestampSimulator{
		latestBlock:      2000,
		blockTimeSeconds: 2.0, // 2s blocks
		baseTimestamp:    1700000000,
	}

	// Run calculation multiple times
	var results []int64
	for i := 0; i < 5; i++ {
		server := createMockTimestampServer(simulator)
		n := createTestNetworkWithTimestampProvider(t, server.URL, simulator)
		n.calculateDynamicBlockLagLimit()
		results = append(results, atomic.LoadInt64(&n.BlockLagLimit))
		server.Close()
	}

	// All results should be identical
	for i := 1; i < len(results); i++ {
		assert.Equal(t, results[0], results[i],
			"Calculation should be deterministic - result %d differs from result 0", i)
	}
}

// TestMeasureBlockTimeFromTimestamps tests the timestamp measurement directly
func TestMeasureBlockTimeFromTimestamps(t *testing.T) {
	sim := blockTimestampSimulator{
		latestBlock:      2000,
		blockTimeSeconds: 1.0, // 1 second per block
		baseTimestamp:    1700000000,
	}
	server := createMockTimestampServer(sim)
	defer server.Close()

	n := createTestNetworkWithTimestampProvider(t, server.URL, sim)

	// Get the first provider
	var p *provider
	for _, prov := range n.Providers {
		p = prov
		break
	}
	require.NotNil(t, p, "Provider should not be nil")

	// Test measureBlockTimeFromTimestamps directly
	blockTimeMs, err := n.measureBlockTimeFromTimestamps(p)

	// Expected: 10 seconds for 10 blocks = 1000ms per block
	assert.NoError(t, err)
	assert.InDelta(t, 1000.0, blockTimeMs, 10.0, "Expected ~1000ms per block")
}

// Helper functions

func createMockTimestampServer(sim blockTimestampSimulator) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sim.shouldFail {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Parse the JSON-RPC request
		var rpcReq din_http.JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&rpcReq); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var response din_http.JSONRPCResponse
		response.JSONRPC = "2.0"
		response.ID = json.RawMessage(`1`)

		switch rpcReq.Method {
		case "eth_blockNumber":
			// Return latest block number in hex
			response.Result = json.RawMessage(fmt.Sprintf(`"0x%x"`, sim.latestBlock))

		case "eth_getBlockByNumber":
			// Parse block number from params
			var params []interface{}
			if err := json.Unmarshal(rpcReq.Params, &params); err != nil || len(params) == 0 {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			blockNumStr, ok := params[0].(string)
			if !ok {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			// Parse hex block number
			var blockNum int64
			fmt.Sscanf(blockNumStr, "0x%x", &blockNum)

			// Calculate timestamp for this block
			timestamp := sim.getTimestampForBlock(blockNum)

			// Return block with timestamp
			blockResult := fmt.Sprintf(`{"number":"0x%x","hash":"0xabc123","timestamp":"0x%x"}`, blockNum, timestamp)
			response.Result = json.RawMessage(blockResult)

		default:
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
}

func createTestNetworkWithTimestampProvider(t *testing.T, serverURL string, sim blockTimestampSimulator) *network {
	n, err := NewNetwork("test-network", EVMHandler, utils.EnvTest, "8080")
	require.NoError(t, err)

	// Initialize network dependencies
	n.logger = logger.NewLoggerClient(zap.NewNop(), utils.EnvTest)
	n.HttpClient = din_http.NewHTTPClient(5 * time.Second)
	n.BlockLagLimit = DefaultBlockLagLimit
	n.HCInterval = 5
	n.RequestAttemptCount = 1
	n.quit = make(chan struct{})

	// Initialize PrometheusClient mock
	ctrl := gomock.NewController(t)
	mockProm := prom.NewMockIPrometheusClient(ctrl)
	mockProm.EXPECT().HandleHealthCheckMetric(gomock.Any()).AnyTimes()
	mockProm.EXPECT().HandleNetworkHealthCheckMetric(gomock.Any()).AnyTimes()
	mockProm.EXPECT().HandleRequestMetrics(gomock.Any(), gomock.Any()).AnyTimes()
	n.PrometheusClient = mockProm

	// Initialize EVM handler
	handler := networklib.NewEVMHandler(&networklib.NetworkConfig{
		ChainID: "1",
		Logger:  n.logger,
	})
	n.SetHandler(handler)

	// Add provider
	n.Providers = make(map[string]*provider)
	p, err := NewProvider(serverURL)
	require.NoError(t, err)
	p.logger = n.logger
	n.Providers["test-provider"] = p

	return n
}
