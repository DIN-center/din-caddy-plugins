package modules

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	reflect "reflect"
	"strings"
	"testing"
	"time"

	"github.com/DIN-center/din-caddy-plugins/lib/auth/siwe"
	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
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

func TestInitialize(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	tests := []struct {
		name                 string
		initialDinMiddleware *DinMiddleware
		expectedError        error
	}{
		{
			name: "Successful initialization",
			initialDinMiddleware: &DinMiddleware{
				Networks: map[string]*network{
					"test-network": {
						Providers: map[string]*provider{
							"provider1": {
								HttpUrl: "http://example1.com",
							},
						},
					},
				},
				testMode: true,
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Create logger and mock context
			logger := zaptest.NewLogger(t)

			// Setup DinMiddleware object
			dinMiddleware := tt.initialDinMiddleware
			dinMiddleware.machineID = "test-machine-id"
			dinMiddleware.logger = logger

			// Call the initialize function
			err := dinMiddleware.initialize(caddy.Context{})

			// Assert the expected results
			if tt.expectedError != nil {
				assert.NotNil(t, err)
			} else {
				assert.NoError(t, err)

				// Assert default values are set if not provided
				assert.NotZero(t, dinMiddleware.RegistryBlockCheckIntervalSec)
				assert.NotZero(t, dinMiddleware.RegistryBlockEpoch)
				assert.Equal(t, 0, dinMiddleware.RegistryPriority)

				// // Assert networks and providers are initialized
				for _, network := range dinMiddleware.Networks {
					assert.NotNil(t, network.HttpClient)
					assert.NotNil(t, network.logger)
					for _, provider := range network.Providers {
						assert.NotNil(t, provider.httpClient)
						assert.NotNil(t, provider.upstream)
					}
				}
			}
		})
	}
}

func TestInitializeProvider(t *testing.T) {
	tests := []struct {
		name          string
		provider      *provider
		httpClient    *din_http.HTTPClient
		expectedError string
	}{
		{
			name: "Successful initialization with http URL",
			provider: &provider{
				HttpUrl: "http://example2.com",
				Auth:    nil,
			},
			httpClient:    &din_http.HTTPClient{},
			expectedError: "",
		},
		{
			name: "Successful initialization with https URL",
			provider: &provider{
				HttpUrl: "https://example3.com",
				Auth:    nil,
			},
			httpClient:    &din_http.HTTPClient{},
			expectedError: "",
		},
		{
			name: "Successful initialization with auth",
			provider: &provider{
				HttpUrl: "http://example4.com",
				Auth: &siwe.SIWEClientAuth{
					ProviderURL: "http://auth.example.com",
				},
			},
			httpClient:    &din_http.HTTPClient{},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			dinMiddleware := &DinMiddleware{
				logger:    logger,
				machineID: "test-machine-id",
			}

			err := dinMiddleware.initializeProvider(tt.provider, tt.httpClient, logger)

			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)

				// Parse the expected URL
				parsedURL, err := url.Parse(tt.provider.HttpUrl)
				assert.NoError(t, err)

				expectedHost := parsedURL.Host
				if parsedURL.Scheme == "https" && parsedURL.Port() == "" {
					expectedHost = parsedURL.Host + ":443"
				}

				// Assert provider upstream
				assert.Equal(t, expectedHost, tt.provider.upstream.Dial)
				// Assert provider path and host
				assert.Equal(t, parsedURL.Path, tt.provider.path)
				assert.Equal(t, parsedURL.Host, tt.provider.host)
				// Assert provider httpClient
				assert.Equal(t, tt.httpClient, tt.provider.httpClient)

				// Check if Auth is started if it exists
				if tt.provider.Auth != nil {
					assert.NotNil(t, tt.provider.Auth)
				}

				// Assert provider logger is set to the middleware logger
				assert.Equal(t, dinMiddleware.logger, tt.provider.logger)
			}
		})
	}
}

func TestDinMiddlewareProvision(t *testing.T) {
	tests := []struct {
		name            string
		networks        map[string]*network
		registryEnabled bool
		initializeErr   error
		expectedError   error
	}{
		{
			name: "Successful provision in test mode",
			networks: map[string]*network{
				"test-network": {},
			},
			expectedError: nil,
		},
		{
			name:            "network not found but registry is enabled",
			registryEnabled: true,
			networks:        map[string]*network{},
			expectedError:   nil,
		},
		{
			name:          "minimum of 1 network not found",
			networks:      map[string]*network{},
			expectedError: fmt.Errorf("expected at least 1 network or registry to be defined"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			dinMiddleware := &DinMiddleware{
				testMode: true, // Ensure test mode is enabled
				logger:   logger,
				Networks: tt.networks,
			}
			if tt.registryEnabled {
				dinMiddleware.RegistryEnabled = tt.registryEnabled
			}

			// Call the Provision method
			err := dinMiddleware.Provision(caddy.Context{})

			// Assert results based on the case
			if tt.expectedError != nil {
				assert.Equal(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, true, dinMiddleware.testMode)
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
