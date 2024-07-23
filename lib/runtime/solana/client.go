package solana

import (
	"encoding/json"
	"net/http"

	din_http "github.com/openrelayxyz/din-caddy-plugins/lib/http"
	"github.com/pkg/errors"
)

type SolanaClient struct {
	HTTPClient *din_http.HTTPClient
}

func NewSolanaClient(httpClient *din_http.HTTPClient) *SolanaClient {
	return &SolanaClient{
		HTTPClient: httpClient,
	}
}

func (s *SolanaClient) GetLatestBlockNumber(httpUrl string, headers map[string]string) (int64, int, error) {
	// Send the POST request to get the latest block
	payload := []byte(`{"jsonrpc":"2.0", "method":"getBlockHeight","params":[],"id":1}`)
	// Send the POST request
	resBytes, statusCode, err := s.HTTPClient.Post(httpUrl, headers, []byte(payload))
	if err != nil {
		return 0, 0, errors.Wrap(err, "Error sending POST request")
	}
	if *statusCode == http.StatusServiceUnavailable {
		return 0, 0, errors.New("503 error Service Unavailable")
	}

	// response struct
	var result struct {
		Jsonrpc string `json:"jsonrpc"`
		Id      int    `json:"id"`
		Result  int    `json:"result"`
	}

	// Unmarshal the response
	err = json.Unmarshal(resBytes, &result)
	if err != nil {
		return 0, 0, errors.Wrap(err, "Error unmarshalling response")
	}

	return int64(result.Result), *statusCode, nil
}
