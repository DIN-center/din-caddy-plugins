# Contracts Reference

Complete API reference for all DIN Registry smart contracts.

## DinRegistry

**Location**: `apps/din-sc/src/DinRegistry.sol`

The central hub and main entry point for all registry operations.

### State Variables

```solidity
address public immutable dinOwner;                    // Registry owner
NetworkRegistry internal immutable s_networkRegistry; // Network storage
INetwork[] public networks;                           // All networks
Provider[] public providers;                          // All providers
mapping(string => bool) public networkMap;            // Network existence
mapping(Provider => bool) public providerMap;         // Provider existence
mapping(INetwork => Provider[]) public network2providers; // Network → Providers
mapping(bytes32 => bool) public providerNetworkMap;   // Provider-Network links
```

### Provider Management

#### createProvider

Creates and registers a new provider.

```solidity
function createProvider(
    address providerEoa,
    string memory name,
    ProviderAuthConfig memory authConfig,
    ProviderStatus providerStatus
) public onlyDinOwner returns (Provider provider)
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `providerEoa` | `address` | EOA that will own the provider |
| `name` | `string` | Human-readable provider name |
| `authConfig` | `ProviderAuthConfig` | Authentication configuration |
| `providerStatus` | `ProviderStatus` | Initial status (typically `Onboarding`) |

**Access**: DIN Owner only

#### unregisterProvider

Removes a provider from the registry.

```solidity
function unregisterProvider(Provider provider) public
```

**Access**: Provider Owner only (for their own provider)

#### getAllProviders

Returns all registered providers.

```solidity
function getAllProviders() external view returns (Provider[] memory)
```

#### getProviders

Returns providers serving a specific network.

```solidity
function getProviders(string memory networkName)
    public view returns (Provider[] memory)
```

### Network Management

#### createNetwork

Creates a new network.

```solidity
function createNetwork(
    string memory name,
    string memory description,
    NetworkOperationsConfig memory config,
    NetworkStatus initialStatus
) public onlyDinOwner returns (INetwork newNetwork)
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | `string` | Unique network identifier |
| `description` | `string` | Human-readable description |
| `config` | `NetworkOperationsConfig` | Operational configuration |
| `initialStatus` | `NetworkStatus` | Initial status |

**Access**: DIN Owner only

#### addMethodToNetwork

Adds a single method to a network.

```solidity
function addMethodToNetwork(string memory name, string memory method)
    public onlyDinOwner returns (uint8 bit)
```

Returns the assigned bit position (1-255).

#### addMethodsToNetwork

Adds multiple methods to a network.

```solidity
function addMethodsToNetwork(string memory name, string[] calldata names)
    public onlyDinOwner returns (uint256 capabilities)
```

Returns the updated capabilities bitmask.

#### getAllNetworks

Returns all registered networks.

```solidity
function getAllNetworks() public view returns (INetwork[] memory)
```

#### getNetworkFromName

Returns a network by name.

```solidity
function getNetworkFromName(string memory networkName)
    external view returns (INetwork)
```

#### getNetworkCapabilities

Returns the capabilities bitmask for a network.

```solidity
function getNetworkCapabilities(string memory networkName)
    public view returns (uint256 caps)
```

#### getAllNetworkMethodNames

Returns all method names for a network.

```solidity
function getAllNetworkMethodNames(string memory networkName)
    public view returns (string[] memory methods)
```

#### getAllNetworkMethods

Returns all Method structs for a network.

```solidity
function getAllNetworkMethods(string memory networkName)
    external view returns (Method[] memory methods)
```

#### getNetworkOperationsConfig

Returns the operations configuration.

```solidity
function getNetworkOperationsConfig(string memory networkName)
    external view returns (NetworkOperationsConfig memory opsConfig)
```

#### setNetworkOperationsConfig

Updates the operations configuration.

```solidity
function setNetworkOperationsConfig(
    string memory networkName,
    NetworkOperationsConfig memory _opsConfig
) external onlyDinOwner
```

**Access**: DIN Owner only

#### getNetworkStatus / setNetworkStatus

```solidity
function getNetworkStatus(string memory networkName)
    external view returns (NetworkStatus status)

function setNetworkStatus(string memory networkName, NetworkStatus status)
    external onlyDinOwner
```

### NetworkService Management

#### createNetworkService

Creates a network service linking a provider to a network.

```solidity
function createNetworkService(
    string memory name,
    uint256 initialCaps,
    string memory serviceUrl,
    NetworkServiceStatus serviceStatus,
    Provider provider
) public returns (NetworkService networkService)
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | `string` | Network name to serve |
| `initialCaps` | `uint256` | Capabilities bitmask (must be subset of network caps) |
| `serviceUrl` | `string` | Service endpoint URL |
| `serviceStatus` | `NetworkServiceStatus` | Initial status |
| `provider` | `Provider` | Provider contract address |

**Access**: Provider Owner only (for their own provider)

### Errors

```solidity
error NetworkExists();              // Network name already registered
error AuthRequireDINOwner();        // Caller is not DIN owner
error AuthRequireRegisteredProvider(); // Provider not registered
error AuthRequireProviderOwner();   // Caller is not provider owner
error UnknownNetwork();             // Network name not found
```

---

## NetworkRegistry

**Location**: `apps/din-sc/src/NetworkRegistry.sol`

Internal registry managing Network contracts. Created automatically by DinRegistry.

### State Variables

```solidity
address public immutable registryOwner;  // DinRegistry address
string public registryName;              // Registry identifier
uint256 public totalNetworks;            // Network count
```

### Functions

#### createNetwork

```solidity
function createNetwork(
    string memory name,
    string memory description,
    NetworkOperationsConfig memory config,
    NetworkStatus initialStatus
) public onlyRegistryOwner returns (Network newNetwork)
```

**Access**: Registry Owner (DinRegistry) only

#### getNetworkFromName

```solidity
function getNetworkFromName(string memory name)
    public view returns (INetwork network)
```

#### getAllNetworks

```solidity
function getAllNetworks() public view returns (INetwork[] memory allNetworks)
```

#### isRegistered

```solidity
function isRegistered(uint256 networkId) public view returns (bool)
function isRegistered(string memory networkName) public view returns (bool)
```

### Events

```solidity
event NetworkRegistered(string registryName, Network network);
```

---

## Network

**Location**: `apps/din-sc/src/Network.sol`

Represents a blockchain protocol with RPC methods.

### State Variables

```solidity
address public immutable networkOwner;
string private networkName;
string private networkDescription;
NetworkStatus private networkStatus;
NetworkOperationsConfig private opsConfig;
uint256 private capabilities;           // Bitmask of supported methods
uint8 private s_nextBit = 1;            // Next bit to assign
Method[] public methods;
mapping(address => bool) public authenticated;
mapping(string => Method) private s_name2method;
mapping(uint8 => Method) public s_bit2method;
```

### Method Management

#### addMethod

```solidity
function addMethod(string memory name)
    public isAuthenticated returns (uint8 bit)
```

**Access**: Authenticated addresses only

#### addMethods

```solidity
function addMethods(string[] calldata names)
    public isAuthenticated returns (uint256 updatedCapabilities)
```

#### removeMethod

```solidity
function removeMethod(uint8 bit) public onlyOwner
function removeMethod(string memory name) public onlyOwner returns (uint8 bit)
```

**Access**: Network Owner only

Marks method as deactivated (soft delete).

### Query Functions

```solidity
function getName() external view returns (string memory)
function getDescription() external view returns (string memory)
function getCapabilities() public view returns (uint256)
function allMethods() public view returns (Method[] memory)
function getMethodId(string memory name) public view returns (uint8 bit)
function getMethodName(uint8 bit) public view returns (string memory name)
function isMethodSupported(uint8 bit) public view returns (bool supported)
function areCapabilitiesSupported(uint256 caps) public view returns (bool)
function getNetworkOperationsConfig() external view returns (NetworkOperationsConfig memory)
function getNetworkStatus() external view returns (NetworkStatus)
```

### Configuration

```solidity
function setNetworkOperationsConfig(NetworkOperationsConfig calldata config)
    external isAuthenticated

function setNetworkStatus(NetworkStatus status) external isAuthenticated
```

### Events

```solidity
event AddMethodToNetwork(Network network, string method, uint8 bit);
event RemoveMethodFromNetwork(Network network, string method, uint8 bit);
event NetworkStatusUpdated(Network indexed network, NetworkStatus newStatus);
event NetworkOperationConfigUpdated(Network indexed network, NetworkOperationsConfig newConfig);
```

### Errors

```solidity
error NetworkNotAuthorized();
error NetworkAuthNotOwner();
error NetworkAuthNotAuthenticated();
error NetworkMethodAlreadyExists();
error NetworkMethodNotFound();
error NetworkMethodDeactivated();
error NetworkInvalidHealthcheckMethod();
error NetworkInvalidMethodBit();
error NetworkInvalidStatus();
```

---

## Provider

**Location**: `apps/din-sc/src/Provider.sol`

Represents an RPC service provider.

### State Variables

```solidity
address public immutable providerOwner;   // EOA owner
address public immutable dinAddress;      // DinRegistry address
ProviderStatus public providerStatus;
string public name;
ProviderAuthConfig public authConfig;
NetworkService[] public services;
mapping(INetwork => NetworkServiceItem) public serviceMap;
```

### Service Management

#### addNetworkService

```solidity
function addNetworkService(
    INetwork network,
    uint256 initialCaps,
    string memory serviceUrl,
    NetworkServiceStatus status
) public isAuthorized isUnknownService(network) returns (NetworkService service)
```

**Access**: Provider Owner or DinRegistry

#### removeNetworkService

```solidity
function removeNetworkService(INetwork network)
    public isAuthorized isKnownNetworkService(network)
```

Uses swap-and-pop for O(1) removal.

#### getAllNetworkServices

```solidity
function getAllNetworkServices() public view returns (NetworkService[] memory)
```

#### serviceCount

```solidity
function serviceCount() public view returns (uint256 count)
```

### Status Management

```solidity
function setProviderStatus(ProviderStatus newStatus) public isAuthorized
```

### Events

```solidity
event AddNetworkServiceToProvider(
    INetwork indexed network,
    NetworkService service,
    uint256 initialCaps,
    NetworkServiceStatus status
);
event RemoveNetworkServiceFromProvider(INetwork indexed network, NetworkService service);
event ProviderStatusUpdated(Provider indexed provider, ProviderStatus newStatus);
```

### Errors

```solidity
error NoProviderNetworkServices();
error UnknownProviderService();
error ProviderServiceAlreadyExists();
error AuthNotProviderOwner();
```

---

## NetworkService

**Location**: `apps/din-sc/src/NetworkService.sol`

Links a Provider to a Network with specific capabilities.

### State Variables

```solidity
INetwork public immutable inetwork;       // Associated network
address public immutable serviceOwner;    // Provider owner's EOA
uint256 public capabilities;              // Supported method bitmask
string public serviceUrl;                 // Service endpoint
NetworkServiceStatus private serviceStatus;
```

### Functions

#### setCapabilities

```solidity
function setCapabilities(uint256 caps) public onlyServiceOwner
```

Validates that `caps` is a subset of network capabilities.

#### setStatus

```solidity
function setStatus(NetworkServiceStatus status) public onlyServiceOwner
```

#### isMethodSupported

```solidity
function isMethodSupported(uint8 bit) public view returns (bool supported)
```

#### getAllMethodNames

```solidity
function getAllMethodNames() public view returns (string[] memory methods)
```

Returns method names by iterating through capability bits.

#### getStatus

```solidity
function getStatus() public view returns (NetworkServiceStatus status)
```

### Events

```solidity
event CapabilitiesUpdated(NetworkService service, uint256 capabilities);
event ServiceStatusUpdated(NetworkService service, NetworkServiceStatus status);
```

### Errors

```solidity
error AuthNotServiceOwner();
error NetworkHasNoMethods();
error CapabilitiesNotSupported();
```

---

## INetwork Interface

**Location**: `apps/din-sc/src/INetwork.sol`

Shared interface and type definitions.

### Full Interface

```solidity
interface INetwork {
    function networkOwner() external view returns (address owner);
    function getName() external view returns (string memory name);
    function getDescription() external view returns (string memory description);
    function getCapabilities() external view returns (uint256 capabilities);
    function addMethod(string calldata name) external returns (uint8 bit);
    function addMethods(string[] calldata names) external returns (uint256 updatedCapabilities);
    function removeMethod(uint8 bit) external;
    function removeMethod(string calldata name) external returns (uint8 bit);
    function isMethodSupported(uint8 bit) external view returns (bool supported);
    function areCapabilitiesSupported(uint256 caps) external view returns (bool supported);
    function getMethodId(string calldata name) external view returns (uint8 bit);
    function getMethodName(uint8 bit) external view returns (string memory name);
    function allMethods() external view returns (Method[] memory methods);
    function getNetworkOperationsConfig() external view returns (NetworkOperationsConfig memory config);
    function setNetworkOperationsConfig(NetworkOperationsConfig calldata config) external;
    function getNetworkStatus() external view returns (NetworkStatus status);
    function setNetworkStatus(NetworkStatus status) external;
}
```
