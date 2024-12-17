package modules

import (
	"fmt"
	"net/http"
	"testing"

	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
)

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name   string
		urlstr string
		output *provider
		hasErr bool
	}{
		{
			name:   "passing localhost",
			urlstr: "http://localhost:8080",
			output: &provider{
				HttpUrl:  "http://localhost:8080",
				host:     "localhost:8080",
				path:     "",
				Headers:  make(map[string]string),
				Priority: 0,
			},
			hasErr: false,
		},
		{
			name:   "passing fullurl with key",
			urlstr: "https://eth.rpc.test.cloud:443/key",
			output: &provider{
				HttpUrl:  "https://eth.rpc.test.cloud:443/key",
				host:     "eth.rpc.test.cloud:443",
				Headers:  make(map[string]string),
				Priority: 0,
			},
			hasErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewProvider(tt.urlstr)
			if err != nil && !tt.hasErr {
				t.Errorf("urlToProviderObject() = %v, want %v", err, tt.hasErr)
			}
			if provider.HttpUrl != tt.output.HttpUrl {
				t.Errorf("HttpUrl = %v, want %v", provider.HttpUrl, tt.output.HttpUrl)
			}
			if provider.host != tt.output.host {
				t.Errorf("host = %v, want %v", provider.host, tt.output.host)
			}
			if provider.path != tt.output.path {
				t.Errorf("path = %v, want %v", provider.path, tt.output.path)
			}
			if len(provider.Headers) != len(tt.output.Headers) {
				t.Errorf("Headers length = %v, want %v", len(provider.Headers), len(tt.output.Headers))
			}
			if provider.Priority != tt.output.Priority {
				t.Errorf("priority = %v, want %v", provider.Priority, tt.output.Priority)
			}
		})
	}
}

func TestAvailable(t *testing.T) {
	tests := []struct {
		name     string
		provider *provider
		output   bool
	}{
		{
			name: "Available with healthy upstream",
			provider: &provider{
				healthStatus: Healthy,
				upstream: &reverseproxy.Upstream{
					Dial: "localhost:8080",
				},
			},
			output: true,
		},
		{
			name: "Available with unhealthy upstream",
			provider: &provider{
				healthStatus: Unhealthy,
				upstream: &reverseproxy.Upstream{
					Dial: "localhost:8080",
				},
			},
			output: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.provider.Available() != tt.output {
				t.Errorf("Available() = %v, want %v", tt.provider.Available(), tt.output)
			}
		})
	}
}

func TestMarkPingFailure(t *testing.T) {
	tests := []struct {
		name     string
		hcThresh int
		provider *provider
		output   HealthStatus
	}{
		{
			name: "markPingFailure with 0 threshold",
			provider: &provider{
				failures:     0,
				successes:    0,
				healthStatus: Healthy,
			},
			hcThresh: 0,
			output:   Unhealthy,
		},
		{
			name: "markPingFailure with 1 threshold",
			provider: &provider{
				failures:     0,
				successes:    0,
				healthStatus: Healthy,
			},
			hcThresh: 1,
			output:   Healthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.provider.markPingFailure(tt.hcThresh)
			if tt.provider.healthStatus != tt.output {
				t.Errorf("markPingFailure() = %v, want %v", tt.provider.healthStatus, tt.output)
			}
		})
	}
}

func TestMarkPingSuccess(t *testing.T) {
	tests := []struct {
		name     string
		hcThresh int
		provider *provider
		output   HealthStatus
	}{
		{
			name: "markPingSuccess with 0 threshold",
			provider: &provider{
				failures:     0,
				successes:    0,
				healthStatus: Unhealthy,
			},
			hcThresh: 0,
			output:   Healthy,
		},
		{
			name: "markPingSuccess with 1 threshold",
			provider: &provider{
				failures:     0,
				successes:    0,
				healthStatus: Unhealthy,
			},
			hcThresh: 1,
			output:   Unhealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.provider.markPingSuccess(tt.hcThresh)
			if tt.provider.healthStatus != tt.output {
				t.Errorf("markPingSuccess() = %v, want %v", tt.provider.healthStatus, tt.output)
			}
		})
	}
}

func TestMarkHealthy(t *testing.T) {
	tests := []struct {
		name                  string
		hcThresh              int
		provider              *provider
		expectedHealthStatus  HealthStatus
		expectedConsecutiveHC int
	}{
		{
			name: "markHealthy when already healthy",
			provider: &provider{
				healthStatus:             Healthy,
				consecutiveHealthyChecks: 5,
			},
			hcThresh:              3,
			expectedHealthStatus:  Healthy,
			expectedConsecutiveHC: 0,
		},
		{
			name: "markHealthy when unhealthy - not enough consecutive checks",
			provider: &provider{
				healthStatus:             Unhealthy,
				consecutiveHealthyChecks: 2,
			},
			hcThresh:              3,
			expectedHealthStatus:  Unhealthy,
			expectedConsecutiveHC: 3,
		},
		{
			name: "markHealthy when unhealthy - threshold reached",
			provider: &provider{
				healthStatus:             Unhealthy,
				consecutiveHealthyChecks: 3,
			},
			hcThresh:              3,
			expectedHealthStatus:  Healthy,
			expectedConsecutiveHC: 0,
		},
		{
			name: "markHealthy when warning",
			provider: &provider{
				healthStatus:             Warning,
				consecutiveHealthyChecks: 2,
			},
			hcThresh:              3,
			expectedHealthStatus:  Healthy,
			expectedConsecutiveHC: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.provider.markHealthy(tt.hcThresh)
			if tt.provider.healthStatus != tt.expectedHealthStatus {
				t.Errorf("healthStatus = %v, want %v", tt.provider.healthStatus, tt.expectedHealthStatus)
			}
			if tt.provider.consecutiveHealthyChecks != tt.expectedConsecutiveHC {
				t.Errorf("consecutiveHealthyChecks = %v, want %v", tt.provider.consecutiveHealthyChecks, tt.expectedConsecutiveHC)
			}
		})
	}
}

func TestMarkWarning(t *testing.T) {
	tests := []struct {
		name                  string
		provider              *provider
		expectedHealthStatus  HealthStatus
		expectedConsecutiveHC int
	}{
		{
			name: "markWarning when healthy",
			provider: &provider{
				healthStatus:             Healthy,
				consecutiveHealthyChecks: 5,
			},
			expectedHealthStatus:  Warning,
			expectedConsecutiveHC: 0,
		},
		{
			name: "markWarning when unhealthy",
			provider: &provider{
				healthStatus:             Unhealthy,
				consecutiveHealthyChecks: 3,
			},
			expectedHealthStatus:  Warning,
			expectedConsecutiveHC: 0,
		},
		{
			name: "markWarning when already warning",
			provider: &provider{
				healthStatus:             Warning,
				consecutiveHealthyChecks: 2,
			},
			expectedHealthStatus:  Warning,
			expectedConsecutiveHC: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.provider.markWarning()
			if tt.provider.healthStatus != tt.expectedHealthStatus {
				t.Errorf("healthStatus = %v, want %v", tt.provider.healthStatus, tt.expectedHealthStatus)
			}
			if tt.provider.consecutiveHealthyChecks != tt.expectedConsecutiveHC {
				t.Errorf("consecutiveHealthyChecks = %v, want %v", tt.provider.consecutiveHealthyChecks, tt.expectedConsecutiveHC)
			}
		})
	}
}

func TestMarkUnhealthy(t *testing.T) {
	tests := []struct {
		name                  string
		provider              *provider
		expectedHealthStatus  HealthStatus
		expectedConsecutiveHC int
	}{
		{
			name: "markUnhealthy when healthy",
			provider: &provider{
				healthStatus:             Healthy,
				consecutiveHealthyChecks: 5,
			},
			expectedHealthStatus:  Unhealthy,
			expectedConsecutiveHC: 0,
		},
		{
			name: "markUnhealthy when warning",
			provider: &provider{
				healthStatus:             Warning,
				consecutiveHealthyChecks: 3,
			},
			expectedHealthStatus:  Unhealthy,
			expectedConsecutiveHC: 0,
		},
		{
			name: "markUnhealthy when already unhealthy",
			provider: &provider{
				healthStatus:             Unhealthy,
				consecutiveHealthyChecks: 2,
			},
			expectedHealthStatus:  Unhealthy,
			expectedConsecutiveHC: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.provider.markUnhealthy()
			if tt.provider.healthStatus != tt.expectedHealthStatus {
				t.Errorf("healthStatus = %v, want %v", tt.provider.healthStatus, tt.expectedHealthStatus)
			}
			if tt.provider.consecutiveHealthyChecks != tt.expectedConsecutiveHC {
				t.Errorf("consecutiveHealthyChecks = %v, want %v", tt.provider.consecutiveHealthyChecks, tt.expectedConsecutiveHC)
			}
		})
	}
}

func TestIntToHex(t *testing.T) {
	tests := []struct {
		name string
		n    int64
		want string
	}{
		{
			name: "convert 0",
			n:    0,
			want: "0x0",
		},
		{
			name: "convert positive number",
			n:    255,
			want: "0xff",
		},
		{
			name: "convert large number",
			n:    1000000,
			want: "0xf4240",
		},
		{
			name: "convert negative number",
			n:    -255,
			want: "0x-ff",
		},
		{
			name: "convert max int64",
			n:    9223372036854775807,
			want: "0x7fffffffffffffff",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := intToHex(tt.n)
			if got != tt.want {
				t.Errorf("intToHex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHexToInt(t *testing.T) {
	tests := []struct {
		name    string
		hex     string
		want    int64
		wantErr bool
	}{
		{
			name:    "convert 0x0",
			hex:     "0x0",
			want:    0,
			wantErr: false,
		},
		{
			name:    "convert 0xff",
			hex:     "0xff",
			want:    255,
			wantErr: false,
		},
		{
			name:    "convert large hex",
			hex:     "0xf4240",
			want:    1000000,
			wantErr: false,
		},
		{
			name:    "convert max value",
			hex:     "0x7fffffffffffffff",
			want:    9223372036854775807,
			wantErr: false,
		},
		{
			name:    "invalid hex prefix",
			hex:     "ff",
			wantErr: true,
		},
		{
			name:    "invalid hex characters",
			hex:     "0xgg",
			wantErr: true,
		},
		{
			name:    "empty string",
			hex:     "",
			wantErr: true,
		},
		{
			name:    "only prefix",
			hex:     "0x",
			wantErr: true,
		},
		{
			name:    "overflow value",
			hex:     "0x8000000000000000",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := hexToInt(tt.hex)
			if (err != nil) != tt.wantErr {
				t.Errorf("hexToInt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("hexToInt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSaveLatestBlockNumber(t *testing.T) {
	tests := []struct {
		name        string
		provider    *provider
		blockNumber int64
		want        uint64
	}{
		{
			name:        "save zero block number",
			provider:    &provider{},
			blockNumber: 0,
			want:        0,
		},
		{
			name:        "save positive block number",
			provider:    &provider{},
			blockNumber: 1000000,
			want:        1000000,
		},
		{
			name:        "overwrite existing block number",
			provider:    &provider{latestBlockNumber: 500000},
			blockNumber: 1000000,
			want:        1000000,
		},
		{
			name:        "save max int64 block number",
			provider:    &provider{},
			blockNumber: 9223372036854775807,
			want:        9223372036854775807,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.provider.saveLatestBlockNumber(tt.blockNumber)
			if err != nil {
				t.Errorf("saveLatestBlockNumber() error = %v", err)
			}
			if tt.provider.latestBlockNumber != tt.want {
				t.Errorf("saveLatestBlockNumber() got = %v, want %v", tt.provider.latestBlockNumber, tt.want)
			}
		})
	}
}

func TestSaveEarliestBlockNumber(t *testing.T) {
	tests := []struct {
		name        string
		provider    *provider
		blockNumber int64
		want        uint64
	}{
		{
			name:        "save zero block number",
			provider:    &provider{},
			blockNumber: 0,
			want:        0,
		},
		{
			name:        "save positive block number",
			provider:    &provider{},
			blockNumber: 1000,
			want:        1000,
		},
		{
			name:        "overwrite existing block number",
			provider:    &provider{earliestBlockNumber: 500},
			blockNumber: 1000,
			want:        1000,
		},
		{
			name:        "save max int64 block number",
			provider:    &provider{},
			blockNumber: 9223372036854775807,
			want:        9223372036854775807,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.provider.saveEarliestBlockNumber(tt.blockNumber)
			if err != nil {
				t.Errorf("saveEarliestBlockNumber() error = %v", err)
			}
			if tt.provider.earliestBlockNumber != tt.want {
				t.Errorf("saveEarliestBlockNumber() got = %v, want %v", tt.provider.earliestBlockNumber, tt.want)
			}
		})
	}
}

func TestGetLatestBlockNumber(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockHttpClient := din_http.NewMockIHTTPClient(mockCtrl)

	tests := []struct {
		name           string
		hcMethod       string
		provider       *provider
		mockResponse   []byte
		mockStatusCode int
		mockError      error
		want           int64
		wantStatus     int
		wantErr        bool
	}{
		{
			name:     "successful hex response",
			hcMethod: "eth_blockNumber",
			provider: &provider{
				HttpUrl:    "http://localhost:8545",
				httpClient: mockHttpClient,
			},
			mockResponse:   []byte(`{"jsonrpc":"2.0","id":1,"result":"0xff"}`),
			mockStatusCode: 200,
			mockError:      nil,
			want:           255,
			wantStatus:     200,
			wantErr:        false,
		},
		{
			name:     "successful decimal response",
			hcMethod: "eth_blockNumber",
			provider: &provider{
				HttpUrl:    "http://localhost:8545",
				httpClient: mockHttpClient,
			},
			mockResponse:   []byte(`{"jsonrpc":"2.0","id":1,"result":255}`),
			mockStatusCode: 200,
			mockError:      nil,
			want:           255,
			wantStatus:     200,
			wantErr:        false,
		},
		{
			name:     "service unavailable",
			hcMethod: "eth_blockNumber",
			provider: &provider{
				HttpUrl:    "http://localhost:8545",
				httpClient: mockHttpClient,
			},
			mockResponse:   []byte{},
			mockStatusCode: http.StatusServiceUnavailable,
			mockError:      nil,
			want:           0,
			wantStatus:     http.StatusServiceUnavailable,
			wantErr:        true,
		},
		{
			name:     "invalid json response",
			hcMethod: "eth_blockNumber",
			provider: &provider{
				HttpUrl:    "http://localhost:8545",
				httpClient: mockHttpClient,
			},
			mockResponse:   []byte(`invalid json`),
			mockStatusCode: 200,
			mockError:      nil,
			want:           0,
			wantStatus:     0,
			wantErr:        true,
		},
		{
			name:     "missing result field",
			hcMethod: "eth_blockNumber",
			provider: &provider{
				HttpUrl:    "http://localhost:8545",
				httpClient: mockHttpClient,
			},
			mockResponse:   []byte(`{"jsonrpc":"2.0","id":1}`),
			mockStatusCode: 200,
			mockError:      nil,
			want:           0,
			wantStatus:     0,
			wantErr:        true,
		},
		{
			name:     "invalid block number format",
			hcMethod: "eth_blockNumber",
			provider: &provider{
				HttpUrl:    "http://localhost:8545",
				httpClient: mockHttpClient,
			},
			mockResponse:   []byte(`{"jsonrpc":"2.0","id":1,"result":"invalid"}`),
			mockStatusCode: 200,
			mockError:      nil,
			want:           0,
			wantStatus:     0,
			wantErr:        true,
		},
		{
			name:     "http client error",
			hcMethod: "eth_blockNumber",
			provider: &provider{
				HttpUrl:    "http://localhost:8545",
				httpClient: mockHttpClient,
			},
			mockResponse:   nil,
			mockStatusCode: 0,
			mockError:      errors.New("connection error"),
			want:           0,
			wantStatus:     0,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup expected HTTP client call
			expectedPayload := []byte(fmt.Sprintf(`{"jsonrpc":"2.0","method": "%s","params":[],"id":1}`, tt.hcMethod))
			mockHttpClient.EXPECT().
				Post(tt.provider.HttpUrl, tt.provider.Headers, expectedPayload, tt.provider.AuthClient()).
				Return(tt.mockResponse, &tt.mockStatusCode, tt.mockError)

			got, gotStatus, err := tt.provider.getLatestBlockNumber(tt.hcMethod)
			if (err != nil) != tt.wantErr {
				t.Errorf("getLatestBlockNumber() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getLatestBlockNumber() got = %v, want %v", got, tt.want)
			}
			if gotStatus != tt.wantStatus {
				t.Errorf("getLatestBlockNumber() gotStatus = %v, want %v", gotStatus, tt.wantStatus)
			}
		})
	}
}

func TestGetEarliestBlockNumber(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockHttpClient := din_http.NewMockIHTTPClient(mockCtrl)

	tests := []struct {
		name            string
		hcMethod        string
		provider        *provider
		mockResponses   [][]byte
		mockStatusCodes []int
		mockErrors      []error
		want            int64
		wantStatus      int
		wantErr         bool
	}{
		{
			name:     "successful response for block 0",
			hcMethod: "eth_getBlockByNumber",
			provider: &provider{
				HttpUrl:    "http://localhost:8545",
				httpClient: mockHttpClient,
			},
			mockResponses:   [][]byte{[]byte(`{"jsonrpc":"2.0","id":1,"result":{"number":"0x0"}}`)},
			mockStatusCodes: []int{200},
			mockErrors:      []error{nil},
			want:            0,
			wantStatus:      200,
			wantErr:         false,
		},
		{
			name:     "block 0 not found, binary search finds block 1",
			hcMethod: "eth_getBlockByNumber",
			provider: &provider{
				HttpUrl:           "http://localhost:8545",
				httpClient:        mockHttpClient,
				latestBlockNumber: 10, // Set a small range for testing
			},
			mockResponses: [][]byte{
				[]byte(`{"jsonrpc":"2.0","id":1,"result":null}`),             // Block 0 not found
				[]byte(`{"jsonrpc":"2.0","id":1,"result":{"number":"0x5"}}`), // Mid point (5)
				[]byte(`{"jsonrpc":"2.0","id":1,"result":{"number":"0x2"}}`), // Mid point (2)
				[]byte(`{"jsonrpc":"2.0","id":1,"result":{"number":"0x1"}}`), // Found block 1
			},
			mockStatusCodes: []int{200, 200, 200, 200},
			mockErrors:      []error{nil, nil, nil, nil},
			want:            1,
			wantStatus:      200,
			wantErr:         false,
		},
		{
			name:     "service unavailable during initial check",
			hcMethod: "eth_getBlockByNumber",
			provider: &provider{
				HttpUrl:    "http://localhost:8545",
				httpClient: mockHttpClient,
			},
			mockResponses:   [][]byte{[]byte{}},
			mockStatusCodes: []int{http.StatusServiceUnavailable},
			mockErrors:      []error{nil},
			want:            0,
			wantStatus:      http.StatusServiceUnavailable,
			wantErr:         true,
		},
		{
			name:     "service unavailable during binary search",
			hcMethod: "eth_getBlockByNumber",
			provider: &provider{
				HttpUrl:           "http://localhost:8545",
				httpClient:        mockHttpClient,
				latestBlockNumber: 10,
			},
			mockResponses: [][]byte{
				[]byte(`{"jsonrpc":"2.0","id":1,"result":null}`), // Block 0 not found
				[]byte{}, // Service unavailable during binary search
			},
			mockStatusCodes: []int{200, http.StatusServiceUnavailable},
			mockErrors:      []error{nil, nil},
			want:            0,
			wantStatus:      http.StatusServiceUnavailable,
			wantErr:         true,
		},
		{
			name:     "invalid json response during binary search",
			hcMethod: "eth_getBlockByNumber",
			provider: &provider{
				HttpUrl:           "http://localhost:8545",
				httpClient:        mockHttpClient,
				latestBlockNumber: 10,
			},
			mockResponses: [][]byte{
				[]byte(`{"jsonrpc":"2.0","id":1,"result":null}`), // Block 0 not found
				[]byte(`invalid json`),                           // Invalid JSON during binary search
			},
			mockStatusCodes: []int{200, 200},
			mockErrors:      []error{nil, nil},
			want:            0,
			wantStatus:      0,
			wantErr:         true,
		},
		{
			name:     "no blocks found during binary search",
			hcMethod: "eth_getBlockByNumber",
			provider: &provider{
				HttpUrl:           "http://localhost:8545",
				httpClient:        mockHttpClient,
				latestBlockNumber: 2,
			},
			mockResponses: [][]byte{
				[]byte(`{"jsonrpc":"2.0","id":1,"result":null}`), // Block 0 not found
				[]byte(`{"jsonrpc":"2.0","id":1,"result":null}`), // Block 1 not found
				[]byte(`{"jsonrpc":"2.0","id":1,"result":null}`), // Block 2 not found
			},
			mockStatusCodes: []int{200, 200, 200},
			mockErrors:      []error{nil, nil, nil},
			want:            0,
			wantStatus:      0,
			wantErr:         true,
		},
		{
			name:     "http client error during initial check",
			hcMethod: "eth_getBlockByNumber",
			provider: &provider{
				HttpUrl:    "http://localhost:8545",
				httpClient: mockHttpClient,
			},
			mockResponses:   [][]byte{nil},
			mockStatusCodes: []int{0},
			mockErrors:      []error{errors.New("connection error")},
			want:            0,
			wantStatus:      0,
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup expected HTTP client calls
			for i, response := range tt.mockResponses {
				statusCode := tt.mockStatusCodes[i]
				err := tt.mockErrors[i]

				var expectedPayload []byte
				if i == 0 {
					// First call is always for block 0
					expectedPayload = []byte(fmt.Sprintf(`{"jsonrpc":"2.0","method": "%s","params":["0x0", false],"id":1}`, tt.hcMethod))
				} else {
					// Skip checking exact payload for binary search calls as they're dynamic
					mockHttpClient.EXPECT().
						Post(tt.provider.HttpUrl, tt.provider.Headers, gomock.Any(), tt.provider.AuthClient()).
						Return(response, &statusCode, err)
					continue
				}

				mockHttpClient.EXPECT().
					Post(tt.provider.HttpUrl, tt.provider.Headers, expectedPayload, tt.provider.AuthClient()).
					Return(response, &statusCode, err)
			}

			got, gotStatus, err := tt.provider.getEarliestBlockNumber(tt.hcMethod, 1)
			if (err != nil) != tt.wantErr {
				t.Errorf("getEarliestBlockNumber() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getEarliestBlockNumber() got = %v, want %v", got, tt.want)
			}
			if gotStatus != tt.wantStatus {
				t.Errorf("getEarliestBlockNumber() gotStatus = %v, want %v", gotStatus, tt.wantStatus)
			}
		})
	}
}
