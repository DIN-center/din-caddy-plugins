package modules

import (
	"context"
	"net/http"
	"net/http/httptest"
	reflect "reflect"
	"strings"
	"testing"
	"time"

	dingo "github.com/DIN-center/din-caddy-plugins/lib/dingo"
	dinsdk "github.com/DIN-center/din-sc/apps/din-go/lib/din"

	prom "github.com/DIN-center/din-caddy-plugins/lib/prometheus"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/golang/mock/gomock"
	"go.uber.org/zap"
)

func TestMiddlewareCaddyModule(t *testing.T) {
	dinMiddleware := new(DinMiddleware)

	tests := []struct {
		name   string
		output caddy.ModuleInfo
	}{
		{
			name: "TestMiddlewareCaddyModuleInit",
			output: caddy.ModuleInfo{
				ID:  "http.handlers.din",
				New: func() caddy.Module { return new(DinMiddleware) },
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modInfo := dinMiddleware.CaddyModule()
			if modInfo.ID != tt.output.ID {
				t.Errorf("CaddyModule() = %v, want %v", modInfo.ID, tt.output.ID)
			}
			if reflect.TypeOf(modInfo.New()) != reflect.TypeOf(tt.output.New()) {
				t.Errorf("CaddyModule() = %v, want %v", modInfo.New(), tt.output.New())
			}
		})
	}
}

func TestMiddlewareServeHTTP(t *testing.T) {
	dinMiddleware := new(DinMiddleware)
	dinMiddleware.testMode = true

	// Large payload to test max request payload size. This is greater than 1KB.
	largePayload := `{";;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
	;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
	;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
	;;;;;;;;;;;;;;;;;;;;;;;;;;":";;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
	;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
	;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
	;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
	;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
	;;;;;;;;;;;;;;;;;;;;;;;;;;;;"}`

	now := time.Now()

	test := []struct {
		name     string
		request  *http.Request
		provider string
		networks map[string]*network
		hasErr   bool
	}{
		{
			name:     "successful request",
			request:  httptest.NewRequest("POST", "http://localhost:8000/eth", strings.NewReader(`{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`)),
			provider: "localhost:8000",
			networks: map[string]*network{
				"eth": {
					Name: "eth",
					Providers: map[string]*provider{
						"localhost:8000": {
							healthStatus: Healthy,
						},
					},
					CheckedProviders: map[string][]healthCheckEntry{
						"localhost:8000": {
							{
								blockNumber: 1,
								timestamp:   &now,
							},
						},
					},
					MaxRequestPayloadSizeKB: DefaultMaxRequestPayloadSizeKB,
				},
			},
			hasErr: false,
		},
		{
			name:     "unsuccesful request, payload too large",
			request:  httptest.NewRequest("POST", "http://localhost:8000/eth", strings.NewReader(largePayload)),
			provider: "localhost:8000",
			networks: map[string]*network{
				"eth": {
					Name: "eth",
					Providers: map[string]*provider{
						"localhost:8000": {
							healthStatus: Healthy,
						},
					},
					CheckedProviders: map[string][]healthCheckEntry{
						"localhost:8000": {
							{
								blockNumber: 1,
								timestamp:   &now,
							},
						},
					},
					MaxRequestPayloadSizeKB: 0,
				},
			},
			hasErr: true,
		},
		{
			name:    "unsuccessful request, path not found",
			request: httptest.NewRequest("GET", "http://localhost:8000/xxx", nil),
			networks: map[string]*network{
				"eth": {},
			},
			hasErr: true,
		},
		{
			name:     "unsuccessful request, network map is empty",
			request:  httptest.NewRequest("GET", "http://localhost:8000/eth", nil),
			networks: map[string]*network{},
			hasErr:   true,
		},
	}

	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			dinMiddleware.Networks = tt.networks
			tt.request = tt.request.WithContext(context.WithValue(tt.request.Context(), caddy.ReplacerCtxKey, caddy.NewReplacer()))
			rw := httptest.NewRecorder()

			repl := tt.request.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
			repl.Set(RequestProviderKey, tt.provider)

			// bodyBytes, err := io.ReadAll(tt.request.Body)
			// if err != nil {
			// 	t.Errorf("ServeHTTP() = %v, want %v", err, nil)
			// }
			// repl.Set(RequestBodyKey, bodyBytes)

			err := dinMiddleware.ServeHTTP(rw, tt.request, caddyhttp.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error { return nil }))
			if err == nil && tt.hasErr {
				t.Errorf("ServeHTTP() = %v, want %v", err, tt.hasErr)
			} else if err != nil && !tt.hasErr {
				t.Errorf("ServeHTTP() = %v, want %v", err, tt.hasErr)
			}
		})
	}
}

func TestDinMiddlewareProvision(t *testing.T) {
	dinMiddleware := new(DinMiddleware)
	dinMiddleware.testMode = true
	mockCtrl := gomock.NewController(t)
	mockPrometheusClient := prom.NewMockIPrometheusClient(mockCtrl)
	logger := zap.NewNop()

	tests := []struct {
		name     string
		networks map[string]*network
		hasErr   bool
	}{
		{
			name: "Provision() populated 1 network, 2 upstreams successful for ethereum runtime",
			networks: map[string]*network{
				"eth": {
					Name:        "eth",
					HCThreshold: 2,
					HCInterval:  5,
					Providers: map[string]*provider{
						"localhost:8000": {
							HttpUrl: "http://localhost:8000/eth",
						},
						"localhost:8001": {
							HttpUrl: "http://localhost:8001/eth",
						},
					},
					CheckedProviders: map[string][]healthCheckEntry{},
					PrometheusClient: mockPrometheusClient,
					logger:           logger,
				},
			},
			hasErr: false,
		},
		{
			name: "Provision() populated 2 network, 1 upstreams successful",
			networks: map[string]*network{
				"eth": {
					Name:        "eth",
					HCThreshold: 2,
					HCInterval:  5,
					Providers: map[string]*provider{
						"localhost:8000": {
							HttpUrl: "http://localhost:8000/eth",
						},
					},
					CheckedProviders: map[string][]healthCheckEntry{},
					PrometheusClient: mockPrometheusClient,
					logger:           logger,
				},
				"starknet-mainnet": {
					Name:        "eth",
					HCMethod:    "starknet_blockNumber",
					HCThreshold: 2,
					HCInterval:  5,
					Providers: map[string]*provider{
						"localhost:8000": {
							HttpUrl: "http://localhost:8000/starknet-mainnet",
						},
					},
					CheckedProviders: map[string][]healthCheckEntry{},
					PrometheusClient: mockPrometheusClient,
					logger:           logger,
				},
			},
			hasErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dinMiddleware.Networks = tt.networks
			mockPrometheusClient.EXPECT().HandleLatestBlockMetric(gomock.Any()).AnyTimes()
			err := dinMiddleware.Provision(caddy.Context{})
			if err != nil && !tt.hasErr {
				t.Errorf("Provision() = %v, want %v", err, tt.hasErr)
			}

			for _, networks := range dinMiddleware.Networks {
				for _, provider := range networks.Providers {
					if provider.upstream.Dial == "" || provider.path == "" {
						t.Errorf("Provision() = %v, want %v", err, tt.hasErr)
					}
				}
			}
		})
	}
}
func TestUnmarshalCaddyfile(t *testing.T) {
	dinMiddleware := new(DinMiddleware)

	tests := []struct {
		name      string
		caddyfile string
		hasErr    bool
	}{
		{
			name: "Valid Caddyfile",
			caddyfile: `networks {
				eth {
					methods eth_blockNumber eth_getBlockByNumber
					providers {
						localhost:8000 {
							headers {
								Content-Type application/json
							}
							priority 1
						}
						localhost:8001 {
							headers {
								Content-Type application/json
							}
							priority 2
						}
					}
					healthcheck_method GET
					healthcheck_threshold 2
					healthcheck_interval 5
					healthcheck_blocklag_limit 10
					max_request_payload_size_kb 100
				}
			}`,
			hasErr: false,
		},
		{
			name: "Invalid Caddyfile - Missing provider",
			caddyfile: `networks {
				eth {
					methods methods eth_blockNumber eth_getBlockByNumber
					healthcheck_method eth_blockNumber
					healthcheck_threshold 2
					healthcheck_interval 5
					healthcheck_blocklag_limit 10
					max_request_payload_size_kb 100
				}
			}`,
			hasErr: true,
		},
		{
			name: "Invalid Caddyfile - Invalid 'methods' argument",
			caddyfile: `networks {
				eth {
					methods
					providers {
						localhost:8000 {
							headers {
								Content-Type application/json
							}
							priority 1
						}
					}
					healthcheck_method GET
					healthcheck_threshold 2
					healthcheck_interval 5
					healthcheck_blocklag_limit 10
					max_request_payload_size_kb 100
				}
			}`,
			hasErr: true,
		},
		{
			name: "Invalid Caddyfile - Invalid 'headers' argument",
			caddyfile: `networks {
				eth {
					methods eth_blockNumber eth_getBlockByNumber
					providers {
						localhost:8000 {
							headers
							priority 1
						}
					}
				}
			}`,
			hasErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dispenser := caddyfile.NewTestDispenser(tt.caddyfile)
			err := dinMiddleware.UnmarshalCaddyfile(dispenser)
			if err != nil && !tt.hasErr {
				t.Errorf("UnmarshalCaddyfile() = %v, want %v", err, tt.hasErr)
			}
		})
	}
}

func TestSyncRegistryWithLatestBlock(t *testing.T) {
	logger := zap.NewNop()
	mockCtrl := gomock.NewController(t)
	mockDingoClient := dingo.NewMockIDingoClient(mockCtrl)
	dinMiddleware := &DinMiddleware{
		RegistryEnv:                         LineaMainnet,
		RegistryBlockEpoch:                  10,
		RegistryLastUpdatedEpochBlockNumber: 40,
		logger:                              logger,
		DingoClient:                         mockDingoClient,
	}

	tests := []struct {
		name                                string
		registryLastUpdatedEpochBlockNumber int64
		latestBlockNumber                   int64
		expectedUpdateCall                  bool
		expectedBlockFloorByEpoch           int64
	}{
		{
			name:                                "Sync should update as block difference is equal to or exceeds epoch 50",
			registryLastUpdatedEpochBlockNumber: int64(40),
			latestBlockNumber:                   int64(50),
			expectedUpdateCall:                  true,
			expectedBlockFloorByEpoch:           int64(50),
		},
		{
			name:                                "Sync should update as block difference is equal to or exceeds epoch 52",
			registryLastUpdatedEpochBlockNumber: int64(40),
			latestBlockNumber:                   int64(52),
			expectedUpdateCall:                  true,
			expectedBlockFloorByEpoch:           int64(50),
		},
		{
			name:                                "Sync should update as block difference is equal to or exceeds epoch 1000",
			registryLastUpdatedEpochBlockNumber: int64(40),
			latestBlockNumber:                   int64(1001),
			expectedUpdateCall:                  true,
			expectedBlockFloorByEpoch:           int64(1000),
		},
		{
			name:                                "Sync should not update as block difference is less than epoch 48",
			registryLastUpdatedEpochBlockNumber: int64(40),
			latestBlockNumber:                   int64(48),
			expectedUpdateCall:                  false,
			expectedBlockFloorByEpoch:           int64(40),
		},
		{
			name:                                "Sync should not update as block difference is less than epoch 30",
			registryLastUpdatedEpochBlockNumber: int64(40),
			latestBlockNumber:                   int64(30),
			expectedUpdateCall:                  false,
			expectedBlockFloorByEpoch:           int64(40),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			dinMiddleware.Networks = map[string]*network{
				LineaMainnet: {
					LatestBlockNumber: tt.latestBlockNumber,
				},
			}
			dinMiddleware.RegistryLastUpdatedEpochBlockNumber = tt.registryLastUpdatedEpochBlockNumber

			// Check if update was called as expected
			if tt.expectedUpdateCall {
				mockDingoClient.EXPECT().GetDataFromRegistry().Return(&dinsdk.DinRegistryData{}, nil).Times(1)
			}
			// Call the function
			dinMiddleware.syncRegistryWithLatestBlock()

			// Validate that RegistryLastUpdatedEpochBlockNumber is updated correctly
			if dinMiddleware.RegistryLastUpdatedEpochBlockNumber != tt.expectedBlockFloorByEpoch {
				t.Errorf("Expected RegistryLastUpdatedEpochBlockNumber = %v, got %v", tt.expectedBlockFloorByEpoch, dinMiddleware.RegistryLastUpdatedEpochBlockNumber)
			}
		})
	}
}
