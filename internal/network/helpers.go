package network

import (
	"encoding/json"

	"github.com/caddyserver/caddy/v2"
	"go.uber.org/zap"

	networklib "github.com/DIN-center/din-caddy-plugins/lib/network"

	"github.com/DIN-center/din-caddy-plugins/lib/logger"
)

// Context key constants
const (
	DinUpstreamsContextKey     = "din.internal.upstreams"
	RequestProviderKey         = "request_provider"
	RequestProviderPriorityKey = "request_provider_priority"
	RequestBodyKey             = "request_body"
	RequestMethodKey           = "request_method"
	HealthStatusKey            = "health_status"
	BlockNumberKey             = "block_number"
)

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

// GenericRequestContext holds network-agnostic request context information
type GenericRequestContext struct {
	Method      string
	HTTPMethod  string
	RequestType networklib.RequestType
	Context     map[string]interface{} // Network-specific context (e.g., RPC params, REST path, etc.)
}

// LogFailedAttempt logs a failed request attempt that will be retried.
// This function is intended to be run as a goroutine to avoid blocking the main request flow.
// It logs the provided information in a structured format for debugging and monitoring purposes.
// This function is completely generic and network-agnostic - all network-specific logic
// should be handled by the caller before passing data to this function.
func LogFailedAttempt(params LogFailedAttemptParams) {
	// Extract provider information from the Caddy replacer context
	providerHost := "unknown"
	providerName := "unknown"
	if provVal, provOk := params.Replacer.Get(RequestProviderKey); provOk {
		if pStr, strOk := provVal.(string); strOk {
			providerHost = pStr

			// Now get the provider name from the providers map
			if providerMapVal, exists := params.Replacer.Get(DinUpstreamsContextKey); exists {
				if providers, castOk := providerMapVal.(map[string]*Provider); castOk {
					if providerObj, exists := providers[providerHost]; exists {
						providerName = providerObj.Name
					}
				}
			}
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
		zap.String("provider", providerHost),
		zap.String("provider_name", providerName),
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
		// Create a snippet of the response for logging
		length := 500
		if len(params.RawResponseBody) < length {
			length = len(params.RawResponseBody)
		}
		rawResponseSnippet := string(params.RawResponseBody[:length])

		var responseJSON interface{}
		if err := json.Unmarshal(params.RawResponseBody, &responseJSON); err == nil {
			logFields = append(logFields, zap.Any("raw_response_body", responseJSON))
		} else {
			logFields = append(logFields, zap.String("raw_response_body", rawResponseSnippet))
		}
	}

	// Log the failed attempt
	params.Logger.Error("Request attempt failed", logFields...)
}

// CreateHealthCheckRequestContext creates synthetic request context for health checks
// so we can reuse LogFailedAttempt for consistent logging
func CreateHealthCheckRequestContext(networkName, providerHost, method string, networkObj *Network) (*caddy.Replacer, *GenericRequestContext, []byte) {
	// Create a synthetic replacer with health check context
	repl := caddy.NewReplacer()
	repl.Set(RequestProviderKey, providerHost)

	// Require handler to create the payload - no fallbacks
	if networkObj == nil || networkObj.Handler == nil {
		// Return empty values if no handler available - let caller handle the error
		return repl, nil, nil
	}

	// Let the handler create the network-specific health check payload
	payload, err := networkObj.Handler.CreateHealthCheckPayload(method)
	if err != nil {
		// Return empty values if handler fails - let caller handle the error
		return repl, nil, nil
	}

	repl.Set(RequestBodyKey, payload)

	// Create generic request context - let handler determine all details
	requestType := networkObj.Handler.GetRequestType()
	httpMethod := networkObj.Handler.GetHealthCheckHTTPMethod()

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

// CreateGetBlockByNumberRequestContext creates synthetic request context for getBlockByNumber calls
// so we can reuse LogFailedAttempt for consistent logging
func CreateGetBlockByNumberRequestContext(networkName, providerHost, method string, blockNumber int64, networkObj *Network) (*caddy.Replacer, *GenericRequestContext, []byte) {
	// Create a synthetic replacer with getBlockByNumber context
	repl := caddy.NewReplacer()
	repl.Set(RequestProviderKey, providerHost)

	// Require handler to create the payload - no fallbacks
	if networkObj == nil || networkObj.Handler == nil {
		// Return empty values if no handler available - let caller handle the error
		return repl, nil, nil
	}

	// Let the handler create the network-specific payload
	payload, err := networkObj.Handler.CreateBlockRequest(method, blockNumber, false)
	if err != nil {
		// Return empty values if handler fails - let caller handle the error
		return repl, nil, nil
	}

	repl.Set(RequestBodyKey, payload)

	// Create generic request context - let handler determine all details
	requestType := networkObj.Handler.GetRequestType()
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
