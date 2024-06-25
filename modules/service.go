package modules

import (
	"fmt"
	"time"

	din_http "github.com/openrelayxyz/din-caddy-plugins/lib/http"
	"github.com/openrelayxyz/din-caddy-plugins/lib/runtime"
	"github.com/openrelayxyz/din-caddy-plugins/lib/runtime/ethereum"
	"github.com/openrelayxyz/din-caddy-plugins/lib/runtime/solana"
	"github.com/openrelayxyz/din-caddy-plugins/lib/runtime/starknet"
)

type service struct {
	Name              string               `json:"name"`
	Providers         map[string]*provider `json:"providers"`
	Methods           []*string            `json:"methods"`
	runtimeClient     runtime.IRuntimeClient
	quit              chan struct{}
	LatestBlockNumber int64 `json:"latest_block_number"`

	// Healthcheck configuration
	checkedProviders map[string][]healthCheckEntry `json:"checked_providers"`
	Runtime          string                        `json:"runtime"`
	HCInterval       int                           `json:"healthceck.interval.seconds"`
	HCThreshold      int                           `json:"healthcheck.threshold"`
	BlockLagLimit    int64                         `json:"healthcheck.blocklag.limit"`
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
	// TODO: check all of the providers simultaneously for more accurate blocknumber results.
	for _, provider := range s.Providers {
		// get the latest block number from the current provider
		providerBlockNumber, statusCode, err := s.runtimeClient.GetLatestBlockNumber(provider.HttpUrl, provider.Headers)
		if err != nil {
			fmt.Println(err, "Error getting latest block number for provider", provider.host, "on service", s.Name)
			provider.markPingFailure(s.HCThreshold)
			continue
		}
		blockTime = time.Now()

		// Ping Health Check
		// TODO: If status code is marked as 429, mark to warning.
		if statusCode > 399 {
			// if the status code is greater than 399, mark the provider as a failure
			provider.markPingFailure(s.HCThreshold)
			continue
		} else {
			provider.markPingSuccess(s.HCThreshold)
		}

		// TODO: if the block number is behind the latest block number, mark to warning.
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
			// if the current provider's latest block number is below the service's latest block number by more than the acceptable threshold, set the current provider to unhealthy
			provider.markUnhealthy()
		}
		// add the current provider to the checked providers map
		s.addNewBlockNumberToCheckedProviders(provider.upstream.Dial, providerBlockNumber, blockTime)
	}
}

// addNewBlockNumberToCheckedProviders adds a new healthCheckEntry to the beginning of the checkedProviders healthCheck list for the given provider
// the list will not exceed 10 entries
func (s *service) addNewBlockNumberToCheckedProviders(providerName string, blockNumber int64, timestamp time.Time) {
	// if the provider is not in the checked providers map, add it with its initial block number and timestamp
	if _, ok := s.checkedProviders[providerName]; !ok {
		s.checkedProviders[providerName] = make([]healthCheckEntry, 10)
		s.checkedProviders[providerName][0] = healthCheckEntry{blockNumber: blockNumber, timestamp: &timestamp}
		return
	}

	// to add a new healthCheckEntry to index 0 of the provider's slice, we need to make a new slice and copy the old slice to the new slice
	newProviderSlice := make([]healthCheckEntry, 10)
	newProviderSlice[0] = healthCheckEntry{blockNumber: blockNumber, timestamp: &timestamp}

	// if the old slice is full at 10 entries, we need to remove the last entry and copy the rest of the entries to the new slice
	if len(s.checkedProviders) == 10 {
		copy(newProviderSlice[1:], s.checkedProviders[providerName][:len(s.checkedProviders[providerName])-1])
	} else {
		copy(newProviderSlice[1:], s.checkedProviders[providerName])
	}

	// once the new slice is created, we can set the checkedProviders map value to the new slice
	s.checkedProviders[providerName] = newProviderSlice
}

func (s *service) evaluateCheckedProviders() {
	for providerName, healthCheckList := range s.checkedProviders {
		if healthCheckList[0].blockNumber+s.BlockLagLimit < s.LatestBlockNumber {
			s.Providers[providerName].markUnhealthy()
		}
	}
}

func (s *service) getRuntimeClient(httpClient *din_http.HTTPClient) runtime.IRuntimeClient {
	switch s.Runtime {
	case SolanaRuntime:
		return solana.NewSolanaClient(httpClient)
	case StarknetRuntime:
		return starknet.NewStarknetClient(httpClient)
	default:
		return ethereum.NewEthereumClient(httpClient)
	}
}

func (s *service) close() {
	close(s.quit)
}
