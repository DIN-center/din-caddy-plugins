package modules

import (
	"testing"

	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	"github.com/DIN-center/din-caddy-plugins/lib/utils"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestReflectionBasedConfigParsing(t *testing.T) {
	tests := []struct {
		name           string
		caddyfile      string
		expectedValues map[string]interface{}
		expectedFlags  map[string]bool
	}{
		{
			name: "Test all healthcheck config fields and flags",
			caddyfile: `networks {
				eth {
					chain_id 0x1
					healthcheck_threshold 5
					healthcheck_timeout 30
					healthcheck_interval 60
					healthcheck_blocklag_limit 100
					healthcheck_blockjump_limit 50
					healthcheck_provider_block_history_size 20
					network_block_history_size 10
					max_request_payload_size_kb 512
					request_attempt_count 3
					archive_enabled true
					providers {
						http://test.com/eth {
							priority 1
						}
					}
				}
			}`,
			expectedValues: map[string]interface{}{
				"HCThreshold":              5,
				"HCTimeout":                30,
				"HCInterval":               60,
				"BlockLagLimit":            int64(100),
				"BlockJumpLimit":           int64(50),
				"ProviderBlockHistorySize": 20,
				"NetworkBlockHistorySize":  10,
				"MaxRequestPayloadSizeKB":  int64(512),
				"RequestAttemptCount":      3,
				"ArchiveEnabled":           true,
			},
			expectedFlags: map[string]bool{
				"HCThresholdSetInCaddyfile":              true,
				"HCTimeoutSetInCaddyfile":                true,
				"HCIntervalSetInCaddyfile":               true,
				"BlockLagLimitSetInCaddyfile":            true,
				"BlockJumpLimitSetInCaddyfile":           true,
				"ProviderBlockHistorySizeSetInCaddyfile": true,
				"NetworkBlockHistorySizeSetInCaddyfile":  true,
				"MaxRequestPayloadSizeKBSetInCaddyfile":  true,
				"RequestAttemptCountSetInCaddyfile":      true,
				"ArchiveEnabledSetInCaddyfile":           true,
			},
		},
		{
			name: "Test partial config with defaults",
			caddyfile: `networks {
				eth {
					chain_id 0x1
					healthcheck_interval 120
					max_request_payload_size_kb 1024
					providers {
						http://test.com/eth {
							priority 1
						}
					}
				}
			}`,
			expectedValues: map[string]interface{}{
				"HCInterval":              120,
				"MaxRequestPayloadSizeKB": int64(1024),
				// Check that defaults are preserved
				"HCThreshold":              DefaultHCThreshold,
				"HCTimeout":                DefaultHCTimeout,
				"BlockLagLimit":            DefaultBlockLagLimit,
				"BlockJumpLimit":           DefaultBlockJumpLimit,
				"ProviderBlockHistorySize": DefaultProviderBlockHistorySize,
				"NetworkBlockHistorySize":  DefaultNetworkBlockHistorySize,
				"RequestAttemptCount":      DefaultRequestAttemptCount,
				"ArchiveEnabled":           DefaultArchiveEnabled,
			},
			expectedFlags: map[string]bool{
				"HCIntervalSetInCaddyfile":              true,
				"MaxRequestPayloadSizeKBSetInCaddyfile": true,
				// Verify unset flags remain false
				"HCThresholdSetInCaddyfile":              false,
				"HCTimeoutSetInCaddyfile":                false,
				"BlockLagLimitSetInCaddyfile":            false,
				"BlockJumpLimitSetInCaddyfile":           false,
				"ProviderBlockHistorySizeSetInCaddyfile": false,
				"NetworkBlockHistorySizeSetInCaddyfile":  false,
				"RequestAttemptCountSetInCaddyfile":      false,
				"ArchiveEnabledSetInCaddyfile":           false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create DinMiddleware instance
			dinMiddleware := new(DinMiddleware)
			dinMiddleware.logger = logger.NewLoggerClient(zap.NewNop(), utils.EnvTest)

			// Parse Caddyfile
			dispenser := caddyfile.NewTestDispenser(tt.caddyfile)
			err := dinMiddleware.UnmarshalCaddyfile(dispenser)
			assert.NoError(t, err)

			// Get the network
			network, exists := dinMiddleware.Networks["eth"]
			assert.True(t, exists, "Network 'eth' should exist")
			assert.NotNil(t, network.CaddyfileFlags, "CaddyfileFlags should not be nil")

			// Check values
			for fieldName, expectedValue := range tt.expectedValues {
				switch fieldName {
				case "HCThreshold":
					assert.Equal(t, expectedValue, network.HCThreshold, "Field %s", fieldName)
				case "HCTimeout":
					assert.Equal(t, expectedValue, network.HCTimeout, "Field %s", fieldName)
				case "HCInterval":
					assert.Equal(t, expectedValue, network.HCInterval, "Field %s", fieldName)
				case "BlockLagLimit":
					assert.Equal(t, expectedValue, network.BlockLagLimit, "Field %s", fieldName)
				case "BlockJumpLimit":
					assert.Equal(t, expectedValue, network.BlockJumpLimit, "Field %s", fieldName)
				case "ProviderBlockHistorySize":
					assert.Equal(t, expectedValue, network.ProviderBlockHistorySize, "Field %s", fieldName)
				case "NetworkBlockHistorySize":
					assert.Equal(t, expectedValue, network.NetworkBlockHistorySize, "Field %s", fieldName)
				case "MaxRequestPayloadSizeKB":
					assert.Equal(t, expectedValue, network.MaxRequestPayloadSizeKB, "Field %s", fieldName)
				case "RequestAttemptCount":
					assert.Equal(t, expectedValue, network.RequestAttemptCount, "Field %s", fieldName)
				case "ArchiveEnabled":
					assert.Equal(t, expectedValue, network.ArchiveEnabled, "Field %s", fieldName)
				}
			}

			// Check flags
			for flagName, expectedFlag := range tt.expectedFlags {
				switch flagName {
				case "HCThresholdSetInCaddyfile":
					assert.Equal(t, expectedFlag, network.CaddyfileFlags.HCThresholdSetInCaddyfile, "Flag %s", flagName)
				case "HCTimeoutSetInCaddyfile":
					assert.Equal(t, expectedFlag, network.CaddyfileFlags.HCTimeoutSetInCaddyfile, "Flag %s", flagName)
				case "HCIntervalSetInCaddyfile":
					assert.Equal(t, expectedFlag, network.CaddyfileFlags.HCIntervalSetInCaddyfile, "Flag %s", flagName)
				case "BlockLagLimitSetInCaddyfile":
					assert.Equal(t, expectedFlag, network.CaddyfileFlags.BlockLagLimitSetInCaddyfile, "Flag %s", flagName)
				case "BlockJumpLimitSetInCaddyfile":
					assert.Equal(t, expectedFlag, network.CaddyfileFlags.BlockJumpLimitSetInCaddyfile, "Flag %s", flagName)
				case "ProviderBlockHistorySizeSetInCaddyfile":
					assert.Equal(t, expectedFlag, network.CaddyfileFlags.ProviderBlockHistorySizeSetInCaddyfile, "Flag %s", flagName)
				case "NetworkBlockHistorySizeSetInCaddyfile":
					assert.Equal(t, expectedFlag, network.CaddyfileFlags.NetworkBlockHistorySizeSetInCaddyfile, "Flag %s", flagName)
				case "MaxRequestPayloadSizeKBSetInCaddyfile":
					assert.Equal(t, expectedFlag, network.CaddyfileFlags.MaxRequestPayloadSizeKBSetInCaddyfile, "Flag %s", flagName)
				case "RequestAttemptCountSetInCaddyfile":
					assert.Equal(t, expectedFlag, network.CaddyfileFlags.RequestAttemptCountSetInCaddyfile, "Flag %s", flagName)
				case "ArchiveEnabledSetInCaddyfile":
					assert.Equal(t, expectedFlag, network.CaddyfileFlags.ArchiveEnabledSetInCaddyfile, "Flag %s", flagName)
				}
			}
		})
	}
}

func TestReflectionBasedConfigErrorHandling(t *testing.T) {
	tests := []struct {
		name      string
		caddyfile string
		hasError  bool
		errorMsg  string
	}{
		{
			name: "Invalid integer value",
			caddyfile: `networks {
				eth {
					chain_id 0x1
					healthcheck_threshold invalid_number
					providers {
						http://test.com/eth {
							priority 1
						}
					}
				}
			}`,
			hasError: true,
			errorMsg: "invalid healthcheck_threshold",
		},
		{
			name: "Invalid boolean value",
			caddyfile: `networks {
				eth {
					chain_id 0x1
					archive_enabled not_a_bool
					providers {
						http://test.com/eth {
							priority 1
						}
					}
				}
			}`,
			hasError: true,
			errorMsg: "invalid archive_enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dinMiddleware := new(DinMiddleware)
			dinMiddleware.logger = logger.NewLoggerClient(zap.NewNop(), utils.EnvTest)

			dispenser := caddyfile.NewTestDispenser(tt.caddyfile)
			err := dinMiddleware.UnmarshalCaddyfile(dispenser)

			if tt.hasError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCaddyUnmarshallerProviders(t *testing.T) {
	tests := []struct {
		name              string
		caddyfile         string
		expectedProviders map[string]*provider
	}{
		{
			name: "Valid Caddyfile with providers (default values)",
			caddyfile: `networks {
				eth {
					chain_id 0x1
					providers {
						http://test.com/eth {
						}
					}
				}
			}`,
			expectedProviders: map[string]*provider{
				"test.com": {
					HttpUrl:  "http://test.com/eth",
					host:     "test.com",
					Name:     "test",
					Priority: 0,
				},
			},
		},
		{
			name: "Valid Caddyfile with providers (default values)",
			caddyfile: `networks {
				eth {
					chain_id 0x1
					providers {
						http://subdomain.test.com/eth {
							priority 1
							name MyCustomProvider
						}
					}
				}
			}`,
			expectedProviders: map[string]*provider{
				"subdomain.test.com": {
					HttpUrl:  "http://subdomain.test.com/eth",
					host:     "subdomain.test.com",
					Name:     "MyCustomProvider",
					Priority: 1,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dinMiddleware := new(DinMiddleware)
			dinMiddleware.logger = logger.NewLoggerClient(zap.NewNop(), utils.EnvTest)

			dispenser := caddyfile.NewTestDispenser(tt.caddyfile)
			err := dinMiddleware.UnmarshalCaddyfile(dispenser)

			assert.NoError(t, err)

			// Get the network
			network, networkExists := dinMiddleware.Networks["eth"]
			assert.True(t, networkExists, "Network 'eth' should exist")

			assert.Equal(t, len(tt.expectedProviders), len(network.Providers), "Should have %d providers", len(tt.expectedProviders))

			// Check providers
			for _, expectedProvider := range tt.expectedProviders {
				actualProvider, exists := network.Providers[expectedProvider.host]
				assert.True(t, exists, "Provider %s should exist", expectedProvider.host)
				assert.Equal(t, expectedProvider.HttpUrl, actualProvider.HttpUrl, "Provider %s should have the correct HTTP URL", expectedProvider.host)
				assert.Equal(t, expectedProvider.host, actualProvider.host, "Provider %s should have the correct host", expectedProvider.host)
				assert.Equal(t, expectedProvider.Name, actualProvider.Name, "Provider %s should have the correct name", expectedProvider.host)
				assert.Equal(t, expectedProvider.Priority, actualProvider.Priority, "Provider %s should have the correct priority", expectedProvider.host)
			}
		})
	}
}
