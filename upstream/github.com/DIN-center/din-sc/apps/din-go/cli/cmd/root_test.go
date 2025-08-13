package dincli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCommand(t *testing.T) {

	t.Run("root command has persistent flags", func(t *testing.T) {
		flags := rootCmd.PersistentFlags()

		// Test din-registry-contract-addr flag
		contractFlag := flags.Lookup("din-registry-contract-addr")
		require.NotNil(t, contractFlag)
		assert.Equal(t, "string", contractFlag.Value.Type())

		// Test rpc-url flag
		rpcFlag := flags.Lookup("rpc-url")
		require.NotNil(t, rpcFlag)
		assert.Equal(t, "string", rpcFlag.Value.Type())

		// Test keystore-path flag
		keystoreFlag := flags.Lookup("keystore-path")
		require.NotNil(t, keystoreFlag)
		assert.Equal(t, "string", keystoreFlag.Value.Type())

		// Test json flag
		jsonFlag := flags.Lookup("json")
		require.NotNil(t, jsonFlag)
		assert.Equal(t, "bool", jsonFlag.Value.Type())
	})

	t.Run("root command has subcommands", func(t *testing.T) {
		commands := rootCmd.Commands()
		commandNames := make([]string, len(commands))
		for i, cmd := range commands {
			commandNames[i] = cmd.Name()
		}

		assert.Contains(t, commandNames, "network")
		assert.Contains(t, commandNames, "provider")
	})

	t.Run("persistent pre-run sets formatter", func(t *testing.T) {
		//set required flags
		rootCmd.PersistentFlags().Set("din-registry-contract-addr", "0x123")
		rootCmd.PersistentFlags().Set("rpc-url", "https://mainnet.infura.io/v3/123")

		// Test with json flag
		rootCmd.PersistentFlags().Set("json", "true")
		rootCmd.PersistentPreRun(rootCmd, []string{})
		assert.IsType(t, &JSONFormatter{}, outputFormatter)

		// Test without json flag
		rootCmd.PersistentFlags().Set("json", "false")
		rootCmd.PersistentPreRun(rootCmd, []string{})
		assert.IsType(t, &PrettyPrinterFormatter{}, outputFormatter)
	})
}
