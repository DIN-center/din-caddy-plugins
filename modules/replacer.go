package modules

import (
	"encoding/json"
	"fmt"

	dinHttp "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/caddyserver/caddy/v2"
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
