package modules

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	dinHttp "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	networklib "github.com/DIN-center/din-caddy-plugins/lib/network"
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
	Error               error           // Can be nil
	Replacer            *caddy.Replacer // Caddy replacer to get context data
	RequestMethod       string          // The request method/endpoint (extracted by caller)
	RequestParams       json.RawMessage // The request parameters (extracted by caller)
	RawResponseBody     []byte          // Raw response body to parse internally
}

// Helper function to log a failed request attempt that will be retried.
// This function is intended to be run as a goroutine to avoid blocking the main request flow.
// It logs the provided information in a structured format for debugging and monitoring purposes.
// This function is completely generic and network-agnostic - all network-specific logic
// should be handled by the caller before passing data to this function.
func logFailedAttempt(params LogFailedAttemptParams) {
	// Extract provider information from the Caddy replacer context
	provider := "unknown"
	if provVal, provOk := params.Replacer.Get(RequestProviderKey); provOk {
		if pStr, strOk := provVal.(string); strOk {
			provider = pStr
		}
	}

	// Use the provided method and params directly (already extracted by caller)
	requestMethod := params.RequestMethod
	if requestMethod == "" {
		requestMethod = "unknown"
	}

	// Extract raw request body for logging purposes
	rawRequestBodySnippet := ""
	if v, okGet := params.Replacer.Get(RequestBodyKey); okGet {
		if bodyBytes, okCast := v.([]byte); okCast && len(bodyBytes) > 0 {
			// Limit the snippet length to 500 characters
			length := 500
			if len(bodyBytes) < length {
				length = len(bodyBytes)
			}
			rawRequestBodySnippet = string(bodyBytes[:length])
		}
	}

	// Build the base log fields
	logFields := []zap.Field{
		zap.String("network", params.NetworkPath),
		zap.String("provider", provider),
		zap.Int("failed_attempt_number", params.FailedAttemptNumber),
		zap.Int("max_attempts", params.MaxAttempts),
		zap.Int("status_code", params.StatusCodeOfFailure),
		zap.String("reason", params.Reason),
		zap.String("request_method", requestMethod),
	}

	// Add error information if provided
	if params.Error != nil {
		logFields = append(logFields, zap.NamedError("error", params.Error))
	}

	// Add request parameters if provided (already extracted by caller)
	if len(params.RequestParams) > 0 && string(params.RequestParams) != "null" {
		var decodedParams interface{}
		if err := json.Unmarshal(params.RequestParams, &decodedParams); err == nil {
			logFields = append(logFields, zap.Any("request_params", decodedParams))
		} else {
			logFields = append(logFields, zap.String("raw_request_params", string(params.RequestParams)))
		}
	}

	// Add raw request body snippet if available
	if rawRequestBodySnippet != "" {
		var requestBodyJSON interface{}
		if err := json.Unmarshal([]byte(rawRequestBodySnippet), &requestBodyJSON); err == nil {
			logFields = append(logFields, zap.Any("raw_request_body", requestBodyJSON))
		} else {
			logFields = append(logFields, zap.String("raw_request_body", rawRequestBodySnippet))
		}
	}

	// Add raw response body snippet if provided
	if len(params.RawResponseBody) > 0 {
		// Handle gzip-compressed response bodies
		processedResponseBody := params.RawResponseBody
		if len(params.RawResponseBody) >= 2 && params.RawResponseBody[0] == 0x1f && params.RawResponseBody[1] == 0x8b {
			headers := make(http.Header)
			headers.Set("Content-Encoding", "gzip")
			processedResponseBody = decompressGzipBodyIfNecessary(headers, params.RawResponseBody, params.Logger, params.NetworkPath)
		}

		// Create a snippet of the response for logging
		length := 500
		if len(processedResponseBody) < length {
			length = len(processedResponseBody)
		}
		rawResponseSnippet := string(processedResponseBody[:length])

		var responseJSON interface{}
		if err := json.Unmarshal(processedResponseBody, &responseJSON); err == nil {
			logFields = append(logFields, zap.Any("raw_response_body", responseJSON))
		} else {
			logFields = append(logFields, zap.String("raw_response_body", rawResponseSnippet))
		}
	}

	// Log the failed attempt
	params.Logger.Error("Request attempt failed", logFields...)
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
		// Pass the request method to let processHCMethodResponseAsync handle method matching
		if params.NetworkObj != nil && params.NetworkObj.handler != nil {
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
			// Debug: log all providers in the map
			var availableProviders []string
			for host := range params.NetworkObj.Providers {
				availableProviders = append(availableProviders, host)
			}
			params.DinMiddleware.logger.Warn("Provider from replacer not found in network's providers map for metrics.", 
				zap.String("provider", params.Provider), 
				zap.String("network", params.NetworkPath),
				zap.Strings("availableProviders", availableProviders))
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

// GenericRequestContext holds network-agnostic request context information
type GenericRequestContext struct {
	Method      string
	HTTPMethod  string
	RequestType networklib.RequestType
	Context     map[string]interface{} // Network-specific context (e.g., RPC params, REST path, etc.)
}

// createHealthCheckRequestContext creates synthetic request context for health checks
// so we can reuse logFailedAttempt for consistent logging
func createHealthCheckRequestContext(networkName, providerHost, method string, networkObj *network) (*caddy.Replacer, *GenericRequestContext, []byte) {
	// Create a synthetic replacer with health check context
	repl := caddy.NewReplacer()
	repl.Set(RequestProviderKey, providerHost)

	// Require handler to create the payload - no fallbacks
	if networkObj == nil || networkObj.handler == nil {
		// Return empty values if no handler available - let caller handle the error
		return repl, nil, nil
	}

	// Let the handler create the network-specific health check payload
	payload, err := networkObj.handler.CreateHealthCheckPayload(method)
	if err != nil {
		// Return empty values if handler fails - let caller handle the error
		return repl, nil, nil
	}

	repl.Set(RequestBodyKey, payload)

	// Create generic request context - let handler determine all details
	requestType := networkObj.handler.GetRequestType()
	httpMethod := networkObj.handler.GetHealthCheckHTTPMethod()

	// Handler is responsible for creating the appropriate context
	requestContext := map[string]interface{}{
		"method": method,
	}

	genericContext := &GenericRequestContext{
		Method:      method,
		HTTPMethod:  httpMethod,
		RequestType: requestType,
		Context:     requestContext,
	}

	return repl, genericContext, payload
}

// createGetBlockByNumberRequestContext creates synthetic request context for getBlockByNumber calls
// so we can reuse logFailedAttempt for consistent logging
func createGetBlockByNumberRequestContext(networkName, providerHost, method string, blockNumber int64, networkObj *network) (*caddy.Replacer, *GenericRequestContext, []byte) {
	// Create a synthetic replacer with getBlockByNumber context
	repl := caddy.NewReplacer()
	repl.Set(RequestProviderKey, providerHost)

	// Require handler to create the payload - no fallbacks
	if networkObj == nil || networkObj.handler == nil {
		// Return empty values if no handler available - let caller handle the error
		return repl, nil, nil
	}

	// Let the handler create the network-specific payload
	payload, err := networkObj.handler.CreateBlockRequest(method, blockNumber, false)
	if err != nil {
		// Return empty values if handler fails - let caller handle the error
		return repl, nil, nil
	}

	repl.Set(RequestBodyKey, payload)

	// Create generic request context - let handler determine all details
	requestType := networkObj.handler.GetRequestType()
	httpMethod := "POST" // Most block requests are POST, but this could be made configurable

	// Handler is responsible for creating the appropriate context
	requestContext := map[string]interface{}{
		"method":      method,
		"blockNumber": blockNumber,
	}

	genericContext := &GenericRequestContext{
		Method:      method,
		HTTPMethod:  httpMethod,
		RequestType: requestType,
		Context:     requestContext,
	}

	return repl, genericContext, payload
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

	// Use the actual max attempts from the network configuration
	maxAttempts := 1 // Default fallback
	if networkObj != nil {
		maxAttempts = networkObj.RequestAttemptCount
	}

	// Use logFailedAttempt for consistent logging format
	logFailedAttempt(LogFailedAttemptParams{
		Reason:              "Context cancellation",
		Logger:              l,
		NetworkPath:         networkPath,
		FailedAttemptNumber: attempt + 1, // Current attempt number (1-indexed)
		MaxAttempts:         maxAttempts, // Use actual max attempts from network config
		StatusCodeOfFailure: statusCode,  // Status code we're about to return
		Error:               err,         // The context cancellation error
		Replacer:            repl,        // Caddy replacer for context data
		RequestMethod:       "unknown",   // Generic - method extraction should be done by caller if needed
		RequestParams:       nil,         // No specific params for context cancellation
		RawResponseBody:     nil,         // No response body for context cancellation
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

		// Record metrics for the context cancellation
		promClient.HandleRequestMetrics(&prom.PromRequestMetricData{
			Method:         "unknown", // Generic - no network-specific parsing
			Network:        networkPath,
			Provider:       provider,
			HostName:       r.Host,
			ResponseStatus: statusCode,
			HealthStatus:   "unhealthy", // Context cancellation indicates unhealthy state
			Environment:    "unknown",   // We don't have access to environment here
		}, duration, nil) // No parsed request body - completely generic
	}
}
