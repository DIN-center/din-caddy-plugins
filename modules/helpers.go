package modules

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	dinHttp "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	prom "github.com/DIN-center/din-caddy-plugins/lib/prometheus"
	"github.com/caddyserver/caddy/v2"
	"go.uber.org/zap"
)

// checkForJSONRPCError checks if a response body contains a JSON-RPC error
// even when the HTTP status code is 200. Returns the error if found, nil otherwise.
func checkForJSONRPCError(responseBody []byte) *dinHttp.JSONRPCError {
	if len(responseBody) == 0 {
		return nil
	}

	// Use the existing decompression function to handle gzipped content
	// Note: We pass empty headers since we're detecting gzip by magic number
	// The decompressGzipBodyIfNecessary function will handle the detection
	processedBody := responseBody
	if len(responseBody) >= 2 && responseBody[0] == 0x1f && responseBody[1] == 0x8b {
		// Create headers to indicate gzip encoding
		headers := make(http.Header)
		headers.Set("Content-Encoding", "gzip")

		// Use the existing decompression function
		processedBody = decompressGzipBodyIfNecessary(headers, responseBody, nil, "checkForJSONRPCError")
	}

	var response dinHttp.JSONRPCResponse
	if err := json.Unmarshal(processedBody, &response); err != nil {
		// If we can't unmarshal as JSON-RPC, it's not a JSON-RPC error
		return nil
	}
	// Return the error if present, nil otherwise
	return response.Error
}

// isJSONRPCErrorRetryable determines if a JSON-RPC error should trigger a retry
// based on the error code. Returns true if the error might be resolved by retrying
// with a different provider, false if retrying won't help.
func isJSONRPCErrorRetryable(jsonRPCError *dinHttp.JSONRPCError) bool {
	if jsonRPCError == nil {
		return false
	}

	switch jsonRPCError.Code {
	// Standard JSON-RPC errors that are NOT retryable (client/request issues)
	case -32700: // Parse error - malformed JSON
		return false
	case -32600: // Invalid Request - malformed request object
		return false
	case -32601: // Method not found - method doesn't exist
		return false
	case -32602: // Invalid params - wrong parameters
		return false

	// Standard JSON-RPC errors that ARE retryable (server issues)
	case -32603: // Internal error - server-side issue
		return true

	// Server error range (-32000 to -32099) - these are typically retryable
	case -32000, -32001, -32002, -32003, -32004, -32005, -32006, -32007, -32008, -32009,
		-32010, -32011, -32012, -32013, -32014, -32015, -32016, -32017, -32018, -32019,
		-32020, -32021, -32022, -32023, -32024, -32025, -32026, -32027, -32028, -32029,
		-32030, -32031, -32032, -32033, -32034, -32035, -32036, -32037, -32038, -32039,
		-32040, -32041, -32042, -32043, -32044, -32045, -32046, -32047, -32048, -32049,
		-32050, -32051, -32052, -32053, -32054, -32055, -32056, -32057, -32058, -32059,
		-32060, -32061, -32062, -32063, -32064, -32065, -32066, -32067, -32068, -32069,
		-32070, -32071, -32072, -32073, -32074, -32075, -32076, -32077, -32078, -32079,
		-32080, -32081, -32082, -32083, -32084, -32085, -32086, -32087, -32088, -32089,
		-32090, -32091, -32092, -32093, -32094, -32095, -32096, -32097, -32098, -32099:
		// Server error range - these are typically retryable
		return true

	// Application-specific errors - analyze by message content for common patterns
	default:
		// For unknown error codes, check message content for common patterns
		message := strings.ToLower(jsonRPCError.Message)

		// Non-retryable patterns (client/request issues)
		nonRetryablePatterns := []string{
			"method not found",
			"invalid params",
			"unauthorized",
			"forbidden",
		}

		for _, pattern := range nonRetryablePatterns {
			if strings.Contains(message, pattern) {
				return false
			}
		}

		// Retryable patterns (server/network issues)
		retryablePatterns := []string{
			"timeout",
			"connection",
			"network",
			"rate limit",
			"server error",
		}

		for _, pattern := range retryablePatterns {
			if strings.Contains(message, pattern) {
				return true
			}
		}

		// For unknown error codes with no recognizable patterns, default to retryable to be safe
		// This ensures we don't miss potentially transient issues
		return true
	}
}

func getRequestBody(repl *caddy.Replacer) (*dinHttp.JSONRPCRequest, error) {
	if v, ok := repl.Get(RequestBodyKey); ok {
		bodyBytes, ok := v.([]byte)
		if !ok {
			return nil, fmt.Errorf("request body is not a byte array")
		}

		var request dinHttp.JSONRPCRequest
		err := json.Unmarshal(bodyBytes, &request)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal request body: %w", err)
		}

		return &request, nil
	}

	// If the request body is not found, return nil
	return nil, nil
}

func getRequestMethod(repl *caddy.Replacer) (string, error) {
	method, ok := repl.Get(RequestMethodKey)
	if !ok {
		return "", fmt.Errorf("request method not found")
	}

	methodStr, ok := method.(string)
	if !ok {
		return "", fmt.Errorf("request method is not a string")
	}

	return methodStr, nil
}

// decompressGzipBodyIfNecessary checks if the body is gzipped based on headers
// and attempts to decompress it. It returns the processed body (decompressed or original).
func decompressGzipBodyIfNecessary(headers http.Header, bodyBytes []byte, lg *logger.LoggerClient, networkPath string) []byte {
	if headers.Get("Content-Encoding") == "gzip" {
		bReader := bytes.NewReader(bodyBytes)
		gzr, errDecompress := gzip.NewReader(bReader)
		if errDecompress == nil {
			decompressedBody, errRead := io.ReadAll(gzr)
			if errRead == nil {
				bodyBytes = decompressedBody // Update bodyBytes with decompressed data
			} else {
				lg.Warn("Failed to read decompressed gzip body", zap.Error(errRead), zap.String("network", networkPath))
			}
			// It's important to close the gzip.Reader.
			// Defer is not suitable here as we want to close it before returning from this block.
			if errClose := gzr.Close(); errClose != nil {
				lg.Warn("Failed to close gzip reader", zap.Error(errClose), zap.String("network", networkPath))
			}
		} else {
			lg.Warn("Failed to create gzip reader for body decompression", zap.Error(errDecompress), zap.String("network", networkPath))
		}
	}
	return bodyBytes
}

// LogFailedAttemptParams holds all the configuration data needed for logging failed attempts
type LogFailedAttemptParams struct {
	Reason              string
	Logger              *logger.LoggerClient
	NetworkPath         string
	FailedAttemptNumber int // 1-indexed
	MaxAttempts         int
	StatusCodeOfFailure int
	Error               error                   // Can be nil
	Replacer            *caddy.Replacer         // Caddy replacer to get context data
	ParsedReqBody       *dinHttp.JSONRPCRequest // Parsed request body
	RawResponseBody     []byte                  // Raw response body to parse internally
}

// Helper function to log a failed request attempt that will be retried.
// This function is intended to be run as a goroutine to avoid blocking the main request flow.
// It extracts comprehensive context from the request/response data and logs it in a structured format
// for debugging and monitoring purposes. The function handles both successful JSON parsing and fallback
// scenarios where parsing fails, ensuring robust logging in all cases.
func logFailedAttempt(params *LogFailedAttemptParams) {
	// --- Extract data within the async function ---
	// All data extraction happens within this goroutine to avoid race conditions
	// and ensure we have a complete snapshot of the request state at failure time.

	// Extract provider information from the Caddy replacer context
	// Default to "unknown" if provider information is not available or malformed
	provider := "unknown"
	if provVal, provOk := params.Replacer.Get(RequestProviderKey); provOk {
		if pStr, strOk := provVal.(string); strOk {
			provider = pStr
		}
	}

	// Initialize variables for request method and parameters extraction
	var requestMethod string
	var requestParams json.RawMessage
	rawRequestBodySnippet := ""

	// Extract raw request body for logging purposes
	// We always try to get the raw body first as it's useful for debugging
	// even if JSON parsing fails later
	if v, okGet := params.Replacer.Get(RequestBodyKey); okGet {
		if bodyBytes, okCast := v.([]byte); okCast && len(bodyBytes) > 0 {
			// Limit the snippet length to 500 characters to avoid overwhelming logs
			// while still providing sufficient context for debugging
			length := 500 // Increased from 100 to 500 for more context
			if len(bodyBytes) < length {
				length = len(bodyBytes)
			}
			rawRequestBodySnippet = string(bodyBytes[:length])
		}
	}

	// Attempt to extract structured request data from the parsed request body
	// This provides the cleanest data when available
	if params.ParsedReqBody != nil {
		requestMethod = params.ParsedReqBody.Method
		if len(params.ParsedReqBody.Params) > 0 {
			requestParams = params.ParsedReqBody.Params
		}
	} else if rawRequestBodySnippet != "" {
		// Fallback: Try to extract method from raw request body if parsing failed
		// This ensures we can still get method information even when the main parsing fails
		var tempReq struct {
			Method string `json:"method"`
		}
		if v, okGet := params.Replacer.Get(RequestBodyKey); okGet {
			if bodyBytes, okCast := v.([]byte); okCast && len(bodyBytes) > 0 {
				// First attempt: Try to unmarshal just the method field
				if err := json.Unmarshal(bodyBytes, &tempReq); err == nil && tempReq.Method != "" {
					requestMethod = tempReq.Method
				} else {
					// Last resort: Use regex to extract method if JSON parsing completely fails
					// This handles cases where the request body might be malformed JSON
					methodRegex := regexp.MustCompile(`"method"\s*:\s*"([^"]+)"`)
					if matches := methodRegex.FindSubmatch(bodyBytes); len(matches) > 1 {
						requestMethod = string(matches[1])
					}
				}
			}
		}
	}

	// Parse response body to extract error information and determine response type
	// This helps identify the nature of the failure (JSON-RPC error vs HTTP error vs malformed response)
	var jsonRPCError *dinHttp.JSONRPCError
	var rawResponseSnippet string
	var parsedResponseType string

	if len(params.RawResponseBody) > 0 {
		// Handle gzip-compressed response bodies
		// Check for gzip magic number (0x1f, 0x8b) to detect compressed content
		processedResponseBody := params.RawResponseBody
		if len(params.RawResponseBody) >= 2 && params.RawResponseBody[0] == 0x1f && params.RawResponseBody[1] == 0x8b {
			// Create headers to indicate gzip encoding for the decompression function
			headers := make(http.Header)
			headers.Set("Content-Encoding", "gzip")
			processedResponseBody = decompressGzipBodyIfNecessary(headers, params.RawResponseBody, params.Logger, params.NetworkPath)
		}

		// Create a snippet of the processed response for logging
		// Limited to 500 characters to balance detail with log readability
		length := 500
		if len(processedResponseBody) < length {
			length = len(processedResponseBody)
		}
		rawResponseSnippet = string(processedResponseBody[:length])

		// Attempt to parse as JSON-RPC response first using the processed (potentially decompressed) body
		// JSON-RPC responses have a specific structure that we want to extract error information from
		var jsonRPCResponse dinHttp.JSONRPCResponse
		if err := json.Unmarshal(processedResponseBody, &jsonRPCResponse); err == nil {
			parsedResponseType = "jsonrpc"
			// Extract JSON-RPC error information if present
			// This is crucial for understanding the specific nature of RPC failures
			if jsonRPCResponse.Error != nil {
				jsonRPCError = jsonRPCResponse.Error
			}
		} else {
			// Fallback: Try to parse as generic JSON to categorize the response type
			var genericJSON interface{}
			if err := json.Unmarshal(processedResponseBody, &genericJSON); err == nil {
				parsedResponseType = "json"
			} else {
				// If it's not valid JSON at all, mark it as raw content
				parsedResponseType = "raw"
			}
		}
	}
	// --- End data extraction ---

	// Build the base log fields that are always present
	// These provide the core context for understanding the failure
	logFields := []zap.Field{
		zap.String("network", params.NetworkPath),                    // Which network/blockchain
		zap.String("provider", provider),                             // Which RPC provider failed
		zap.Int("failed_attempt_number", params.FailedAttemptNumber), // Current retry attempt
		zap.Int("max_attempts", params.MaxAttempts),                  // Total retry attempts configured
		zap.Int("status_code", params.StatusCodeOfFailure),           // HTTP status code of the failure
		zap.String("reason", params.Reason),                          // High-level reason for the failure
	}

	// Add error information if an error object was provided
	// This captures Go errors that occurred during request processing
	if params.Error != nil {
		logFields = append(logFields, zap.NamedError("error", params.Error))
	}

	// Add JSON-RPC specific error information if present
	// JSON-RPC errors provide structured error codes and messages that are valuable for debugging
	if jsonRPCError != nil {
		logFields = append(logFields,
			zap.Int("jsonrpc_error_code", jsonRPCError.Code),          // Standard JSON-RPC error code
			zap.String("jsonrpc_error_message", jsonRPCError.Message)) // Human-readable error message
		// Include additional error data if provided by the RPC server
		if jsonRPCError.Data != nil {
			logFields = append(logFields, zap.Any("jsonrpc_error_data", jsonRPCError.Data))
		}
	}

	// Add request method information if available
	// The RPC method helps identify which specific operation failed
	if requestMethod != "" {
		logFields = append(logFields, zap.String("request_method", requestMethod))
	}

	// Add structured request parameters if available
	// Parameters help understand the specific request that failed
	if len(requestParams) > 0 && string(requestParams) != "null" {
		var decodedParams interface{}
		// Attempt to unmarshal parameters for structured logging
		if errUnmarshal := json.Unmarshal(requestParams, &decodedParams); errUnmarshal == nil {
			logFields = append(logFields, zap.Any("request_params", decodedParams))
		} else {
			// Fallback to raw string if structured parsing fails
			logFields = append(logFields, zap.String("raw_request_params", string(requestParams)))
			// Log the parsing failure separately for debugging
			params.Logger.Debug("Async retry error log: Failed to unmarshal requestParams for structured logging, logging as raw string.",
				zap.Error(errUnmarshal),
				zap.String("raw_params_attempted", string(requestParams)))
		}
	}

	// Always log raw request body snippet if available
	// This provides the complete request context even when structured parsing fails
	if rawRequestBodySnippet != "" {
		// Try to parse as JSON for structured logging first
		var requestBodyJSON interface{}
		if err := json.Unmarshal([]byte(rawRequestBodySnippet), &requestBodyJSON); err == nil {
			logFields = append(logFields, zap.Any("raw_request_body", requestBodyJSON))
		} else {
			// If not valid JSON, log as string to preserve the exact content
			logFields = append(logFields, zap.String("raw_request_body", rawRequestBodySnippet))
		}
	}

	// Log response information to understand what the server returned
	// This is crucial for diagnosing whether the issue is with the request or the server's response
	if rawResponseSnippet != "" {
		// Try to parse as JSON for structured logging
		var responseBodyJSON interface{}
		if err := json.Unmarshal([]byte(rawResponseSnippet), &responseBodyJSON); err == nil {
			logFields = append(logFields,
				zap.Any("raw_response_body", responseBodyJSON),  // Structured response data
				zap.String("response_type", parsedResponseType)) // Type classification
		} else {
			// If not valid JSON, log as string along with type classification
			logFields = append(logFields,
				zap.String("raw_response_body", rawResponseSnippet),
				zap.String("response_type", parsedResponseType))
		}
	}

	// Emit the final warning log with all collected context
	// This creates a comprehensive log entry that contains all available information
	// about the failed request attempt, making it easier to debug issues
	params.Logger.Warn("Request attempt failed, initiating retry", logFields...)
}

// PostRequestTaskParams holds all parameters for handlePostRequestTasks.
// Using a struct helps in managing parameters and improving code readability.
// Ensure all fields are exported if this struct needs to be instantiated outside this package directly.
// For now, assuming it's instantiated and used within the same package.
type PostRequestTaskParams struct {
	DinMiddleware *DinMiddleware
	RWWrapper     *ResponseWriterWrapper
	NetworkObj    *network
	NetworkPath   string
	Provider      string
	Replacer      *caddy.Replacer
	Duration      time.Duration
	OriginalReq   *http.Request
	ParsedReqBody *dinHttp.JSONRPCRequest
}

// handlePostRequestTasks encapsulates logic that runs after the main response has been written.
// This includes body decompression, asynchronous HCMethod processing, and metrics reporting.
func handlePostRequestTasks(params PostRequestTaskParams) {
	// Initialize default values for status code, headers, and response body.
	// These are used in case the ResponseWriterWrapper (rww) is nil or doesn't provide them.
	effectiveStatusCode := http.StatusInternalServerError
	responseHeaders := make(http.Header)
	var rawResponseBody []byte

	// Check if the ResponseWriterWrapper is available.
	// This wrapper captures the response details (status code, headers, body) from the downstream handler.
	if params.RWWrapper != nil {
		params.DinMiddleware.logger.Debug("rww is not nil during post-processing.", zap.String("network", params.NetworkPath))
		// Use the actual status code from the response.
		effectiveStatusCode = params.RWWrapper.statusCode
		// Use the actual headers from the response.
		responseHeaders = params.RWWrapper.Header()

		// Check if the response body was captured.
		if params.RWWrapper.body != nil {
			params.DinMiddleware.logger.Debug("rww.body is not nil. Attempting to read bytes.", zap.String("network", params.NetworkPath), zap.Int("body_len_approx", params.RWWrapper.body.Len()))
			// Copy the response body bytes to avoid issues with the original buffer being modified.
			sourceBytes := params.RWWrapper.body.Bytes()
			rawResponseBody = make([]byte, len(sourceBytes))
			copy(rawResponseBody, sourceBytes)
		} else {
			// If the body in the wrapper is nil, use an empty byte slice.
			params.DinMiddleware.logger.Debug("rww.body is nil; using empty slice for rawResponseBody.", zap.String("network", params.NetworkPath))
			rawResponseBody = []byte{}
		}
	} else {
		// Log a warning if the ResponseWriterWrapper is nil, as this is unexpected.
		// The default initialized values for rawResponseBody (empty byte slice) will be used.
		params.DinMiddleware.logger.Warn("rww is nil before post-request processing. This is unexpected.", zap.String("network", params.NetworkPath))
		rawResponseBody = []byte{} // Ensure rawResponseBody is an empty slice, not nil
	}

	// Decompress the response body if it's GZIP encoded.
	// This is necessary to inspect the content of the response, e.g., for health check processing.
	processedResponseBody := decompressGzipBodyIfNecessary(responseHeaders, rawResponseBody, params.DinMiddleware.logger, params.NetworkPath)

	// Attempt to get the RPC method from the request.
	// This is used to determine if the request was for a health check method.
	requestMethod, errGetMethod := getRequestMethod(params.Replacer)
	if errGetMethod != nil {
		// Log a warning if the method cannot be retrieved, and skip health check processing.
		params.DinMiddleware.logger.Warn("Failed to get request method for HCMethod check; skipping async HC processing.", zap.Error(errGetMethod), zap.String("network", params.NetworkPath))
	} else {
		// Asynchronously process the response if it's for a configured health check method.
		// This involves extracting block numbers and updating network/provider health status.
		// It's run in a goroutine to avoid blocking the main request-response flow.
		// Ensure NetworkObj is not nil before checking health check method to prevent panics.
		if params.NetworkObj != nil && requestMethod == params.NetworkObj.getHealthCheckMethod() {
			go params.DinMiddleware.processHCMethodResponseAsync(params.NetworkObj, params.NetworkPath, processedResponseBody, effectiveStatusCode, requestMethod)
		}
	}

	// Determine the health status of the provider that handled the request.
	// This information is used for Prometheus metrics.
	healthStatus := "unknown" // Default health status.
	// Ensure NetworkObj is not nil before accessing its Providers map.
	if params.Provider != "" && params.NetworkObj != nil {
		// Look up the provider in the network's provider map.
		if providerObj, ok := params.NetworkObj.Providers[params.Provider]; ok && providerObj != nil {
			// Get the latest block entry for the provider to determine its health.
			latestBlock := providerObj.getLatestBlockEntry()
			if latestBlock != nil {
				healthStatus = latestBlock.healthStatus.String()
			} else {
				// Log if the provider has no block entries yet.
				params.DinMiddleware.logger.Debug("Provider has no block entries yet for metrics.", zap.String("provider", params.Provider), zap.String("network", params.NetworkPath))
			}
		} else {
			// Log a warning if the provider from the replacer is not found in the network's map.
			params.DinMiddleware.logger.Warn("Provider from replacer not found in network's providers map for metrics.", zap.String("provider", params.Provider), zap.String("network", params.NetworkPath))
		}
	} else if params.NetworkObj == nil {
		// Log a warning if NetworkObj is nil during health status retrieval.
		params.DinMiddleware.logger.Warn("networkObj is nil during health status retrieval for metrics.", zap.String("network", params.NetworkPath))
	} else { // provider is ""
		// Log a warning if the provider key was not found in the replacer.
		params.DinMiddleware.logger.Warn("Provider key not found in replacer for metrics reporting.", zap.String("network", params.NetworkPath))
	}

	// Skip Prometheus metrics reporting if in test mode.
	if params.DinMiddleware.testMode {
		return
	}

	// Ensure the Prometheus client is initialized before attempting to record metrics.
	if params.DinMiddleware.PrometheusClient == nil {
		params.DinMiddleware.logger.Error("PrometheusClient is nil in handlePostRequestTasks, skipping metrics.")
		return
	}

	// Record Prometheus metrics for the request.
	// This includes details like network, provider, response status, health status, and duration.
	params.DinMiddleware.PrometheusClient.HandleRequestMetrics(&prom.PromRequestMetricData{
		Method:         requestMethod,
		Network:        params.NetworkPath,
		Provider:       params.Provider,
		HostName:       params.OriginalReq.Host,
		ResponseStatus: effectiveStatusCode,
		HealthStatus:   healthStatus,
		Environment:    string(params.DinMiddleware.Env),
	}, params.Duration, params.ParsedReqBody)
}

// createHealthCheckRequestContext creates synthetic request context for health checks
// so we can reuse logFailedAttempt for consistent logging
func createHealthCheckRequestContext(networkName, providerHost, method string) (*caddy.Replacer, *dinHttp.JSONRPCRequest, []byte) {
	// Create a synthetic replacer with health check context
	repl := caddy.NewReplacer()
	repl.Set(RequestProviderKey, providerHost)

	// Create payload using handler-aware logic
	payload := createHealthCheckPayload(method)
	repl.Set(RequestBodyKey, payload)

	// Extract params from payload for accurate JSONRPCRequest
	var params json.RawMessage
	var tempReq struct {
		Params json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(payload, &tempReq); err == nil && len(tempReq.Params) > 0 {
		params = tempReq.Params
	} else {
		// Default to empty array for health checks
		params = json.RawMessage(`[]`)
	}

	// Create synthetic JSONRPCRequest
	jsonRPCReq := &dinHttp.JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		ID:      json.RawMessage(`1`),
		Params:  params,
	}

	return repl, jsonRPCReq, payload
}

// createHealthCheckPayload creates health check payload with handler support
// This is a helper function that can be used independently of network objects
func createHealthCheckPayload(method string) []byte {
	// For now, use standard JSON-RPC format since we don't have network context
	// This maintains backward compatibility while allowing future handler integration
	payload := fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","id":1}`, method)
	return []byte(payload)
}

// createGetBlockByNumberRequestContext creates synthetic request context for getBlockByNumber calls
// so we can reuse logFailedAttempt for consistent logging
func createGetBlockByNumberRequestContext(networkName, providerHost, method string, blockNumber int64, networkObj *network) (*caddy.Replacer, *dinHttp.JSONRPCRequest, []byte) {
	// Create a synthetic replacer with getBlockByNumber context
	repl := caddy.NewReplacer()
	repl.Set(RequestProviderKey, providerHost)

	var payload []byte
	var params json.RawMessage

	// PRIMARY: Use handler method for network-specific block request creation
	if networkObj != nil && networkObj.handler != nil {
		var err error
		payload, err = networkObj.handler.CreateBlockRequest(method, blockNumber, false)
		if err != nil {
			// Log handler failure and fall back to legacy logic
			if networkObj.logger != nil {
				networkObj.logger.Warn("Handler failed to create block request, falling back to legacy logic",
					zap.String("network", networkName),
					zap.String("method", method),
					zap.Int64("block_number", blockNumber),
					zap.Error(err))
			}
			// Fallback to legacy logic
			payload, params = createLegacyBlockRequest(networkName, method, blockNumber)
		} else {
			// Extract params from handler-created payload
			var tempReq struct {
				Params json.RawMessage `json:"params"`
			}
			if err := json.Unmarshal(payload, &tempReq); err == nil && len(tempReq.Params) > 0 {
				params = tempReq.Params
			} else {
				// If params extraction fails, fall back to legacy params
				_, params = createLegacyBlockRequest(networkName, method, blockNumber)
			}
		}
	} else {
		// FALLBACK: Use legacy logic when no handler available
		payload, params = createLegacyBlockRequest(networkName, method, blockNumber)
	}

	repl.Set(RequestBodyKey, payload)

	// Create synthetic JSONRPCRequest
	jsonRPCReq := &dinHttp.JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		ID:      json.RawMessage(`1`),
		Params:  params,
	}

	return repl, jsonRPCReq, payload
}

// createLegacyBlockRequest creates block request using legacy string-based logic
// This maintains backward compatibility for networks without handlers
func createLegacyBlockRequest(networkName, method string, blockNumber int64) ([]byte, json.RawMessage) {
	var payload []byte
	var params json.RawMessage

	// Use network name patterns to determine format (legacy approach)
	if strings.Contains(networkName, "solana") {
		// Solana format: [blockNumber, {"encoding": "json", "transactionDetails": "none", "rewards": false}]
		payload = []byte(fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","id":1,"params":[%d, {"encoding": "json", "transactionDetails": "none", "rewards": false}]}`, method, blockNumber))
		params = json.RawMessage(fmt.Sprintf(`[%d, {"encoding": "json", "transactionDetails": "none", "rewards": false}]`, blockNumber))
	} else {
		// EVM format (default): ["0x{blockNumberHex}", false]
		blockNumberHex := fmt.Sprintf("%#x", blockNumber)
		payload = []byte(fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","id":1,"params":["%s", false]}`, method, blockNumberHex))
		params = json.RawMessage(fmt.Sprintf(`["%s", false]`, blockNumberHex))
	}

	return payload, params
}

// checkRequestContext checks if the request context has been cancelled or exceeded deadline
// Returns an error if the context is done, nil if the context is still active
func checkRequestContext(l *logger.LoggerClient, r *http.Request, networkPath string, attempt int) error {
	select {
	case <-r.Context().Done():
		switch r.Context().Err() {
		case context.Canceled:
			l.Debug("Request cancelled by client",
				zap.String("network", networkPath),
				zap.Int("attempt", attempt+1))
			return fmt.Errorf("request cancelled by client")
		case context.DeadlineExceeded:
			l.Debug("Request deadline exceeded",
				zap.String("network", networkPath),
				zap.Int("attempt", attempt+1))
			return fmt.Errorf("request deadline exceeded")
		default:
			l.Debug("Request context error",
				zap.String("network", networkPath),
				zap.Int("attempt", attempt+1),
				zap.Error(r.Context().Err()))
			return fmt.Errorf("request context error: %w", r.Context().Err())
		}
	default:
		// Context is still active, continue
		return nil
	}
}

// handleContextCancellation handles context cancellation by returning appropriate HTTP responses
// and logging using the standard logFailedAttempt format for consistency
func handleContextCancellation(l *logger.LoggerClient, promClient *prom.PrometheusClient, rw http.ResponseWriter, r *http.Request, networkPath string, attempt int, err error, reqStartTime time.Time, networkObj *network) {
	// Calculate duration
	duration := time.Since(reqStartTime)
	// Determine the appropriate HTTP status code and response based on the error type
	var statusCode int
	var responseBody string

	switch {
	case strings.Contains(err.Error(), "cancelled by client"):
		statusCode = http.StatusRequestTimeout // 408
		responseBody = `{"error": "Request cancelled by client", "code": 408}`
	case strings.Contains(err.Error(), "deadline exceeded"):
		statusCode = http.StatusGatewayTimeout // 504
		responseBody = `{"error": "Request timeout exceeded", "code": 504}`
	default:
		statusCode = http.StatusServiceUnavailable // 503
		responseBody = `{"error": "Service unavailable", "code": 503}`
	}

	// Get request context data
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)

	// Parse request body for consistent logging and metrics
	var parsedReqBody *dinHttp.JSONRPCRequest
	if v, ok := repl.Get(RequestBodyKey); ok {
		if bodyBytes, ok := v.([]byte); ok && len(bodyBytes) > 0 {
			var jsonRPCReq dinHttp.JSONRPCRequest
			if err := json.Unmarshal(bodyBytes, &jsonRPCReq); err == nil {
				parsedReqBody = &jsonRPCReq
			}
		}
	}

	// Use the actual max attempts from the network configuration
	maxAttempts := 1 // Default fallback
	if networkObj != nil {
		maxAttempts = networkObj.RequestAttemptCount
	}

	// Use logFailedAttempt for consistent logging format
	logFailedAttempt(&LogFailedAttemptParams{
		Reason:              "Context cancellation",
		Logger:              l,
		NetworkPath:         networkPath,
		FailedAttemptNumber: attempt + 1,   // Current attempt number (1-indexed)
		MaxAttempts:         maxAttempts,   // Use actual max attempts from network config
		StatusCodeOfFailure: statusCode,    // Status code we're about to return
		Error:               err,           // The context cancellation error
		Replacer:            repl,          // Caddy replacer for context data
		ParsedReqBody:       parsedReqBody, // Parsed request body
		RawResponseBody:     nil,           // No response body for context cancellation
	})

	// Set appropriate headers and write response
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(statusCode)
	rw.Write([]byte(responseBody))

	// Collect Prometheus metrics for context cancellation
	if promClient != nil {
		// Get provider from replacer for metrics
		var provider string
		if v, ok := repl.Get(RequestProviderKey); ok {
			if pStr, ok := v.(string); ok {
				provider = pStr
			}
		}
		if provider == "" {
			provider = "unknown"
		}

		// Get method name, fallback to "unknown" if parsedReqBody is nil
		methodName := "unknown"
		if parsedReqBody != nil {
			methodName = parsedReqBody.Method
		}

		// Record metrics for the context cancellation
		promClient.HandleRequestMetrics(&prom.PromRequestMetricData{
			Method:         methodName,
			Network:        networkPath,
			Provider:       provider,
			HostName:       r.Host,
			ResponseStatus: statusCode,
			HealthStatus:   "unhealthy", // Context cancellation indicates unhealthy state
			Environment:    "unknown",   // We don't have access to environment here
		}, duration, parsedReqBody)
	}
}
