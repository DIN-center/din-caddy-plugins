package modules

import (
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	networklib "github.com/DIN-center/din-caddy-plugins/lib/network"
	"github.com/DIN-center/din-caddy-plugins/lib/utils"
)

// TestHandlerSetOnce verifies that handlers are only set once during the entire flow
func TestHandlerSetOnce(t *testing.T) {
	// Test the full flow: UnmarshalCaddyfile -> Provision
	d := &DinMiddleware{
		Networks:        make(map[string]*network),
		handlerRegistry: networklib.DefaultRegistry,
		logger:          logger.NewLoggerClient(zap.NewNop(), utils.EnvTest),
	}

	// Simulate Caddyfile parsing
	caddyfileContent := `networks {
		ethereum-mainnet {
			chain_id "0x1"
			handler evm
			providers {
				http://localhost:8545
			}
		}
	}`

	dispenser := caddyfile.NewTestDispenser(caddyfileContent)
	err := d.UnmarshalCaddyfile(dispenser)
	require.NoError(t, err)

	// Check network exists after UnmarshalCaddyfile
	ethNetwork := d.Networks["ethereum-mainnet"]
	require.NotNil(t, ethNetwork)
	// Handler should NOT be set yet after UnmarshalCaddyfile
	require.Nil(t, ethNetwork.handler)

	// Simulate Provision
	d.testMode = true // Use test mode to skip external dependencies
	ctx := caddy.Context{}
	err = d.Provision(ctx)
	require.NoError(t, err)

	// Verify handler is now set after Provision
	require.NotNil(t, ethNetwork.handler, "Handler should be set after Provision")
}

// TestHandlerSetForJSONLoadedConfig verifies handlers are properly initialized for JSON-loaded configs
func TestHandlerSetForJSONLoadedConfig(t *testing.T) {
	// Create a network as if loaded from JSON (no handler)
	d := &DinMiddleware{
		Networks:        make(map[string]*network),
		handlerRegistry: networklib.DefaultRegistry,
		logger:          logger.NewLoggerClient(zap.NewNop(), utils.EnvTest),
		testMode:        true,
	}

	// Manually create network without handler (simulating JSON load)
	network := &network{
		Name:        "ethereum-mainnet",
		HandlerType: EVMHandler,
		ChainId:     "0x1",
		Providers: map[string]*provider{
			"localhost:8545": {
				HttpUrl: "http://localhost:8545",
				host:    "localhost:8545",
			},
		},
	}
	d.Networks["ethereum-mainnet"] = network

	// Verify no handler initially
	assert.Nil(t, network.handler)

	// Run Provision
	ctx := caddy.Context{}
	err := d.Provision(ctx)
	require.NoError(t, err)

	// Verify handler was set during Provision
	assert.NotNil(t, network.handler)
	assert.Equal(t, string(EVMHandler), network.handler.GetType())
}

// TestHandlerNotSetForEmptyType verifies no handler is set for networks without type
func TestHandlerNotSetForEmptyType(t *testing.T) {
	d := &DinMiddleware{
		Networks:        make(map[string]*network),
		handlerRegistry: networklib.DefaultRegistry,
		logger:          logger.NewLoggerClient(zap.NewNop(), utils.EnvTest),
		testMode:        true,
	}

	// Create network without type
	network := &network{
		Name:        "unknown-network",
		HandlerType: "", // No handler type specified
		ChainId:     "unknown:1",
		Providers: map[string]*provider{
			"localhost:8545": {
				HttpUrl: "http://localhost:8545",
				host:    "localhost:8545",
			},
		},
	}
	d.Networks["unknown-network"] = network

	// Run Provision
	ctx := caddy.Context{}
	err := d.Provision(ctx)
	require.NoError(t, err)

	// Verify no handler was set
	assert.Nil(t, network.handler)
}
