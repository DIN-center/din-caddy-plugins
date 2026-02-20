package network

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
)

type StellarRpcHandler struct {
	config  *NetworkConfig
	version string
	logger  *logger.LoggerClient
}

func NewStellarRpcHandler(config *NetworkConfig) *StellarRpcHandler {
	return &StellarRpcHandler{
		config:  config,
		version: "1.0.0",
		logger:  config.Logger,
	}
}

// Metadata methods for registry
func (h *StellarRpcHandler) GetType() HandlerType {
	return StellarRpcHandlerType
}

func (h *StellarRpcHandler) GetName() string {
	return "Stellar JSON-RPC Handler"
}

func (h *StellarRpcHandler) GetVersion() string {
	return h.version
}

func (h *StellarRpcHandler) GetRequestType() RequestType {
	return RequestTypeRPC
}

// Lifecycle methods
func (h *StellarRpcHandler) Initialize(config *NetworkConfig) error {
	h.config = config

	if config.Logger != nil {
		h.logger = config.Logger
	}

	return nil
}

func (h *StellarRpcHandler) Shutdown() error {
	return nil
}

// === EXISTING METHODS ===

func (h *StellarRpcHandler) ProcessRequest(req *http.Request) error {
	// Validate the request using our validation logic
	if err := h.ValidateRequest(req); err != nil {
		return err
	}

	// Path translation is handled in DinSelect module
	return nil
}

// ExtractMethod extracts the JSON-RPC method from the request body
func (h *StellarRpcHandler) ExtractMethod(req *http.Request, body []byte) (string, error) {
	if len(body) == 0 {
		return "", fmt.Errorf("empty request body")
	}

	var rpcRequest din_http.JSONRPCRequest
	if err := json.Unmarshal(body, &rpcRequest); err != nil {
		return "", fmt.Errorf("failed to parse JSON-RPC request: %w", err)
	}

	if rpcRequest.Method == "" {
		return "", fmt.Errorf("missing method in JSON-RPC request")
	}

	return rpcRequest.Method, nil
}

// ConfigureRequestPath configures the request path for JSON-RPC requests
func (h *StellarRpcHandler) ConfigureRequestPath(req *http.Request, providerPath string, providerQuery string, networkName string) error {
	ConfigureJSONRPCRequestPath(req, providerPath)
	return nil
}

func (h *StellarRpcHandler) ValidateRequest(req *http.Request) error {
	// Check content type
	contentType := req.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		return fmt.Errorf("invalid content type for Stellar JSON-RPC: %s", contentType)
	}

	// Check method
	if req.Method != "POST" {
		return fmt.Errorf("Stellar JSON-RPC requires POST method, got %s", req.Method)
	}

	return nil
}

func (h *StellarRpcHandler) ParseResponse(body []byte, statusCode int) error {
	// Use the shared JSON-RPC response parser
	return ParseJSONRPCResponse(body, statusCode)
}

func (h *StellarRpcHandler) IsRetryableError(err error, statusCode int) bool {
	// Use the shared JSON-RPC error retry logic
	return IsRetryableJSONRPCError(err, statusCode)
}

func (h *StellarRpcHandler) IsRetryableOnDifferentProvider(err error, statusCode int) bool {
	return IsRetryableOnDifferentProviderJSONRPCError(err, statusCode)
}

// === NEW NETWORK-SPECIFIC METHODS ===

// Chain ID and Namespace methods
func (h *StellarRpcHandler) GetNamespace() string {
	return "stellar"
}

func (h *StellarRpcHandler) ValidateChainID(chainID string) error {
	if len(chainID) == 0 {
		return fmt.Errorf("empty chain ID")
	}

	// Each Stellar network has a unique identifier in the format “[Network Name] ; [Month of Creation] [Year of Creation]” also known as the network passphrase.
	// Check for known network prefixes: "Public Global Stellar Network" or "Test SDF Network"
	// See documentation: https://developers.stellar.org/docs/networks#network-passphrases
	validPrefixes := []string{
		"public global stellar network",
		"test sdf network",
	}

	qualifiedChainID := strings.ToLower(strings.TrimSpace(chainID))
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(qualifiedChainID, prefix) {
			return nil
		}
	}

	return fmt.Errorf("invalid Stellar network passphrase: see supported networks at https://developers.stellar.org/docs/networks#network-passphrases, got: %s", chainID)
}

func (h *StellarRpcHandler) ExtractChainReference(result interface{}) (string, error) {
	// For Stellar, the result from getNetwork is an object containing networkPassphrase
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid network result type: %T", result)
	}

	passphrase, ok := resultMap["passphrase"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid passphrase in network result")
	}

	return passphrase, nil
}

// Block Operations methods
func (h *StellarRpcHandler) FormatBlockHeight(sequenceNumber int64) string {
	return strconv.FormatInt(sequenceNumber, 10) // Decimal format for Stellar ledger sequence
}

func (h *StellarRpcHandler) CreateBlockRequest(method string, sequenceNumber int64, _ bool) ([]byte, error) {
	// Stellar RPC getLedgers expects a start ledger sequence
	// For a single ledger, we set limit=1
	payload := fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","id":1,"params":{"startLedger":%d, "pagination": {"limit": 1}}}`,
		method, sequenceNumber,
	)
	return []byte(payload), nil
}

func (h *StellarRpcHandler) ParseBlockResponse(body []byte) (interface{}, error) {
	// First check for JSON-RPC errors using generic response
	var genericResponse din_http.JSONRPCResponse
	if err := json.Unmarshal(body, &genericResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON-RPC response: %w", err)
	}

	// Check for JSON-RPC errors in getLedgers response
	if genericResponse.Error != nil {
		return nil, fmt.Errorf("stellar getLedgers error: %s", genericResponse.Error.Message)
	}

	// Parse as Stellar-specific response if no errors
	var response din_http.JSONRPCStellarLedgerResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Stellar ledger response: %w", err)
	}

	return response, nil
}

// ParseBlockNumberResponse parses the block number from getHealth response
func (h *StellarRpcHandler) ParseBlockNumberResponse(body []byte, statusCode int) (int64, error) {
	// Check HTTP status first
	if statusCode >= 400 {
		if statusCode == 429 {
			return 0, fmt.Errorf("rate limit error (status code: %d)", statusCode)
		}
		return 0, fmt.Errorf("error status code: %d", statusCode)
	}

	blockInfo, err := h.ParseHealthCheckResponse(body)
	if err != nil {
		return 0, fmt.Errorf("failed to parse Stellar health check response: %w", err)
	}

	return blockInfo.Number, nil
}

// RequiresSeparateBlockInfoCall returns false for Stellar as health check includes block info
func (h *StellarRpcHandler) RequiresSeparateBlockInfoCall() bool {
	return false
}

// GetBlockInfoMethod returns empty for Stellar as it uses the same endpoint for health and block info
func (h *StellarRpcHandler) GetBlockInfoMethod() string {
	return "" // Not used for JSON-RPC chains
}

// GetChainID retrieves the chain ID (network passphrase) from the Stellar provider
func (h *StellarRpcHandler) GetChainID(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int) (string, error) {
	// Use custom logic for Stellar's getNetwork method
	return h.getNetworkPassphrase(httpUrl, headers, httpClient, authClient, requestAttempts)
}

// getNetworkPassphrase retrieves the network passphrase using getNetwork method
func (h *StellarRpcHandler) getNetworkPassphrase(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int) (string, error) {
	// Create JSON-RPC payload for getNetwork request
	payload := []byte(`{"jsonrpc":"2.0","method":"getNetwork","id":1}`)

	var lastErr error
	for attempt := 0; attempt < requestAttempts; attempt++ {
		// Make POST request with payload
		resBytes, statusCode, err := httpClient.Post(httpUrl, headers, payload, authClient)
		if err != nil {
			lastErr = fmt.Errorf("error sending HTTP request: %w", err)
			continue
		}

		// Parse response
		passphrase, err := h.ParseChainIDResponse(resBytes, *statusCode)
		if err != nil {
			lastErr = err
			continue
		}

		// Success!
		return passphrase, nil
	}

	return "", fmt.Errorf("failed after %d attempts: %w", requestAttempts, lastErr)
}

// Archive Mode methods
func (h *StellarRpcHandler) SupportsArchiveMode() bool {
	return false // Stellar RPC retains 7 days of ledgers by default, no special archive mode needed
}

func (h *StellarRpcHandler) GetArchiveMethod() string {
	return ""
}

func (h *StellarRpcHandler) CreateArchivePayload(method string, blockHeight string) ([]byte, error) {
	return nil, fmt.Errorf("Stellar does not require special archive mode")
}

func (h *StellarRpcHandler) ParseArchiveResponse(body []byte) error {
	return fmt.Errorf("Stellar does not require special archive mode")
}

// Network Capabilities methods
func (h *StellarRpcHandler) SupportsGetBlockByNumber() bool {
	return true
}

func (h *StellarRpcHandler) SupportsDynamicBlockLag() bool {
	return true
}

func (h *StellarRpcHandler) GetSupportedMethods() []string {
	return []string{
		"getEvents",
		"getFeeStats",
		"getHealth",
		"getLatestLedger",
		"getLedgerEntries",
		"getLedgers",
		"getNetwork",
		"getTransaction",
		"getTransactions",
		"getVersionInfo",
		"sendTransaction",
		"simulateTransaction",
	}
}

func (h *StellarRpcHandler) GetBlockByNumberMethod() string {
	return "getLedgers"
}

// Data Format Conversions methods
func (h *StellarRpcHandler) ExtractBlockHash(blockResponse interface{}) string {
	if stellarResponse, ok := blockResponse.(din_http.JSONRPCStellarLedgerResponse); ok {
		if len(stellarResponse.Result.Ledgers) == 0 {
			return ""
		}
		return stellarResponse.Result.Ledgers[0].Hash
	}
	return ""
}

// Health Check Specifics methods
func (h *StellarRpcHandler) GetHealthCheckMethod() string {
	// getHealth is used for health checks
	return "getHealth"
}

func (h *StellarRpcHandler) GetHealthCheckHTTPMethod() string {
	return "POST" // Stellar uses JSON-RPC POST requests
}
func (h *StellarRpcHandler) CreateHealthCheckPayload(method string) ([]byte, error) {
	payload := fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","id":1}`, method)
	return []byte(payload), nil
}

func (h *StellarRpcHandler) ParseHealthCheckResponse(body []byte) (*BlockInfo, error) {
	// First check for JSON-RPC errors using generic response
	var genericResponse din_http.JSONRPCResponse
	if err := json.Unmarshal(body, &genericResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON-RPC response: %w", err)
	}

	// Check for JSON-RPC errors in getHealth response
	if genericResponse.Error != nil {
		return nil, fmt.Errorf("stellar getHealth error: %s", genericResponse.Error.Message)
	}

	// Parse as Stellar-specific response if no errors
	var response din_http.JSONRPCStellarGetHealthResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Stellar health check response: %w", err)
	}

	return &BlockInfo{
		Number:    response.Result.LatestLedger,
		Hash:      "",
		Timestamp: time.Now(),
	}, nil
}

func (h *StellarRpcHandler) GetChainIDMethod() string {
	return "getNetwork"
}

func (h *StellarRpcHandler) ParseChainIDResponse(body []byte, statusCode int) (string, error) {
	if statusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP error: %d", statusCode)
	}

	var respObject map[string]interface{}
	if err := json.Unmarshal(body, &respObject); err != nil {
		return "", fmt.Errorf("failed to parse Stellar network response: %w", err)
	}

	// Check for JSON-RPC error
	if errorField, exists := respObject["error"]; exists && errorField != nil {
		return "", fmt.Errorf("JSON-RPC error: %v", errorField)
	}

	// Extract result field
	result, ok := respObject["result"]
	if !ok {
		return "", fmt.Errorf("missing result field in network response")
	}

	// Extract chain reference (network passphrase)
	chainReference, err := h.ExtractChainReference(result)
	if err != nil {
		return "", fmt.Errorf("failed to extract network passphrase: %w", err)
	}

	return chainReference, nil
}

// GetLatestBlockNumber retrieves the latest ledger sequence for Stellar chains
func (h *StellarRpcHandler) GetLatestBlockNumber(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int) (*LatestBlockResult, error) {
	// Create JSON-RPC payload for latest block number request
	payload := []byte(`{"jsonrpc":"2.0","method":"getLatestLedger","id":1}`)
	return GetLatestBlockNumberViaJSONRPC(
		httpUrl,
		payload,
		headers,
		httpClient,
		authClient,
		requestAttempts,
		ParseSequenceNumber,
	)
}

// ParseSequenceNumber parses the sequence number from the getLatestLedger result object:
//
//	{
//	  "result": {
//	    "id": "<ledger hash>",
//	    "sequence": 2002985,
//	    "closeTime": "<ISO 8601 timestamp>",
//	    "headerXdr": "<base64 encoded ledger header>"
//	  }
//	}
func ParseSequenceNumber(result json.RawMessage) (int64, error) {
	var resObj map[string]interface{}
	if err := json.Unmarshal(result, &resObj); err != nil {
		return 0, fmt.Errorf("failed to unmarshal getLatestLedger result: %w", err)
	}
	seq, ok := resObj["sequence"]
	if !ok {
		return 0, fmt.Errorf("missing sequence field in getLatestLedger result")
	}
	switch v := seq.(type) {
	case float64:
		return int64(v), nil
	case int64:
		return v, nil
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse sequence as int64: %w", err)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unsupported type for sequence field: %T", seq)
	}
}

// PerformArchiveCheck performs archive mode check for Stellar chains
func (h *StellarRpcHandler) PerformArchiveCheck(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, blockHeight string) error {
	// Stellar RPC retains 7 days of ledgers by default, no special archive mode
	return fmt.Errorf("Stellar does not support archive mode")
}

// PerformGetBlockByNumber performs get ledger by sequence operation for Stellar chains
func (h *StellarRpcHandler) PerformGetBlockByNumber(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, sequenceNumber int64) (interface{}, error) {
	return PerformGetBlockByNumberViaJSONRPC(
		httpUrl,
		headers,
		httpClient,
		authClient,
		requestAttempts,
		sequenceNumber,
		h.GetBlockByNumberMethod(), // "getLedgers"
		h.CreateBlockRequest,       // Stellar-specific block request creation
		h.ParseBlockResponse,       // Stellar-specific block response parsing
	)
}

// GetBlockTimestamp retrieves the Unix timestamp for a specific ledger sequence
func (h *StellarRpcHandler) GetBlockTimestamp(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, sequenceNumber int64) (int64, error) {
	// Get ledger data
	ledgerData, err := h.PerformGetBlockByNumber(httpUrl, headers, httpClient, authClient, requestAttempts, sequenceNumber)
	if err != nil {
		return 0, fmt.Errorf("failed to get ledger %d: %w", sequenceNumber, err)
	}

	// Parse timestamp from ledger response
	ledgerResponse, ok := ledgerData.(din_http.JSONRPCStellarLedgerResponse)
	if !ok {
		return 0, fmt.Errorf("invalid ledger response type: %T", ledgerData)
	}

	// Check if there are any ledgers in the response
	if len(ledgerResponse.Result.Ledgers) == 0 {
		return 0, fmt.Errorf("no ledger found in response for sequence %d", sequenceNumber)
	}

	ledger := ledgerResponse.Result.Ledgers[0]

	// Stellar ledgers have ledgerCloseTime as Unix timestamp string
	ledgerCloseTime := ledger.LedgerCloseTime
	if ledgerCloseTime == "" {
		return 0, fmt.Errorf("missing closedAt in ledger %d", sequenceNumber)
	}

	// Parse Unix timestamp string to int64
	timestamp, err := strconv.ParseInt(ledgerCloseTime, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse closedAt timestamp: %w", err)
	}

	return timestamp, nil
}
