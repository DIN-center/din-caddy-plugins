package modules

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	dinHttp "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	prom "github.com/DIN-center/din-caddy-plugins/lib/prometheus"
	"github.com/caddyserver/caddy/v2"
	"go.uber.org/zap"
)

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

// ensureUniqueProviderHost ensures the provider has a unique host in the network's providers map
// by appending a counter if necessary. Returns the unique host value.
func (d *DinMiddleware) ensureUniqueProviderHost(networkName string, host string) string {
	// Get existing hosts with the same base name
	baseHosts := []string{}

	for existingHost := range d.Networks[networkName].Providers {
		// We need to match exact host or host-N pattern
		if existingHost == host || strings.HasPrefix(existingHost, host+"-") {
			baseHosts = append(baseHosts, existingHost)
		}
	}

	// If no hosts with this base exist yet, use base host without suffix
	if len(baseHosts) == 0 {
		return host
	}

	// For subsequent hosts, use host-1, host-2, etc.
	return fmt.Sprintf("%s-%d", host, len(baseHosts))
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
		// Ensure NetworkObj is not nil before accessing HCMethod to prevent panics.
		if params.NetworkObj != nil && requestMethod == params.NetworkObj.HCMethod {
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

// processHCMethodResponseAsync asynchronously processes responses for health check method requests.
// It extracts the request method from the replacer, verifies if it matches the network's health check
// method (HCMethod), and if so, processes the block number from the response body.
// This function is designed to run in a separate goroutine to avoid blocking the main request handling flow.
// When a valid block number is extracted, it also retrieves the corresponding block hash and adds both
// to the network's block history. This information is crucial for tracking the network's current state
// and ensuring proper synchronization across providers.
func (d *DinMiddleware) processHCMethodResponseAsync(networkObj *network, networkPath string, respBody []byte, respStatus int, method string) {
	if len(respBody) == 0 || networkObj == nil || networkObj.HCMethod == "" {
		return
	}

	// If method is "other_method" and networkObj.HCMethod is "eth_blockNumber", this should be true.
	if method != networkObj.HCMethod {
		return
	}

	// This log should only appear if Condition 2 is false.
	d.logger.Debug("Goroutine: Processing response for HCMethod", zap.String("method", method), zap.String("network", networkPath))

	// Pass the address of respStatus to processBlockNumberResponse
	// processBlockNumberResponse checks for respStatus >= 400
	blockNumber, _, processingError := networkObj.processBlockNumberResponse(respBody, &respStatus)
	if processingError != nil {
		d.logger.Warn("Goroutine: HCMethod matched, error processing block number from response using processBlockNumberResponse", zap.Error(processingError), zap.String("network", networkPath), zap.String("response_body_snippet", string(respBody)))
		return
	}

	block, err := networkObj.getBlockByNumber(blockNumber)
	if err != nil {
		d.logger.Warn("Goroutine: HCMethod matched, error getting block hash for block number", zap.Error(err), zap.String("network", networkPath), zap.Int64("block_number", blockNumber))
		return
	}

	// save the block number to the network object's history
	networkObj.AddNetworkBlockEntry(blockNumber, block) // Add the block number and block hash to the network object's history as long as its the the latest block number
	d.logger.Debug("Goroutine: HCMethod matched, successfully processed block number and added to network history", zap.Int64("block_number", blockNumber), zap.String("network", networkPath))
}
