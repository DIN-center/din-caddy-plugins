package modules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	dinHttp "github.com/DIN-center/din-caddy-plugins/lib/http"
	networklib "github.com/DIN-center/din-caddy-plugins/lib/network"
	prom "github.com/DIN-center/din-caddy-plugins/lib/prometheus"

	stdliberrors "errors"
)

// ServeHTTP is the main handler for the middleware that is ran for every request.
// It checks if the network path is defined in the networks map and sets the provider in the context.
func (d *DinMiddleware) ServeHTTP(rw http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error { //nolint:gocyclo

	// Caddy replacer is used to set the context for the request
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)

	// Extract network path - for REST APIs, this is just the first segment
	fullPath := strings.TrimPrefix(r.URL.Path, "/")
	pathSegments := strings.Split(fullPath, "/")
	networkPath := pathSegments[0] // Get the first segment as network name

	// If the network path is empty, return an empty JSON object with a 200.
	if networkPath == "" {
		rw.WriteHeader(http.StatusOK)
		_, err := rw.Write([]byte("{}"))

		return err
	}

	// Safely access the networks map for guaranteed consistency
	// It assumes that the network object is immutable after the lock is released or
	// that any internal shared state in the network object is protected by a granular lock (at the shared state level)
	// IMPORTANT NOTE: network objects are modified by the DIN Registry (e.g. adding/removing providers, methods, etc.)
	// This means that DIN Registry cannot be enabled without a refactor of the middleware to protect the shared state at the granular level
	d.mu.RLock()
	networkObj, ok := d.Networks[networkPath]
	d.mu.RUnlock()
	if !ok {
		// If the network is not defined, return a 404.
		rw.WriteHeader(http.StatusNotFound)
		_, err := rw.Write([]byte("Not Found\n"))

		return fmt.Errorf("network undefined: %w", err)
	}

	// Ensure handler is available
	if networkObj.Handler == nil {
		d.logger.Error("No handler available for network", zap.String("network", networkPath))
		rw.WriteHeader(http.StatusInternalServerError)
		_, err := rw.Write([]byte("Internal Server Error\n"))

		return fmt.Errorf("no handler available for network %s: %w", networkPath, err)
	}

	// Store network object in replacer for later use
	repl.Set("network_object", networkObj)
	if api_key := r.Header.Get("Din-Api-Key"); api_key != "" {
		repl.Set("din_api_key", d.getAPIKeyId(api_key))
		r.Header.Del("Din-Api-Key") // We don't want to pass this information to providers
	} else {
		repl.Set("din_api_key", "unspecified")
	}

	// Strip Authorization header from client requests
	// The proxy handles all authentication internally
	// NOTE: This removes clients' ability to pass their own Authorization headers to providers.
	// While there is no current use case for this capability, if future requirements emerge
	// for client-controlled authentication (e.g., provider-specific bearer tokens),
	// this behavior will need to be re-evaluated.
	r.Header.Del("Authorization")

	// Middleware focuses on request validation only
	// DinSelect will handle all REST API path processing during provider configuration

	// Process the request using the handler for validation only
	if err := networkObj.Handler.ProcessRequest(r); err != nil {
		d.logger.Error("Handler failed to process request", zap.String("network", networkPath), zap.Error(err))

		var (
			statusCode int
			body       string
		)

		// Check if it's an HTTPError with specific status code
		httpErr := &networklib.HTTPError{}
		if errors.As(err, &httpErr) {
			statusCode = httpErr.StatusCode
			body = httpErr.Message + "\n"
		} else {
			statusCode = http.StatusBadRequest
			body = "Bad Request\n"
		}

		rw.WriteHeader(statusCode)
		_, writeErr := rw.Write([]byte(body))

		return stdliberrors.Join(fmt.Errorf("handler failed to process request: %w", err), writeErr)
	}

	// Read request body and save in context
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil
	}
	repl.Set(RequestBodyKey, bodyBytes)
	// Set request body back to original state
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Check if the request payload is too large
	if (len(bodyBytes) / 1024) > int(networkObj.MaxRequestPayloadSizeKB) {
		// If the request payload is too large, return an error
		rw.WriteHeader(http.StatusRequestEntityTooLarge)
		_, err := rw.Write([]byte("Request payload too large\n"))
		if err != nil {
			return errors.Wrap(err, "request payload too large")
		}

		return fmt.Errorf("request payload too large")
	}

	// Check to see if the request is empty (only for JSON-RPC requests)
	// REST APIs like beacon chain can have empty bodies for GET requests
	if len(bodyBytes) == 0 && networkObj.Handler.GetRequestType() == networklib.RequestTypeRPC {
		// if the request body is empty for JSON-RPC, do not increment the prometheus metric, return an error
		// this is specifically for OPTIONS requests and invalid JSON-RPC payload bodies
		rw.WriteHeader(http.StatusBadRequest)
		_, err := rw.Write([]byte("Request body is empty\n"))
		if err != nil {
			return fmt.Errorf("error writing response: %w", err)
		}
		return fmt.Errorf("request body is empty")
	}

	// Extract the method using the handler
	method, err := networkObj.Handler.ExtractMethod(r, bodyBytes)
	if err != nil {
		d.logger.Error("Failed to extract method", zap.String("network", networkPath), zap.Error(err))
		// Don't fail the request, just set method to unknown for metrics
		method = "unknown"
	}
	repl.Set(RequestMethodKey, method)

	// Create a new response writer wrapper to capture the response body and status code
	var rww *ResponseWriterWrapper

	if networkObj.MethodFilter != nil {
		// For method filtering, we need to parse the JSON-RPC request
		// Only do this if we have a body and it's a JSON-RPC request
		var parsedRequest *dinHttp.JSONRPCRequest
		if len(bodyBytes) > 0 && networkObj.Handler.GetRequestType() == networklib.RequestTypeRPC {
			var rpcReq dinHttp.JSONRPCRequest
			if err := json.Unmarshal(bodyBytes, &rpcReq); err == nil {
				parsedRequest = &rpcReq
			}
		}
		// Set the upstreams in the context for the request
		// Type assert MethodFilter to ProviderFilter interface
		if filter, ok := networkObj.MethodFilter.(ProviderFilter); ok {
			repl.Set(DinUpstreamsContextKey, filter.FilterProviders(parsedRequest, networkObj.Providers))
		} else {
			repl.Set(DinUpstreamsContextKey, networkObj.Providers)
		}
	} else {
		// Set the upstreams in the context for the request
		repl.Set(DinUpstreamsContextKey, networkObj.Providers)
	}

	// Set if dynamic load balancing should be done
	repl.Set(DinScoreBasedLoadBalancingContextKey, d.DynamicLoadBalancing.Enabled)

	reqStartTime := time.Now()

	// Track if we should log metrics at the end (only for final outcomes)
	var shouldLogMetrics bool

	// Retry the request if it fails up to the max attempt request count
	// Retries occur when:
	// 1. HTTP errors (non-200 status codes or upstream errors)
	// 2. Retryable JSON-RPC errors (server errors, timeouts, rate limits, etc.)
	// Non-retryable JSON-RPC errors (method not found, invalid params) will not trigger retries
	for attempt := 0; attempt < networkObj.RequestAttemptCount; attempt++ {
		if err := checkRequestContext(d.logger, r, networkPath, attempt); err != nil {
			handleContextCancellation(d.logger, d.PrometheusClient, rw, r, networkPath, attempt, err, reqStartTime, networkObj)
			return nil
		}
		rww = NewResponseWriterWrapper(rw)

		// If the request fails, reset the request body and custom header if its present to the original request state
		if attempt > 0 {
			var reqBody []byte
			if v, ok := repl.Get(RequestBodyKey); ok {
				reqBody = v.([]byte)
			}
			r.Body = io.NopCloser(bytes.NewReader(reqBody))

			// Remove the custom header if it was set
			if rww.Header().Get(DinProviderInfo) != "" {
				rww.Header().Del(DinProviderInfo)
			}
		}
		// Serve the request
		err = next.ServeHTTP(rww, r)

		// Check for success and handle response
		if err == nil {
			// Get response body
			var responseBody []byte
			if rww.body != nil {
				responseBody = rww.body.Bytes()
			}

			// Decompress gzip if necessary before passing to handler
			responseBody = decompressGzipBodyIfNecessary(rww.Header(), responseBody, d.logger, networkPath)

			// Check for application-level errors using the handler
			var appError error
			if rww.statusCode >= 200 && rww.statusCode < 300 {
				// For successful HTTP responses, check for application-level errors
				appError = networkObj.Handler.ParseResponse(responseBody, rww.statusCode)
			} else {
				// For non-2xx responses, create an HTTP error
				appError = fmt.Errorf("HTTP error: %d", rww.statusCode)
			}
			if appError == nil {
				// Request was successful
				shouldLogMetrics = true
				break
			}

			// Use the method we already extracted
			var params json.RawMessage

			// Check if the error is retryable using the handler
			if !networkObj.Handler.IsRetryableError(appError, rww.statusCode) {
				// Non-retryable error
				// Log this for debugging purposes since we won't retry
				logFailedAttempt(LogFailedAttemptParams{
					Reason:              "Non-retryable application error",
					Logger:              d.logger,
					NetworkPath:         networkPath,
					FailedAttemptNumber: attempt + 1,
					MaxAttempts:         networkObj.RequestAttemptCount,
					StatusCodeOfFailure: rww.statusCode,
					Error:               appError,
					Replacer:            repl,
					RequestMethod:       method,
					RequestParams:       params,
					RawResponseBody:     responseBody,
				})
				break
			}

			// Log retryable application error
			logFailedAttempt(LogFailedAttemptParams{
				Reason:              "Retryable application error",
				Logger:              d.logger,
				NetworkPath:         networkPath,
				FailedAttemptNumber: attempt + 1,
				MaxAttempts:         networkObj.RequestAttemptCount,
				StatusCodeOfFailure: rww.statusCode,
				Error:               appError,
				Replacer:            repl,
				RequestMethod:       method,
				RequestParams:       params,
				RawResponseBody:     responseBody,
			})

			continue
		}

		// Check for HTTP/low-level errors (non-200 status codes, network issues, etc.)
		if err != nil {
			// Use the method we already extracted
			var params json.RawMessage

			// Log HTTP/low-level error
			var responseBody []byte
			if rww.body != nil {
				responseBody = rww.body.Bytes()
			}

			// Determine if this is the final attempt or not
			reason := "HTTP/network error"
			if attempt == networkObj.RequestAttemptCount-1 {
				reason = "HTTP/network error (final attempt)"
				shouldLogMetrics = true
			}

			logFailedAttempt(LogFailedAttemptParams{
				Reason:              reason,
				Logger:              d.logger,
				NetworkPath:         networkPath,
				FailedAttemptNumber: attempt + 1,
				MaxAttempts:         networkObj.RequestAttemptCount,
				StatusCodeOfFailure: rww.statusCode,
				Error:               err,
				Replacer:            repl,
				RequestMethod:       method,
				RequestParams:       params,
				RawResponseBody:     responseBody,
			})
		}
	}

	// Handle final error case (all retries exhausted)
	if err != nil {
		// Get provider for metrics
		var provider string
		if v, ok := repl.Get(RequestProviderKey); ok {
			provider = v.(string)
		}

		// Get priority for metrics
		priority := 0
		if v, ok := repl.Get(RequestProviderPriorityKey); ok {
			if pInt, ok := v.(int); ok {
				priority = pInt
			}
		}

		// Get provider name for metrics
		providerName := "unknown"
		if v, ok := networkObj.Providers[provider]; ok {
			providerName = v.Name
		}

		duration := time.Since(reqStartTime)

		// Determine appropriate status code for the failure
		statusCode := http.StatusInternalServerError // Default for HTTP errors
		if rww != nil && rww.statusCode > 0 {
			statusCode = rww.statusCode
		}

		// Log metrics for final failure (only if we haven't already marked it for logging)
		if !shouldLogMetrics && !d.testMode && d.PrometheusClient != nil {
			// Record metrics for the failed request
			d.PrometheusClient.HandleRequestMetrics(&prom.PromRequestMetricData{
				Method:         method,
				Network:        networkPath,
				Provider:       provider,
				ProviderName:   providerName,
				ApiKey:         getRequestAPIKey(repl),
				HostName:       r.Host,
				ResponseStatus: statusCode,
				HealthStatus:   "unhealthy", // All providers failed
				Priority:       priority,
				Environment:    string(d.Env),
			}, duration, nil)
		}

		return errors.Wrap(err, "Error serving HTTP")
	}

	var provider string
	if v, ok := repl.Get(RequestProviderKey); ok {
		provider = v.(string)
	}

	duration := time.Since(reqStartTime)
	// Write the response body and status to the original response writer
	// This is done after the request is attempted multiple times if needed
	if rww != nil {

		// Copy headers from wrapper to original response writer
		for k, v := range rww.Header() {
			rw.Header()[k] = v
		}

		rww.ResponseWriter.WriteHeader(rww.statusCode)

		// Write the body
		_, err := rw.Write(rww.body.Bytes())
		if err != nil {
			d.logger.Error("Error writing response body", zap.String("network", networkPath), zap.Error(err))
			return errors.Wrap(err, "Error writing response body")
		}

		// Flush the response to ensure all data is sent to the client
		if flusher, ok := rw.(http.Flusher); ok {
			flusher.Flush()
		}
	}

	// Only log metrics if this is a final outcome (success or final failure)
	if shouldLogMetrics {
		// Post-Request Processing is now handled by the helper function
		handlePostRequestTasks(PostRequestTaskParams{
			DinMiddleware: d,
			RWWrapper:     rww,
			NetworkObj:    networkObj,
			NetworkPath:   networkPath,
			Provider:      provider,
			Replacer:      repl,
			Duration:      duration,
			OriginalReq:   r,
			ParsedReqBody: nil,
		})
	}

	return nil
}
