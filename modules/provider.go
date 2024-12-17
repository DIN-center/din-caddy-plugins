package modules

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	"github.com/DIN-center/din-caddy-plugins/lib/auth/siwe"
	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type provider struct {
	HttpUrl             string
	path                string
	host                string
	Headers             map[string]string
	upstream            *reverseproxy.Upstream
	httpClient          din_http.IHTTPClient
	logger              *zap.Logger
	failures            int
	successes           int
	healthStatus        HealthStatus // 0 = Healthy, 1 = Warning, 2 = Unhealthy
	latestBlockNumber   uint64
	earliestBlockNumber uint64
	Priority            int
	quit                chan struct{}

	// Registry Configuration Values
	Methods []*string            `json:"methods"`
	Auth    *siwe.SIWEClientAuth `json:"auth"`

	consecutiveHealthyChecks int
}

func NewProvider(urlStr string) (*provider, error) {
	url, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	p := &provider{
		HttpUrl:                  urlStr,
		host:                     url.Host,
		Headers:                  make(map[string]string),
		consecutiveHealthyChecks: 0,
	}
	return p, nil
}

// Available indicates whether the Caddy upstream is available, and
// whether the provider's healthchecks indicate the upstream is healthy.
func (p *provider) Available() bool {
	return p.upstream.Available() && p.Healthy()
}

func (p *provider) IsAvailableWithWarning() bool {
	return p.upstream.Available() && p.Warning()
}

func (p *provider) AuthClient() auth.IAuthClient {
	if p.Auth == nil {
		return nil
	}
	return p.Auth
}

// markPingFailure records the failure, and if the failure count exceeds the healthcheck threshold
// marks the upstream as unhealthy
func (p *provider) markPingFailure(hcThreshold int) {
	p.failures++
	p.successes = 0
	if p.healthStatus == Healthy && p.failures > hcThreshold {
		p.healthStatus = Unhealthy
	}
}

func (p *provider) markPingWarning() {
	p.successes = 0
	p.failures = 0
	p.healthStatus = Warning
}

// markPingSuccess records a successful healthcheck, and if the success count exceeds the healthcheck
// threshold marks the upstream as healthy
func (p *provider) markPingSuccess(hcThreshold int) {
	p.successes++
	if p.healthStatus == Unhealthy && p.successes > hcThreshold {
		p.failures = 0
		p.healthStatus = Healthy
	}
}

func (p *provider) markHealthy(hcThreshold int) {
	if p.healthStatus == Unhealthy {
		p.consecutiveHealthyChecks++
		if p.consecutiveHealthyChecks > hcThreshold {
			p.healthStatus = Healthy
			p.consecutiveHealthyChecks = 0
		}
		return
	}
	p.consecutiveHealthyChecks = 0
	p.healthStatus = Healthy
}

func (p *provider) markWarning() {
	p.healthStatus = Warning
	p.consecutiveHealthyChecks = 0
}

func (p *provider) markUnhealthy() {
	p.healthStatus = Unhealthy
	p.consecutiveHealthyChecks = 0
}

// Healthy returns True if the node is passing healthchecks, False otherwise
func (p *provider) Healthy() bool {
	if p.healthStatus == Healthy {
		return true
	} else {
		return false
	}
}

// Warning returns True if the node is returning warning in healthchecks, False otherwise
func (p *provider) Warning() bool {
	if p.healthStatus == Warning {
		return true
	} else {
		return false
	}
}

// intToHex converts an int64 to a "0x" prefixed hex string
func intToHex(n int64) string {
	return fmt.Sprintf("0x%x", n)
}

func hexToInt(hex string) (int64, error) {
	// Check for empty string
	if len(hex) == 0 {
		return 0, fmt.Errorf("empty hex string")
	}

	// Check for minimum length (0x + at least one digit)
	if len(hex) < 3 {
		return 0, fmt.Errorf("invalid hex format: too short")
	}

	// Verify 0x prefix
	if hex[:2] != "0x" {
		return 0, fmt.Errorf("invalid hex format: missing 0x prefix")
	}

	hexNum := hex[2:]
	return strconv.ParseInt(hexNum, 16, 64)
}

func parseBlockNumber(result interface{}) (int64, error) {
	switch result := result.(type) {
	case string:
		if result == "" || result[:2] != "0x" {
			return 0, errors.New("Invalid block number")
		}
		// Convert the hexadecimal string to an int64
		blockNumber, err := hexToInt(result)
		if err != nil {
			return 0, errors.Wrap(err, "Error converting block number")
		}
		return blockNumber, nil
	case float64:
		return int64(result), nil
	default:
		return 0, errors.New("unsupported block number type")
	}
}

func (p *provider) getLatestBlockNumber(hcMethod string) (int64, int, error) {
	payload := []byte(fmt.Sprintf(`{"jsonrpc":"2.0","method": "%s","params":[],"id":1}`, hcMethod))

	// Send the POST request
	resBytes, statusCode, err := p.httpClient.Post(p.HttpUrl, p.Headers, []byte(payload), p.AuthClient())
	if err != nil {
		return 0, 0, errors.Wrap(err, "Error sending POST request")
	}

	if *statusCode == http.StatusServiceUnavailable || *statusCode == StatusOriginUnreachable {
		return 0, *statusCode, errors.New("Network Unavailable")
	}

	// response struct
	var respObject map[string]interface{}

	// Unmarshal the response
	err = json.Unmarshal(resBytes, &respObject)
	if err != nil {
		return 0, 0, errors.Wrap(err, "Error unmarshalling response")
	}

	if _, ok := respObject["result"]; !ok {
		return 0, 0, errors.New("Error getting block number from response")
	}

	blockNumber, err := parseBlockNumber(respObject["result"])
	if err != nil {
		return 0, 0, err
	}
	return blockNumber, *statusCode, nil
}

// getEarliestBlockNumber gets the earliest block number from the provider
// First checks block 1, then uses binary search if not found
// This is only enabled for EVM based networks
func (p *provider) getEarliestBlockNumber(getBlockNumberMethod string, retryCount int) (int64, int, error) {
	// First, try block 1
	payload := []byte(fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","params":["%s", false],"id":1}`, getBlockNumberMethod, "0x1"))
	resBytes, statusCode, err := p.httpClient.Post(p.HttpUrl, p.Headers, payload, p.AuthClient())
	if err != nil {
		return 0, 0, errors.Wrap(err, "Error sending POST request")
	}

	// Check node availability
	if *statusCode == http.StatusServiceUnavailable || *statusCode == StatusOriginUnreachable {
		return 0, *statusCode, errors.New("Network Unavailable")
	}

	var respObject map[string]interface{}
	err = json.Unmarshal(resBytes, &respObject)
	if err != nil {
		return 0, 0, errors.Wrap(err, "Error unmarshalling response")
	}

	// If block 1 exists, parse and return its number
	if result, ok := respObject["result"]; ok && result != nil {
		blockInfo, ok := result.(map[string]interface{})
		if !ok {
			return 0, 0, errors.New("invalid block info format")
		}

		if blockInfo["number"] == nil {
			return 0, 0, errors.New("block number is nil")
		}

		blockNumber, err := parseBlockNumber(blockInfo["number"])
		if err != nil {
			return 0, 0, err
		}
		return blockNumber, *statusCode, nil
	}

	// If block 1 doesn't exist, use binary search to find earliest block
	return p.binarySearchEarliestBlock(getBlockNumberMethod)
}

func (p *provider) saveLatestBlockNumber(blockNumber int64) error {
	p.latestBlockNumber = uint64(blockNumber)
	return nil
}

func (p *provider) saveEarliestBlockNumber(blockNumber int64) error {
	p.earliestBlockNumber = uint64(blockNumber)
	return nil
}

// binarySearchEarliestBlock performs a binary search to find the earliest available block
func (p *provider) binarySearchEarliestBlock(getBlockNumberMethod string) (int64, int, error) {
	// We need the latest block number to set the upper bound
	if p.latestBlockNumber == 0 {
		return 0, 0, errors.New("latest block number not set")
	}

	left := int64(1)
	right := int64(p.latestBlockNumber)
	var lastFoundBlock int64
	var lastStatusCode int

	for left <= right {
		mid := left + (right-left)/2

		// Create the JSON-RPC payload for getting the block
		payload := []byte(fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","params":["%s", false],"id":1}`,
			getBlockNumberMethod, intToHex(mid)))

		// Send the request
		resBytes, statusCode, err := p.httpClient.Post(p.HttpUrl, p.Headers, payload, p.AuthClient())
		if err != nil {
			return 0, 0, errors.Wrap(err, "Error sending POST request")
		}

		// Check node availability
		if *statusCode == http.StatusServiceUnavailable || *statusCode == StatusOriginUnreachable {
			return 0, *statusCode, errors.New("Network Unavailable")
		}

		// Parse the response
		var respObject map[string]interface{}
		err = json.Unmarshal(resBytes, &respObject)
		if err != nil {
			return 0, 0, errors.Wrap(err, "Error unmarshalling response")
		}

		// Check if the block exists
		result, exists := respObject["result"]
		if exists && result != nil {
			// Block exists, try to find an earlier one
			blockInfo, ok := result.(map[string]interface{})
			if !ok {
				return 0, 0, errors.New("invalid block info format")
			}

			if blockInfo["number"] == nil {
				return 0, 0, errors.New("block number is nil")
			}

			blockNumber, err := parseBlockNumber(blockInfo["number"])
			if err != nil {
				return 0, 0, err
			}

			lastFoundBlock = blockNumber
			lastStatusCode = *statusCode
			right = mid - 1
		} else {
			// Block doesn't exist, try a later one
			left = mid + 1
		}
	}

	if lastFoundBlock == 0 {
		return 0, 0, errors.New("no valid blocks found")
	}

	return lastFoundBlock, lastStatusCode, nil
}
