package modules

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	dinHttp "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
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
