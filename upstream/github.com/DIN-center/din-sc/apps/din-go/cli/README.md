# DIN CLI Tool

The DIN CLI tool provides a user-friendly interface for reading from and writing to the DIN Registry Smart Contracts. It supports the main registry entities including `Networks` and `Providers`.

## Installation

The DIN CLI is built as part of the `din-go` build process. From the monodin project root, run:

```bash
pnpm run build:all
```

The CLI binary is installed under `dist/apps/din-go/bin/din-cli`. Add this path to your current `$PATH` environment variable or use the binary directly from the build directory.

## Getting Started

Once the DIN CLI is installed, you can explore all available commands by typing `din-cli --help`:

```
A CLI tool for managing networks, providers, and their states in the DIN registry.

Usage:
  din-cli [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  network     Manage networks
  provider    Manage providers

Flags:
      --din-registry-contract-addr string   The DIN Registry contract address, takes precedence over environment variable
  -h, --help                                help for din-cli
      --json                                Output in JSON format
      --keystore-path string                The path to the keystore file (wallet credentials), takes precedence over environment variable
      --rpc-url string                      The RPC URL to connect to, takes precedence over environment variable
      --tx-confirmation-sec int             The maximum number of seconds to wait for a transaction confirmation. Applies to all commands that write to the registry (default 10)

Use "din-cli [command] --help" for more information about a command.
```

**Note:** All commands require the DIN Registry contract address and RPC URL to be configured. This can be achieved using either global flags or system environment variables.

### Configuration Options

#### Using Global Flags
Set the following global flags:
- `--din-registry-contract-addr`: Smart contract address where the DIN Registry is deployed
- `--rpc-url`: JSON-RPC endpoint URL (supports local blockchain nodes like Anvil)
- `--json`: Output results in JSON format instead of human-readable format
- `--tx-confirmation-sec`: Number of seconds to wait for transaction confirmation before displaying success status and gas consumption details

#### Using Environment Variables
For convenience and brevity in commands, you may prefer to use system environment variables:

```bash
export DIN_REGISTRY_CONTRACT_ADDR=<0x...>
export RPC_URL=http://<endpoint>
```

## Network Commands

The `network` command provides functionality to manage networks in the DIN registry.

### List Networks
```bash
din-cli network list [--uri <network://flavor>] [--full]
```

**Options:**
- `--uri`: Filter by network URI in the format 'network://flavor'
- `--full`: Show all details of the network(s) including methods and providers

**Examples:**
```bash
# List all networks
din-cli network list

# List a specific network
din-cli network list --uri ethereum://mainnet

# List all networks with full details
din-cli network list --full
```

### Set Network Status

**Required Flags:**
- `--uri`: Network URI in the format 'network://flavor'
- `--status`: Network status (None, Onboarding, Active, Maintenance, Decommissioned, Retired)
- Either the flag `--keystore-path` or environment variable `KEYSTORE_PATH` must be set

```bash
din-cli network set-status --uri ethereum://mainnet --status Retired
```

See more details in [Writing to the DIN Registry](#writing-to-the-din-registry) section on how to set the credentials required to modify the DIN Registry.

### Set Network Configuration
**Required Flags:**
- `--uri`: Network URI in the format 'network://flavor'
- `--config-json`: Network configuration in JSON format (as in the output format when the `--json` flag is passed, see "network_config" field)
- Either the flag `--keystore-path` or environment variable `KEYSTORE_PATH` must be set

```bash
din-cli network set-config --uri ethereum://mainnet --config-json '{"health_check_interval_sec": 30, "block_lag_limit": 10}'
```

**Note on `--config-json` flag:** This is a JSON string that should be passed to the command. You can update multiple fields at the same time. If you provide the same values as currently stored on-chain, no change is made (see the command output).
Valid fields are the same as output via the command `din-cli network list --uri <network://flavor> --json`.

See more details in [Writing to the DIN Registry](#writing-to-the-din-registry) section on how to set the credentials required to modify the DIN Registry.

## Provider Commands

The `provider` command provides functionality to manage providers in the DIN registry.

### List Providers
```bash
din-cli provider list [--network-uri <network://flavor>] [--name <provider_name>]
```

**Options:**
- `--network-uri`: Filter providers by network URI
- `--name`: Filter providers by name

**Examples:**
```bash
# List all providers
din-cli provider list

# List providers for a specific network
din-cli provider list --network-uri ethereum://mainnet

# List a specific provider by name
din-cli provider list --name "Infura"
```

### Set Provider Status
**Required Flags:**
- `--contract-addr`: Provider contract address
- `--status`: Provider status (None, Onboarding, Active, Maintenance, Retired)
- Either the flag `--keystore-path` or environment variable `KEYSTORE_PATH` must be set

**Example:**
```bash
din-cli provider set-status --contract-addr 0x8E8A3c0547c9A17F051D2E5CAf7E0e21C0719E1C --status Active
```

See more details in [Writing to the DIN Registry](#writing-to-the-din-registry) section on how to set the credentials required to modify the DIN Registry.

### Remove Provider

**Required Flags:**
- `--contract-addr`: Provider contract address
- Either the flag `--keystore-path` or environment variable `KEYSTORE_PATH` must be set

**Note:** This command will prompt for confirmation if the provider has registered network services.

```bash
din-cli provider remove --contract-addr 0x8E8A3c0547c9A17F051D2E5CAf7E0e21C0719E1C
```

See more details in [Writing to the DIN Registry](#writing-to-the-din-registry) section on how to set the credentials required to modify the DIN Registry.

## Writing to the DIN Registry

All write operations to the DIN Registry require authentication using a keystore file. The CLI will prompt for the keystore password when performing write operations.

### Authentication Setup
1. Ensure you have a keystore file containing your wallet credentials (you can create one using the `cast wallet` command)
2. Set the keystore path using either:
   - Global flag: `--keystore-path /path/to/keystore`
   - Environment variable: `export KEYSTORE_PATH=/path/to/keystore`
3. When prompted, enter your keystore password

### Write Operations
The following commands modify the registry and require authentication:
- `network set-status`: Update network status
- `network set-config`: Update network configuration
- `provider set-status`: Update provider status
- `provider remove`: Remove a provider from the registry

## DevOps Support

The CLI provides JSON output support for automation and integration with other tools.

### JSON Output
Use the `--json` flag to get machine-readable output:

```bash
# Get JSON output for network listing
din-cli network list --json

# Get JSON output for provider listing
din-cli provider list --json
```

### Automation Examples

#### Filtering with jq
```bash
# Get only active networks
din-cli network list --json | jq '.[] | select(.status == "Active")'

# Get provider addresses for a specific network
din-cli provider list --network-uri ethereum://mainnet --json | jq '.[].address'

# Count total providers
din-cli provider list --json | jq length
```

#### Scripting Examples
```bash
#!/bin/bash
# Check if a specific network exists
if din-cli network list --uri ethereum://mainnet --json | jq -e '.[]' > /dev/null; then
    echo "Network exists"
else
    echo "Network not found"
fi

# Monitor provider status changes
din-cli provider list --json | jq '.[] | {name: .name, status: .status, address: .address}'

# Implement an ad-hoc monitoring system with alerting
while true; do
    unhealthy=$(din-cli provider list --json | \
        jq -r '.[] | select(.status != "Active") | .name')

    if [ -n "$unhealthy" ]; then
        echo "Unhealthy providers detected: $unhealthy"
        # Send alert notification
    fi 

    sleep 300 # Check every 5 minutes
done
```

#### CI/CD Integration
```bash
# Validate network configuration in CI
din-cli network list --uri ethereum://mainnet --json | jq -e '.network_config.health_check_interval_sec > 0' || exit 1

# Automated status updates
din-cli network set-status --uri ethereum://mainnet --status Active
```

### Error Handling
The CLI provides clear error messages and appropriate exit codes for automation:
- Exit code 0: Success
- Exit code 1: Error (with descriptive message)

All JSON output maintains consistent structure for reliable parsing in automated workflows.
