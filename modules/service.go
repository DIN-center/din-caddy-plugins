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

	Runtime     string `json:"runtime"`
	HCInterval  int    `json:"healthceck.interval.seconds"`
	HCThreshold int    `json:"healthcheck.threshold"`
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

func (s *service) healthCheck() {
	if s.LatestBlockNumber == 0 {
		fmt.Print("\n\n")
		fmt.Println("initialization of service ", s.Name, nil)
		fmt.Print("\n")
	} else {
		fmt.Print("\n\n")
		fmt.Println("previous service block number ", s.Name, s.LatestBlockNumber)
		fmt.Print("\n")
	}

	checkedProviders := make(map[string]int64)

	for _, provider := range s.Providers {
		// get the latest block number from the current provider
		latestBlockNumber, statusCode, err := s.runtimeClient.GetLatestBlockNumber(provider.HttpUrl, provider.Headers)
		if err != nil {
			fmt.Println("error getting latest block number", err)
			continue
		}

		// Ping Health Check
		if statusCode > 399 {
			// if the status code is greater than 399, mark the provider as a failure
			provider.markPingFailure(s.HCThreshold)
			continue
		} else {
			provider.markPingSuccess(s.HCThreshold)
		}

		// Consistency health check
		if s.LatestBlockNumber == 0 || s.LatestBlockNumber < latestBlockNumber {
			// if the current provider's latest block number is greater than the service's latest block number, update the service's latest block number,
			// set the current provider as healthy and loop through all of the previously checked providers and set them as unhealthy
			s.LatestBlockNumber = latestBlockNumber
			fmt.Println("new service latest block number", s.LatestBlockNumber)

			provider.healthy = true

			for _, blockNumber := range checkedProviders {
				if blockNumber != s.LatestBlockNumber {
					provider.healthy = false
				}
			}
		} else if s.LatestBlockNumber == latestBlockNumber {
			// if the current provider's latest block number is equal to the service's latest block number, set the current provider to healthy
			provider.healthy = true
		} else {
			// if the current provider's latest block number is less than the service's latest block number, set the current provider to unhealthy
			provider.healthy = false
		}
		// add the current provider to the checked providers map
		checkedProviders[provider.upstream.Dial] = latestBlockNumber
		fmt.Println("provider:", provider.upstream.Dial, "provider latest block number", latestBlockNumber)
	}

	for hostName := range checkedProviders {
		healthyStatus := "unhealthy"
		if s.Providers[hostName].Healthy() {
			healthyStatus = "healthy"
		}
		fmt.Println(s.Providers[hostName].upstream.Dial, "status", healthyStatus)
	}
	fmt.Println()
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
