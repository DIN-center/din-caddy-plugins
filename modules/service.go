package modules

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/openrelayxyz/din-caddy-plugins/lib/http"
	"github.com/pkg/errors"
)

type service struct {
	Name              string               `json:"name"`
	Providers         map[string]*provider `json:"providers"`
	Methods           []*string            `json:"methods"`
	quit              chan struct{}
	LatestBlockNumber int64 `json:"latest_block_number"`
	HTTPClient        http.IHTTPClient

	// Healthcheck configuration
	CheckedProviders map[string][]healthCheckEntry `json:"checked_providers"`
	HCMethod         string                        `json:"healthcheck_method"`
	HCInterval       int                           `json:"healthceck_interval_seconds"`
	HCThreshold      int                           `json:"healthcheck_threshold"`
	BlockLagLimit    int64                         `json:"healthcheck_blocklag_limit"`
}

func (s *service) startHealthcheck() {
	s.healthCheck()
	ticker := time.NewTicker(time.Second * time.Duration(s.HCInterval))
	go func() {
		// Keep an index for RPC request IDs
		for i := 0; ; i++ {
			select {
			// Cleanup if the quit channel gets closed. Right now nothing closes this channel, but
			// once we integrate the authentication work there's code that should.
			case <-s.quit:
				ticker.Stop()
				return
			case <-ticker.C:
				// Set up the healthcheck request with authentication for this provider.
				s.healthCheck()
			}
		}
	}()
}

type healthCheckEntry struct {
	blockNumber int64
	timestamp   *time.Time
}

func (s *service) healthCheck() {
	var blockTime time.Time
	// TODO: check all of the providers simultaneously using async job management for more accurate blocknumber results.
	for _, provider := range s.Providers {
		// get the latest block number from the current provider
		providerBlockNumber, statusCode, err := s.getLatestBlockNumber(provider.HttpUrl, provider.Headers)
		if err != nil {
			fmt.Println(err, "Error getting latest block number for provider", provider.host, "on service", s.Name)
			provider.markPingFailure(s.HCThreshold)
			continue
		}
		blockTime = time.Now()

		// Ping Health Check
		if statusCode > 399 {
			if statusCode == 429 {
				// if the status code is 429, mark the provider as a warning
				provider.markPingWarning()
			} else {
				// if the status code is greater than 399, mark the provider as a failure
				provider.markPingFailure(s.HCThreshold)
			}
			continue
		} else {
			provider.markPingSuccess(s.HCThreshold)
		}

		// Consistency health check
		if s.LatestBlockNumber == 0 || s.LatestBlockNumber < providerBlockNumber {
			// if the current provider's latest block number is greater than the service's latest block number, update the service's latest block number,
			// set the current provider as healthy and loop through all of the previously checked providers and set them as unhealthy
			s.LatestBlockNumber = providerBlockNumber

			provider.markHealthy()

			s.evaluateCheckedProviders()
		} else if s.LatestBlockNumber == providerBlockNumber {
			// if the current provider's latest block number is equal to the service's latest block number, set the current provider to healthy
			provider.markHealthy()
		} else if providerBlockNumber+s.BlockLagLimit < s.LatestBlockNumber {
			// if the current provider's latest block number is below the service's latest block number by more than the acceptable threshold, set the current provider to warning
			provider.markWarning()
		}

		// TODO: create a check based on time window of a provider's latest block number

		// add the current provider to the checked providers map
		s.addHealthCheckToCheckedProviderList(provider.upstream.Dial, healthCheckEntry{blockNumber: providerBlockNumber, timestamp: &blockTime})
	}
}

// addHealthCheckToCheckedProviderList adds a new healthCheckEntry to the beginning of the CheckedProviders healthCheck list for the given provider
// the list will not exceed 10 entries
func (s *service) addHealthCheckToCheckedProviderList(providerName string, healthCheckInput healthCheckEntry) {
	// if the provider is not in the checked providers map, add it with its initial block number and timestamp
	if _, ok := s.CheckedProviders[providerName]; !ok {
		s.CheckedProviders[providerName] = []healthCheckEntry{healthCheckInput}
		return
	}

	// to add a new healthCheckEntry to index 0 of the provider's slice, we need to make a new slice and copy the old slice to the new slice
	newHealthCheckList := []healthCheckEntry{healthCheckInput}

	// if the old slice is full at 10 entries, we need to remove the last entry and copy the rest of the entries to the new slice
	if len(s.CheckedProviders[providerName]) == 10 {
		s.CheckedProviders[providerName] = append(newHealthCheckList, s.CheckedProviders[providerName][:9]...)
		copy(newHealthCheckList[1:], s.CheckedProviders[providerName][:len(s.CheckedProviders[providerName])-1])
	} else {
		s.CheckedProviders[providerName] = append(newHealthCheckList, s.CheckedProviders[providerName]...)
	}
}

func (s *service) evaluateCheckedProviders() {
	for providerName, healthCheckList := range s.CheckedProviders {
		if healthCheckList[0].blockNumber+s.BlockLagLimit < s.LatestBlockNumber {
			s.Providers[providerName].markWarning()
		}
	}
}

func (s *service) getLatestBlockNumber(httpUrl string, headers map[string]string) (int64, int, error) {
	payload := []byte(fmt.Sprintf(`{"jsonrpc":"2.0","method": "%s","params":[],"id":1}`, s.HCMethod))

	// Send the POST request
	resBytes, statusCode, err := s.HTTPClient.Post(httpUrl, headers, []byte(payload))
	if err != nil {
		return 0, 0, errors.Wrap(err, "Error sending POST request")
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

	var blockNumber int64

	switch result := respObject["result"].(type) {
	case string:
		if result == "" || result[:2] != "0x" {
			return 0, 0, errors.New("Invalid block number")
		}

		// Convert the hexadecimal string to an int64
		blockNumber, err = strconv.ParseInt(result[2:], 16, 64)
		if err != nil {
			return 0, 0, errors.Wrap(err, "Error converting block number")
		}
	case float64:
		blockNumber = int64(result)
	default:
		return 0, 0, errors.New("unsupported block number type")
	}
	return blockNumber, *statusCode, nil
}

func (s *service) close() {
	close(s.quit)
}
