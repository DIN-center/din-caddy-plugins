package modules

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

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
// If the blocknumber at 0 isn't available, then binary search is used to find the earliest block number
// This is only enabled for EVM based networks
func (p *provider) getEarliestBlockNumber(getBlockNumberMethod string, retryCount int) (int64, int, error) {
	// First attempt to get block 0 since it's commonly the earliest block
	payload := []byte(fmt.Sprintf(`{"jsonrpc":"2.0","method": "%s","params":["0x0", false],"id":1}`, getBlockNumberMethod))

	// Make HTTP request to the node
	resBytes, statusCode, err := p.httpClient.Post(p.HttpUrl, p.Headers, []byte(payload), p.AuthClient())
	if err != nil {
		return 0, 0, errors.Wrap(err, "Error sending POST request")
	}

	// Check if node is available
	if *statusCode == http.StatusServiceUnavailable || *statusCode == StatusOriginUnreachable {
		return 0, *statusCode, errors.New("Network Unavailable")
	}

	var respObject map[string]interface{}

	// Parse JSON response
	err = json.Unmarshal(resBytes, &respObject)
	if err != nil {
		return 0, 0, errors.Wrap(err, "Error unmarshalling response")
	}

	// If block 0 exists, parse and return its number
	if result, ok := respObject["result"]; ok && result != nil {
		// Result should be a map containing block details
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

	// If block 0 doesn't exist, use binary search to find earliest block
	return p.binarySearchEarliestBlock(getBlockNumberMethod)
}

// binarySearchEarliestBlock performs a binary search to find the earliest available block
func (p *provider) binarySearchEarliestBlock(getBlockNumberMethod string) (int64, int, error) {
	if p.latestBlockNumber == 0 {
		return 0, 0, errors.New("latest block number not set")
	}

	left := int64(1)
	right := int64(p.latestBlockNumber)
	var earliestBlock int64

	maxIterations := 100
	iterations := 0

	for left <= right {
		iterations++
		if iterations > maxIterations {
			return 0, 0, errors.New("binary search exceeded maximum iterations")
		}

		mid := left + (right-left)/2
		hexBlock := intToHex(mid)

		// Query the midpoint block
		payload := []byte(fmt.Sprintf(`{"jsonrpc":"2.0","method": "%s","params":["%s", false],"id":1}`, getBlockNumberMethod, hexBlock))
		resBytes, statusCode, err := p.httpClient.Post(p.HttpUrl, p.Headers, []byte(payload), p.AuthClient())
		if err != nil {
			return 0, 0, errors.Wrap(err, "Error sending POST request during binary search")
		}

		// Check node availability
		if *statusCode == http.StatusServiceUnavailable || *statusCode == StatusOriginUnreachable {
			return 0, *statusCode, errors.New("Network Unavailable during binary search")
		}

		var respObject map[string]interface{}
		err = json.Unmarshal(resBytes, &respObject)
		if err != nil {
			return 0, 0, errors.Wrap(err, "Error unmarshalling response during binary search")
		}

		// Check for RPC error
		if errObj, hasError := respObject["error"]; hasError {
			return 0, 0, errors.Errorf("RPC error during binary search: %v", errObj)
		}

		// If block exists, look for earlier blocks
		// If block doesn't exist, look for later blocks
		if result, ok := respObject["result"]; ok && result != nil {
			earliestBlock = mid
			right = mid - 1
		} else {
			left = mid + 1
		}

		// Rate limiting protection
		time.Sleep(time.Millisecond * 100)
	}

	// Return error if no valid block was found
	if earliestBlock == 0 {
		return 0, 0, errors.New("Could not find earliest block")
	}

	return earliestBlock, http.StatusOK, nil
}

func (p *provider) saveLatestBlockNumber(blockNumber int64) error {
	p.latestBlockNumber = uint64(blockNumber)
	return nil
}

func (p *provider) saveEarliestBlockNumber(blockNumber int64) error {
	p.earliestBlockNumber = uint64(blockNumber)
	return nil
}
