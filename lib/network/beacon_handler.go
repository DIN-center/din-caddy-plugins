// lib/network/beacon_handler.go
package network

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	"go.uber.org/zap"
)

// BeaconChainHandler handles Ethereum Beacon Chain REST API requests
type BeaconChainHandler struct {
	config              *NetworkConfig
	healthCheckEndpoint string
	version             string
	logger              *logger.LoggerClient
}

// NewBeaconChainHandler creates a new Beacon Chain handler instance
func NewBeaconChainHandler(config *NetworkConfig) *BeaconChainHandler {
	return &BeaconChainHandler{
		config:              config,
		healthCheckEndpoint: "/eth/v1/node/health",
		version:             "1.0.0",
		logger:              config.Logger,
	}
}

// Metadata methods for registry
func (h *BeaconChainHandler) GetType() string {
	return "beacon-chain"
}

func (h *BeaconChainHandler) GetName() string {
	return "Ethereum Beacon Chain Handler"
}

func (h *BeaconChainHandler) GetVersion() string {
	return h.version
}

func (h *BeaconChainHandler) GetRequestType() RequestType {
	return RequestTypeREST
}

// Lifecycle methods
func (h *BeaconChainHandler) Initialize(config *NetworkConfig) error {
	h.config = config

	// Update logger from config if available
	if config.Logger != nil {
		h.logger = config.Logger
	}

	return nil
}

func (h *BeaconChainHandler) Shutdown() error {
	// Cleanup resources if needed
	return nil
}

// === EXISTING METHODS ===

// Request processing methods
func (h *BeaconChainHandler) ProcessRequest(req *http.Request) error {
	// Validate the request
	if err := h.ValidateRequest(req); err != nil {
		return err
	}

	return nil
}

// ExtractMethod extracts the method name from the request for logging/metrics
// For REST APIs like Beacon Chain, the method is the URL path
func (h *BeaconChainHandler) ExtractMethod(req *http.Request, body []byte) (string, error) {
	// For REST APIs, the "method" is the path
	return req.URL.Path, nil
}

// ConfigureRequestPath configures the request path for REST API requests
func (h *BeaconChainHandler) ConfigureRequestPath(req *http.Request, providerPath string, networkName string) error {
	ConfigureRESTRequestPath(req, providerPath, networkName)
	return nil
}

func (h *BeaconChainHandler) ValidateRequest(req *http.Request) error {
	// Check for valid HTTP methods - Beacon Chain REST API supports GET and POST
	if req.Method != "GET" && req.Method != "POST" {
		return fmt.Errorf("unsupported HTTP method for Beacon Chain REST API: %s", req.Method)
	}

	// Check for valid path format - must contain Beacon Chain API pattern
	// Accept both formats: "/eth/v1/..." and "/network-name/eth/v1/..."
	path := req.URL.Path
	if !strings.Contains(path, "/eth/v") {
		return fmt.Errorf("invalid Beacon Chain API path: %s", path)
	}

	// For POST requests, validate content type if present
	if req.Method == "POST" {
		contentType := req.Header.Get("Content-Type")
		if contentType != "" && !strings.Contains(contentType, "application/json") {
			return fmt.Errorf("invalid content type for Beacon Chain REST API POST request: %s", contentType)
		}
	}

	return nil
}

// Response handling methods
func (h *BeaconChainHandler) ParseResponse(body []byte, statusCode int) error {
	// Check HTTP status code and handle non-2xx responses as errors
	if statusCode < 200 || statusCode >= 300 {
		err := fmt.Errorf("HTTP error: %d", statusCode)
		return err
	}

	// Parse JSON response
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		jsonErr := fmt.Errorf("failed to parse JSON response: %w", err)
		return jsonErr
	}

	// Check for error field in response
	if errorField, exists := response["error"]; exists && errorField != nil {
		apiErr := fmt.Errorf("Beacon Chain API error: %v", errorField)
		return apiErr
	}

	return nil
}

func (h *BeaconChainHandler) IsRetryableError(err error, statusCode int) bool {
	// HTTP server errors are retryable
	if statusCode >= 500 {
		return true
	}

	// Rate limiting is retryable
	if statusCode == 429 {
		return true
	}

	// If no error provided, check only status code
	if err == nil {
		return false
	}

	// Connection errors are retryable (following EVM handler patterns)
	errMsg := strings.ToLower(err.Error())
	retryablePatterns := []string{
		"timeout",
		"connection",
		"network",
		"rate limit",
		"server error",
		"internal error",
		"unavailable",
		"socket hang up",
		"connection reset",
		"temporary failure",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}

	return false
}

// === NEW NETWORK-SPECIFIC METHODS ===

// Chain ID and Namespace methods
func (h *BeaconChainHandler) GetNamespace() string {
	return "beacon" // Beacon chain uses same namespace as Ethereum mainnet
}

func (h *BeaconChainHandler) ValidateChainID(chainID string) error {
	// Beacon chain uses the same chain ID format as Ethereum mainnet
	if !strings.HasPrefix(chainID, "beacon:") {
		return fmt.Errorf("invalid Beacon Chain chain ID format: %s, expected format: beacon:{chainId}", chainID)
	}

	// Extract chain ID number and validate it's numeric
	parts := strings.Split(chainID, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid Beacon Chain chain ID format: %s", chainID)
	}

	chainIDNum := parts[1]
	if chainIDNum == "" {
		return fmt.Errorf("empty chain ID number in: %s", chainID)
	}

	// Convert to ensure it's a valid number
	if _, err := strconv.ParseInt(chainIDNum, 10, 64); err != nil {
		return fmt.Errorf("invalid chain ID number in %s: %w", chainID, err)
	}

	return nil
}

func (h *BeaconChainHandler) FormatChainID(networkReference string) string {
	return h.GetNamespace() + ":" + networkReference
}

func (h *BeaconChainHandler) ExtractChainReference(result interface{}) (string, error) {
	// For beacon chain, we extract chain reference from genesis response
	// The result should be the parsed response bytes
	if responseBytes, ok := result.([]byte); ok {
		var genesisResponse BeaconGenesisResponse
		if err := json.Unmarshal(responseBytes, &genesisResponse); err != nil {
			return "", fmt.Errorf("failed to parse beacon genesis response: %w", err)
		}
		// For beacon chain, we use "1" as the chain reference for mainnet
		return "1", nil
	}

	// If it's already a string, return as is
	if chainRef, ok := result.(string); ok {
		return chainRef, nil
	}

	return "", fmt.Errorf("invalid chain reference type: %T", result)
}

// Block Operations methods
func (h *BeaconChainHandler) FormatBlockHeight(blockNum int64) string {
	return strconv.FormatInt(blockNum, 10) // Decimal format for REST API
}

func (h *BeaconChainHandler) CreateBlockRequest(method string, blockNum int64, includeTransactions bool) ([]byte, error) {
	// Beacon chain uses REST API, not JSON-RPC
	return nil, fmt.Errorf("Beacon Chain uses REST API, not JSON-RPC block requests")
}

func (h *BeaconChainHandler) ParseBlockResponse(body []byte) (interface{}, error) {
	// Parse beacon chain block response
	var response BeaconHeadResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Beacon Chain block response: %w", err)
	}
	return response, nil
}

// ParseBlockNumberResponse parses the block number from a raw response
func (h *BeaconChainHandler) ParseBlockNumberResponse(body []byte, statusCode int) (int64, error) {
	// Check HTTP status first
	if statusCode >= 400 {
		if statusCode == 429 {
			return 0, fmt.Errorf("rate limit error (status code: %d)", statusCode)
		}
		return 0, fmt.Errorf("error status code: %d", statusCode)
	}

	// For beacon chain REST API, parse the health check response to get slot number
	blockInfo, err := h.ParseHealthCheckResponse(body)
	if err != nil {
		return 0, err
	}

	// Return slot number as the "block number" for consistency
	return blockInfo.Number, nil
}

// Archive Mode methods
func (h *BeaconChainHandler) SupportsArchiveMode() bool {
	return false // Archive mode disabled for beacon chain
}

func (h *BeaconChainHandler) GetArchiveMethod() string {
	return ""
}

func (h *BeaconChainHandler) CreateArchivePayload(method string, blockHeight string) ([]byte, error) {
	return nil, fmt.Errorf("Beacon Chain does not support archive mode")
}

func (h *BeaconChainHandler) ParseArchiveResponse(body []byte) error {
	return fmt.Errorf("Beacon Chain does not support archive mode")
}

// Network Capabilities methods
func (h *BeaconChainHandler) SupportsGetBlockByNumber() bool {
	return false // Beacon chain doesn't support getBlockByNumber
}

func (h *BeaconChainHandler) GetSupportedMethods() []string {
	// These are REST endpoints, not JSON-RPC methods
	return []string{
		"/eth/v1/beacon/headers/head",
		"/eth/v1/beacon/blocks/head",
		"/eth/v2/beacon/blocks/head",
		"/eth/v2/beacon/blocks/{block_id}",
		"/eth/v1/beacon/states/head/validators",
		"/eth/v1/beacon/genesis",
		"/eth/v1/node/version",
		"/eth/v1/node/health",
		"/eth/v1/config/fork_schedule",
	}
}

func (h *BeaconChainHandler) GetBlockByNumberMethod() string {
	// Use the blocks endpoint - the {block_id} will be replaced with actual slot number
	return "/eth/v2/beacon/blocks/{block_id}"
}

// Data Format Conversions methods
func (h *BeaconChainHandler) ExtractBlockHash(blockData interface{}) string {
	if beaconResponse, ok := blockData.(BeaconHeadResponse); ok {
		if len(beaconResponse.Data) > 0 {
			return beaconResponse.Data[0].Root
		}
	}
	return ""
}

func (h *BeaconChainHandler) ExtractBlockNumber(response []byte) (int64, error) {
	// Beacon chain uses slots instead of block numbers
	var beaconResponse BeaconHeadResponse
	if err := json.Unmarshal(response, &beaconResponse); err != nil {
		return 0, fmt.Errorf("failed to unmarshal beacon response: %w", err)
	}

	// Validate that we have data
	if len(beaconResponse.Data) == 0 {
		return 0, fmt.Errorf("beacon response contains no data entries")
	}

	slot, err := h.parseSlot(beaconResponse.Data[0].Header.Message.Slot)
	if err != nil {
		return 0, fmt.Errorf("failed to parse slot as block number: %w", err)
	}

	return slot, nil
}

// Health Check Specifics methods
func (h *BeaconChainHandler) GetHealthCheckMethod() string {
	// Use the node health endpoint for health checks
	// This returns HTTP status codes: 200 (ready), 206 (syncing), 503 (not initialized)
	return h.healthCheckEndpoint
}

func (h *BeaconChainHandler) GetHealthCheckHTTPMethod() string {
	return "GET" // Beacon chain uses REST GET requests
}

func (h *BeaconChainHandler) RequiresSeparateBlockInfoCall() bool {
	return true
}

func (h *BeaconChainHandler) GetBlockInfoMethod() string {
	// Use the /eth/v2/beacon/blocks/head endpoint to get the latest block
	// This provides complete block information including slot number
	return "/eth/v2/beacon/blocks/head"
}

func (h *BeaconChainHandler) GetChainIDMethod() string {
	// Use config/spec endpoint to get network information
	// This provides chain configuration including the CONFIG_NAME
	return "/eth/v1/config/spec"
}

func (h *BeaconChainHandler) CreateHealthCheckPayload(method string) ([]byte, error) {
	// Beacon chain uses REST API, no payload needed for GET requests
	return nil, nil
}

func (h *BeaconChainHandler) ParseHealthCheckResponse(body []byte) (*BlockInfo, error) {
	// Handle empty response (node health endpoint returns empty body for 200 OK)
	if len(body) == 0 {
		return &BlockInfo{
			Number:    -1,
			Hash:      "",
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"health": "ready",
				"slot":   int64(-1),
				"epoch":  int64(-1),
			},
		}, nil
	}

	// from the block info endpoint (/eth/v2/beacon/blocks/head)
	var blockResponse BeaconBlockResponse
	if err := json.Unmarshal(body, &blockResponse); err != nil {
		return nil, fmt.Errorf("failed to parse beacon block response: %w", err)
	}

	// Parse slot from the response
	slot, err := h.parseSlot(blockResponse.Data.Message.Slot)
	if err != nil {
		return nil, fmt.Errorf("failed to parse slot: %w", err)
	}

	// Calculate epoch from slot (32 slots per epoch)
	epoch := slot / 32

	return &BlockInfo{
		Number:    slot, // Use slot as block number for consistency
		Hash:      blockResponse.Data.Message.ParentRoot,
		Timestamp: time.Now(), // Beacon chain responses don't include timestamp in header
		Metadata: map[string]interface{}{
			"slot":                 slot,
			"epoch":                epoch,
			"execution_optimistic": blockResponse.ExecutionOptimistic,
			"finalized":            blockResponse.Finalized,
		},
	}, nil
}

func (h *BeaconChainHandler) ParseChainIDResponse(body []byte, statusCode int) (string, error) {
	if statusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP error: %d", statusCode)
	}

	// Parse the config/spec response
	var specResponse struct {
		Data map[string]string `json:"data"`
	}

	if err := json.Unmarshal(body, &specResponse); err != nil {
		return "", fmt.Errorf("failed to parse beacon config response: %w", err)
	}

	// Extract DEPOSIT_CHAIN_ID which is the actual Ethereum chain ID
	chainID, exists := specResponse.Data["DEPOSIT_CHAIN_ID"]
	if !exists {
		return "", fmt.Errorf("DEPOSIT_CHAIN_ID not found in beacon config response")
	}

	return h.FormatChainID(chainID), nil
}

// GetChainID retrieves the chain ID for beacon chain
// For beacon chain, we can either return the configured chain ID directly
// or make a REST API call to get the network configuration
func (h *BeaconChainHandler) GetChainID(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int) (string, error) {
	var lastErr error
	for attempt := 0; attempt < requestAttempts; attempt++ {
		// Make GET request to config/spec endpoint
		configURL, err := url.JoinPath(httpUrl, h.GetChainIDMethod())
		if err != nil {
			lastErr = fmt.Errorf("failed to construct config URL: %w", err)
			continue
		}
		resBytes, statusCode, err := httpClient.Get(configURL, headers, authClient)
		if err != nil {
			lastErr = fmt.Errorf("error sending HTTP request: %w", err)
			continue
		}

		// Parse the response to extract chain ID
		chainID, err := h.ParseChainIDResponse(resBytes, *statusCode)
		if err != nil {
			lastErr = err
			continue
		}

		// Success!
		return chainID, nil
	}

	return "", fmt.Errorf("failed after %d attempts: %w", requestAttempts, lastErr)
}

// GetLatestBlockNumber retrieves the latest block number (slot) for beacon chain
// Uses the REST API endpoint /eth/v2/beacon/blocks/head to get the latest slot
func (h *BeaconChainHandler) GetLatestBlockNumber(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int) (*LatestBlockResult, error) {
	var lastErr error
	var lastResponseStatus int
	var lastHealthStatus HealthStatus = Unhealthy

	// Use the block info endpoint to get latest slot
	blockInfoMethod := h.GetBlockInfoMethod()

	h.logger.Debug("Starting beacon chain latest block number request",
		zap.String("endpoint", blockInfoMethod),
		zap.String("base_url", httpUrl),
		zap.Int("max_attempts", requestAttempts))

	for attempt := 0; attempt < requestAttempts; attempt++ {
		// Make GET request to beacon headers/head endpoint
		blockInfoURL, err := url.JoinPath(httpUrl, blockInfoMethod)
		if err != nil {
			lastErr = fmt.Errorf("failed to construct block info URL: %w", err)
			lastHealthStatus = Unhealthy
			continue
		}

		h.logger.Debug("Making GET request for latest block number",
			zap.String("url", blockInfoURL),
			zap.String("network", "beacon"),
			zap.Int("attempt", attempt+1))

		resBytes, statusCode, err := httpClient.Get(blockInfoURL, headers, authClient)
		if statusCode != nil {
			lastResponseStatus = *statusCode
		}

		if err != nil {
			lastErr = fmt.Errorf("error sending HTTP request: %w", err)
			// Check if it's a retryable error based on status code
			if lastResponseStatus >= 500 || lastResponseStatus == 429 {
				lastHealthStatus = Warning
			} else {
				lastHealthStatus = Unhealthy
			}
			h.logger.Debug("HTTP request failed",
				zap.Error(err),
				zap.Int("status_code", lastResponseStatus),
				zap.Int("attempt", attempt+1))
			continue
		}

		// Check HTTP status code
		if lastResponseStatus >= 400 {
			if lastResponseStatus == 429 {
				lastErr = fmt.Errorf("rate limit error (status code: %d)", lastResponseStatus)
				lastHealthStatus = Warning
			} else {
				lastErr = fmt.Errorf("error status code: %d", lastResponseStatus)
				lastHealthStatus = Unhealthy
			}
			h.logger.Debug("HTTP request returned error status",
				zap.Int("status_code", lastResponseStatus),
				zap.String("response_body", string(resBytes)),
				zap.Int("attempt", attempt+1))
			continue
		}

		h.logger.Debug("Received successful response from beacon API",
			zap.Int("status_code", lastResponseStatus),
			zap.Int("response_size", len(resBytes)),
			zap.String("response_preview", string(resBytes[:min(len(resBytes), 200)])))

		// Parse the beacon chain response to get block info
		blockInfo, err := h.ParseHealthCheckResponse(resBytes)
		if err != nil {
			lastErr = fmt.Errorf("failed to parse beacon chain response: %w", err)
			lastHealthStatus = Unhealthy
			h.logger.Debug("Failed to parse beacon response",
				zap.Error(err),
				zap.String("response_snippet", string(resBytes[:min(len(resBytes), 500)])),
				zap.Int("attempt", attempt+1))
			continue
		}

		if blockInfo == nil {
			lastErr = fmt.Errorf("beacon chain response parsing returned nil block info")
			lastHealthStatus = Unhealthy
			h.logger.Debug("Received nil block info from parser",
				zap.Int("attempt", attempt+1))
			continue
		}

		// Success! Use slot as block number for consistency with other networks
		h.logger.Debug("Successfully retrieved latest block number",
			zap.Int64("slot", blockInfo.Number), // Number field contains slot for beacon chain
			zap.Int64("block_number", blockInfo.Number),
			zap.Int64("epoch", getInt64FromMetadata(blockInfo.Metadata, "epoch")),
			zap.String("hash", blockInfo.Hash),
			zap.Bool("execution_optimistic", getBoolFromMetadata(blockInfo.Metadata, "execution_optimistic")),
			zap.Bool("finalized", getBoolFromMetadata(blockInfo.Metadata, "finalized")))

		return &LatestBlockResult{
			BlockNumber:    blockInfo.Number, // This will be the slot number
			HealthStatus:   Healthy,
			ResponseStatus: lastResponseStatus,
			Metadata: map[string]interface{}{
				"slot":      blockInfo.Number, // Number field contains slot for beacon chain
				"epoch":     getInt64FromMetadata(blockInfo.Metadata, "epoch"),
				"hash":      blockInfo.Hash,
				"timestamp": blockInfo.Timestamp,
				"endpoint":  blockInfoMethod,
			},
		}, nil
	}

	// All attempts failed
	h.logger.Warn("All attempts to get latest block number failed",
		zap.Error(lastErr),
		zap.Int("attempts", requestAttempts),
		zap.String("health_status", lastHealthStatus.String()),
		zap.String("endpoint", blockInfoMethod))

	return &LatestBlockResult{
		BlockNumber:    0,
		HealthStatus:   lastHealthStatus,
		ResponseStatus: lastResponseStatus,
		Metadata:       make(map[string]interface{}),
	}, fmt.Errorf("failed after %d attempts: %w", requestAttempts, lastErr)
}

// PerformArchiveCheck for beacon chain - stubbed out (archive mode disabled)
func (h *BeaconChainHandler) PerformArchiveCheck(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, blockHeight string) error {
	// Archive checks are disabled for beacon chain
	return nil
}

// PerformGetBlockByNumber for beacon chain using blocks endpoint
func (h *BeaconChainHandler) PerformGetBlockByNumber(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, blockNumber int64) (interface{}, error) {
	// Use the blocks endpoint with the specific slot number
	blockEndpoint := fmt.Sprintf("/eth/v2/beacon/blocks/%d", blockNumber)
	fullURL, err := url.JoinPath(httpUrl, blockEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to construct block URL: %w", err)
	}

	resBytes, statusCode, err := httpClient.Get(fullURL, headers, authClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get block by number: %w", err)
	}

	if *statusCode >= 400 {
		return nil, fmt.Errorf("HTTP error %d getting block by number", *statusCode)
	}

	// Parse the response using the same structure as health check
	var blockResponse BeaconBlockResponse
	if err := json.Unmarshal(resBytes, &blockResponse); err != nil {
		return nil, fmt.Errorf("failed to parse block response: %w", err)
	}

	return blockResponse, nil
}

// === EXISTING HELPER METHODS ===

// Response structures for beacon chain
type BeaconHeadResponse struct {
	ExecutionOptimistic bool `json:"execution_optimistic"`
	Finalized           bool `json:"finalized"`
	Data                []struct {
		Root      string `json:"root"`
		Canonical bool   `json:"canonical"`
		Header    struct {
			Message struct {
				Slot          string `json:"slot"`
				ProposerIndex string `json:"proposer_index"`
				ParentRoot    string `json:"parent_root"`
				StateRoot     string `json:"state_root"`
				BodyRoot      string `json:"body_root"`
			} `json:"message"`
			Signature string `json:"signature"`
		} `json:"header"`
	} `json:"data"`
}

// BeaconBlockResponse represents the response from /eth/v2/beacon/blocks/head
type BeaconBlockResponse struct {
	Version             string `json:"version"`
	ExecutionOptimistic bool   `json:"execution_optimistic"`
	Finalized           bool   `json:"finalized"`
	Data                struct {
		Message struct {
			Slot          string      `json:"slot"`
			ProposerIndex string      `json:"proposer_index"`
			ParentRoot    string      `json:"parent_root"`
			StateRoot     string      `json:"state_root"`
			Body          interface{} `json:"body"` // Complex structure, we only need slot
		} `json:"message"`
		Signature string `json:"signature"`
	} `json:"data"`
}

type BeaconGenesisResponse struct {
	Data struct {
		GenesisTime           string `json:"genesis_time"`
		GenesisValidatorsRoot string `json:"genesis_validators_root"`
		GenesisForkVersion    string `json:"genesis_fork_version"`
	} `json:"data"`
}

// Helper function to convert slot string to int64
func (h *BeaconChainHandler) parseSlot(slotStr string) (int64, error) {
	slot, err := strconv.ParseInt(slotStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse slot: %w", err)
	}
	return slot, nil
}

// Helper function to safely get int64 value from metadata
func getInt64FromMetadata(metadata map[string]interface{}, key string) int64 {
	if metadata == nil {
		return 0
	}
	if val, ok := metadata[key]; ok {
		// Handle both int64 and int types
		if intVal, ok := val.(int64); ok {
			return intVal
		}
		if intVal, ok := val.(int); ok {
			return int64(intVal)
		}
	}
	return 0
}

// Helper function to safely get bool value from metadata
func getBoolFromMetadata(metadata map[string]interface{}, key string) bool {
	if metadata == nil {
		return false
	}
	if val, ok := metadata[key]; ok {
		if boolVal, ok := val.(bool); ok {
			return boolVal
		}
	}
	return false
}
