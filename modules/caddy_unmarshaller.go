package modules

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.uber.org/zap"

	"github.com/DIN-center/din-caddy-plugins/lib/auth/oidc"
	"github.com/DIN-center/din-caddy-plugins/lib/auth/siwe"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	networklib "github.com/DIN-center/din-caddy-plugins/lib/network"
	"github.com/DIN-center/din-caddy-plugins/lib/utils"
)

// caddyfileParser encapsulates the parsing logic for Caddyfile
type caddyfileParser struct {
	middleware       *DinMiddleware
	dispenser        *caddyfile.Dispenser
	siweSignerClient siwe.ISIWESignerClient
	caddyPort        string
}

// newCaddyfileParser creates a new parser instance
func newCaddyfileParser(d *DinMiddleware, dispenser *caddyfile.Dispenser) *caddyfileParser {
	return &caddyfileParser{
		middleware:       d,
		dispenser:        dispenser,
		siweSignerClient: siwe.NewSIWESignerClient(),
	}
}

// UnmarshalCaddyfile sets up reverse proxy provider and method data based on the Caddyfile configuration
func (d *DinMiddleware) UnmarshalCaddyfile(dispenser *caddyfile.Dispenser) error {
	parser := newCaddyfileParser(d, dispenser)
	return parser.parse()
}

// ParseCaddyfile is called by httpcaddyfile to parse the Caddyfile configuration
func (d *DinMiddleware) ParseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	err := d.UnmarshalCaddyfile(h.Dispenser)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// parse is the main parsing entry point
func (p *caddyfileParser) parse() error {
	// Initialize parser context
	if err := p.initialize(); err != nil {
		return err
	}

	// Parse directives
	for p.dispenser.Next() {
		if err := p.parseDirective(); err != nil {
			return err
		}
	}

	return nil
}

// initialize sets up the initial state for parsing
func (p *caddyfileParser) initialize() error {
	// Initialize Networks map if needed
	if p.middleware.Networks == nil {
		p.middleware.Networks = make(map[string]*network)
	}

	// Initialize API Key map if needed
	if p.middleware.ApiKeys == nil {
		p.middleware.ApiKeys = make(map[string]string)
	}

	// Set environment
	p.middleware.Env = utils.GetEnv()

	// Initialize basic logger for deprecation warnings during parsing
	if p.middleware.logger == nil {
		p.middleware.logger = logger.NewLoggerClient(zap.NewNop(), p.middleware.Env)
	}

	// Initialize handler registry for validation during parsing
	if p.middleware.handlerRegistry == nil {
		p.middleware.handlerRegistry = networklib.DefaultRegistry
		networklib.RegisterBuiltinHandlers()
	}

	return nil
}

// parseDirective handles top-level directives
func (p *caddyfileParser) parseDirective() error {
	switch p.dispenser.Val() {
	case "port":
		return p.parsePort()
	case "siwe-signer":
		return p.parseSiweSigner()
	case "networks":
		return p.parseNetworks()
	case "din_registry":
		return p.parseDinRegistry()
	case "api_keys":
		return p.parseAPIKeys()
	case "unknown_api_key_salt":
		return p.parseAPISalt()
	case "dynamic_load_balancing":
		return p.parseDynamicLoadBalancing()
	default:
		// Continue processing for other directives
		return nil
	}
}

// parsePort handles port configuration
func (p *caddyfileParser) parsePort() error {
	p.dispenser.Next()
	p.caddyPort = p.dispenser.Val()
	if p.caddyPort == "" {
		p.caddyPort = DefaultPort
	}
	p.middleware.CaddyPort = p.caddyPort
	return nil
}

// parseSiweSigner handles SIWE signer configuration
func (p *caddyfileParser) parseSiweSigner() error {
	var key []byte
	var err error

	for n1 := p.dispenser.Nesting(); p.dispenser.NextBlock(n1); {
		switch p.dispenser.Val() {
		case "secret_file":
			p.dispenser.NextBlock(n1)
			key, err = p.parseSecretKeyFromFile(p.dispenser.Val())
			if err != nil {
				return p.dispenser.Errf("failed to read secret file: %v", err)
			}
		case "secret":
			p.dispenser.NextBlock(n1)
			key, err = p.parseSecretKey(p.dispenser.Val())
			if err != nil {
				return p.dispenser.Errf("error parsing secret: %v", err)
			}
		}
	}

	if len(key) == 0 {
		return p.dispenser.Errf("no key material in siwe-signer definition")
	}

	p.middleware.DefaultSiweSigner = &siwe.SigningConfig{
		PrivateKey: key,
	}

	if err := p.siweSignerClient.GenPrivKey(p.middleware.DefaultSiweSigner); err != nil {
		return err
	}

	return nil
}

// parseNetworks handles the networks block
func (p *caddyfileParser) parseNetworks() error {
	for n1 := p.dispenser.Nesting(); p.dispenser.NextBlock(n1); {
		networkName := p.dispenser.Val()
		if err := p.parseNetwork(networkName, n1); err != nil {
			return err
		}
	}
	return nil
}

// parseNetwork handles individual network configuration
func (p *caddyfileParser) parseNetwork(networkName string, parentNesting int) error {
	// Ensure caddyPort is set
	if p.caddyPort == "" {
		p.caddyPort = DefaultPort
	}

	// Create network if it doesn't exist
	if _, exists := p.middleware.Networks[networkName]; !exists {
		newNetwork, err := NewNetwork(networkName, "", p.middleware.Env, p.caddyPort)
		if err != nil {
			return fmt.Errorf("failed to create network '%s': %w", networkName, err)
		}
		p.middleware.Networks[networkName] = newNetwork
	}

	// Parse network configuration
	for nesting := p.dispenser.Nesting(); p.dispenser.NextBlock(nesting); {
		if err := p.parseNetworkField(networkName, nesting); err != nil {
			return err
		}
	}

	// Validate network configuration
	return p.validateNetwork(networkName)
}

// parseNetworkField handles individual network configuration fields
func (p *caddyfileParser) parseNetworkField(networkName string, nesting int) error {
	network := p.middleware.Networks[networkName]

	switch p.dispenser.Val() {
	case "methods":
		return p.parseNetworkMethods(network)
	case "handler":
		return p.parseNetworkHandler(network)
	case "routed_methods":
		return p.parseRoutedMethods(network)
	case "providers":
		return p.parseProviders(network, nesting)
	case "healthcheck_endpoint":
		return p.parseStringField(&network.HCEndpoint)
	case "chain_id":
		return p.parseChainId(network, networkName)
	case "healthcheck_threshold", "healthcheck_timeout", "healthcheck_interval",
		"healthcheck_blocklag_limit", "healthcheck_blockjump_limit",
		"healthcheck_provider_block_history_size", "network_block_history_size",
		"max_request_payload_size_kb", "request_attempt_count", "archive_enabled":
		return p.parseConfigField(network, p.dispenser.Val())
	case "custom_config":
		return p.parseCustomConfig(network, nesting)
	default:
		return p.dispenser.Errf("unrecognized option: %s", p.dispenser.Val())
	}
}

// parseNetworkMethods parses the methods configuration
func (p *caddyfileParser) parseNetworkMethods(network *network) error {
	network.Methods = make([]*string, p.dispenser.CountRemainingArgs())
	for i := 0; i < p.dispenser.CountRemainingArgs(); i++ {
		network.Methods[i] = new(string)
	}
	if !p.dispenser.Args(network.Methods...) {
		return p.dispenser.Errf("invalid 'methods' argument for network %s", network.Name)
	}
	return nil
}

// parseNetworkHandler parses the handler type
func (p *caddyfileParser) parseNetworkHandler(network *network) error {
	p.dispenser.Next()
	explicitType := p.dispenser.Val()
	network.HandlerType = HandlerType(explicitType)
	if network.CaddyfileFlags != nil {
		network.CaddyfileFlags.HandlerTypeSetInCaddyfile = true
	}
	// Handler creation is deferred to Provision phase for proper logger initialization
	return nil
}

// parseRoutedMethods parses routed methods configuration
func (p *caddyfileParser) parseRoutedMethods(network *network) error {
	methods := make([]*string, p.dispenser.CountRemainingArgs())
	for i := 0; i < p.dispenser.CountRemainingArgs(); i++ {
		methods[i] = new(string)
	}
	if !p.dispenser.Args(methods...) {
		return p.dispenser.Errf("invalid 'routed_methods' argument for network %s", network.Name)
	}

	methodMap := make(map[string]struct{})
	for _, method := range methods {
		methodMap[*method] = struct{}{}
	}
	network.MethodFilter = &methodFilter{
		FilteredMethods: methodMap,
	}
	return nil
}

// parseProviders handles the providers block
func (p *caddyfileParser) parseProviders(network *network, parentNesting int) error {
	for p.dispenser.NextBlock(parentNesting + 1) {
		providerUrl := p.dispenser.Val()
		if err := p.parseProvider(network, providerUrl, parentNesting+1); err != nil {
			return err
		}
	}
	return nil
}

// parseProvider handles individual provider configuration
func (p *caddyfileParser) parseProvider(network *network, providerUrl string, parentNesting int) error {
	providerObj, err := NewProvider(providerUrl)
	if err != nil {
		return fmt.Errorf("error creating provider: %w", err)
	}

	// Parse provider fields
	for p.dispenser.NextBlock(parentNesting + 1) {
		if err := p.parseProviderField(providerObj, parentNesting+1); err != nil {
			return err
		}
	}

	// Parse URL and set host
	parsedUrl, err := url.Parse(providerObj.HttpUrl)
	if err != nil {
		return fmt.Errorf("error parsing provider URL: %w", err)
	}

	// Initialize provider with a unique host
	providerObj.host = p.middleware.ensureUniqueProviderHost(network.Name, parsedUrl, providerObj.Headers)
	network.Providers[providerObj.host] = providerObj

	// Debug logging
	if p.middleware.logger != nil {
		p.middleware.logger.Debug("Added provider to network map",
			zap.String("network", network.Name),
			zap.String("providerHost", providerObj.host),
			zap.String("providerName", providerObj.Name),
			zap.String("providerUrl", providerObj.HttpUrl))
	}

	return nil
}

// parseProviderField handles individual provider configuration fields
func (p *caddyfileParser) parseProviderField(provider *provider, nesting int) error {
	switch p.dispenser.Val() {
	case "methods":
		return p.parseProviderMethods(provider)
	case "auth":
		return p.parseProviderAuth(provider, nesting)
	case "headers":
		return p.parseProviderHeaders(provider, nesting)
	case "priority":
		return p.parseProviderPriority(provider, nesting)
	case "name":
		return p.parseProviderName(provider, nesting)
	default:
		return p.dispenser.Errf("unrecognized provider option: %s", p.dispenser.Val())
	}
}

// parseProviderMethods parses provider methods
func (p *caddyfileParser) parseProviderMethods(provider *provider) error {
	methods := make([]*string, p.dispenser.CountRemainingArgs())
	for i := 0; i < p.dispenser.CountRemainingArgs(); i++ {
		methods[i] = new(string)
	}
	if !p.dispenser.Args(methods...) {
		return p.dispenser.Errf("invalid 'methods' argument for provider %s", provider.HttpUrl)
	}

	provider.Methods = make(map[string]struct{})
	for _, method := range methods {
		provider.Methods[*method] = struct{}{}
	}
	return nil
}

// parseProviderAuth parses authentication configuration (SIWE or OIDC)
func (p *caddyfileParser) parseProviderAuth(provider *provider, parentNesting int) error {
	var authType string
	var siweAuth *siwe.SIWEClientAuth
	var oidcClient *oidc.OIDCClient

	// Default to SIWE for backward compatibility
	siweAuth = p.siweSignerClient.CreateNewSIWEAuth(
		strings.TrimSuffix(provider.HttpUrl, "/")+"/auth", 16)

	// Parse auth configuration
	for p.dispenser.NextBlock(parentNesting + 1) {
		switch p.dispenser.Val() {
		case "type":
			p.dispenser.NextBlock(parentNesting + 1)
			authType = p.dispenser.Val()
			// Initialize based on auth type
			switch authType {
			case "siwe":
				// Already initialized above as default
			case "oidc":
				// Switch to OIDC
				siweAuth = nil
				oidcClient = &oidc.OIDCClient{
					Scope: "openid", // default scope
				}
			default:
				return fmt.Errorf("unknown auth type: %s (supported: siwe, oidc)", authType)
			}
		case "url":
			p.dispenser.NextBlock(parentNesting + 1)
			urlVal := p.dispenser.Val()
			if siweAuth != nil {
				siweAuth.ProviderURL = urlVal
			} else if oidcClient != nil {
				oidcClient.TokenURL = urlVal
			}
		case "client_id":
			if oidcClient == nil {
				return fmt.Errorf("client_id is only valid for OIDC auth type")
			}
			p.dispenser.NextBlock(parentNesting + 1)
			oidcClient.ClientID = p.dispenser.Val()
		case "client_secret":
			if oidcClient == nil {
				return fmt.Errorf("client_secret is only valid for OIDC auth type")
			}
			p.dispenser.NextBlock(parentNesting + 1)
			oidcClient.ClientSecret = p.dispenser.Val()
		case "duration_seconds":
			if oidcClient == nil {
				return fmt.Errorf("duration_seconds is only valid for OIDC auth type")
			}
			p.dispenser.NextBlock(parentNesting + 1)
			duration, err := strconv.Atoi(p.dispenser.Val())
			if err != nil {
				return fmt.Errorf("invalid duration_seconds: %w", err)
			}
			oidcClient.DurationSeconds = duration
		case "sessions":
			if siweAuth == nil {
				return fmt.Errorf("sessions is only valid for SIWE auth type")
			}
			p.dispenser.NextBlock(parentNesting + 1)
			sessionCount, err := strconv.Atoi(p.dispenser.Val())
			if err != nil {
				return fmt.Errorf("invalid session count: %w", err)
			}
			siweAuth.SessionCount = sessionCount
		case "signer":
			if siweAuth == nil {
				return fmt.Errorf("signer is only valid for SIWE auth type")
			}
			signerKey, err := p.parseProviderSigner(parentNesting + 2)
			if err != nil {
				return err
			}
			siweAuth.Signer = &siwe.SigningConfig{
				PrivateKey: signerKey,
			}
			if err := p.siweSignerClient.GenPrivKey(siweAuth.Signer); err != nil {
				return fmt.Errorf("failed to generate private key: %w", err)
			}
		default:
			return p.dispenser.Errf("unrecognized auth option: %s", p.dispenser.Val())
		}
	}

	// Validate and set the appropriate auth
	if siweAuth != nil {
		// Use default signer if not specified
		if siweAuth.Signer == nil {
			if p.middleware.DefaultSiweSigner == nil {
				return p.dispenser.Errf("signer must be set for SIWE auth")
			}
			siweAuth.Signer = p.middleware.DefaultSiweSigner
		}
		provider.Auth = siweAuth
	} else if oidcClient != nil {
		// Validate OIDC configuration
		if oidcClient.ClientID == "" {
			return p.dispenser.Errf("client_id is required for OIDC auth")
		}
		if oidcClient.ClientSecret == "" {
			return p.dispenser.Errf("client_secret is required for OIDC auth")
		}
		if oidcClient.TokenURL == "" {
			return p.dispenser.Errf("url is required for OIDC auth")
		}
		provider.OIDCClient = oidcClient
	}

	return nil
}

// parseProviderSigner parses the signer configuration
func (p *caddyfileParser) parseProviderSigner(parentNesting int) ([]byte, error) {
	var key []byte
	var err error

	for p.dispenser.NextBlock(parentNesting) {
		switch p.dispenser.Val() {
		case "secret_file":
			p.dispenser.NextBlock(parentNesting)
			key, err = p.parseSecretKeyFromFile(p.dispenser.Val())
			if err != nil {
				return nil, p.dispenser.Errf("failed to read secret file: %v", err)
			}
		case "secret":
			p.dispenser.NextBlock(parentNesting)
			key, err = p.parseSecretKey(p.dispenser.Val())
			if err != nil {
				return nil, fmt.Errorf("failed to decode secret: %w", err)
			}
		}
	}

	return key, nil
}

// parseProviderHeaders parses provider headers
func (p *caddyfileParser) parseProviderHeaders(provider *provider, parentNesting int) error {
	for p.dispenser.NextBlock(parentNesting + 1) {
		k := p.dispenser.Val()
		var v string
		if p.dispenser.Args(&v) {
			provider.Headers[k] = v
		} else {
			return p.dispenser.Errf("header should have key and value")
		}
	}
	return nil
}

// parseProviderPriority parses provider priority
func (p *caddyfileParser) parseProviderPriority(provider *provider, nesting int) error {
	p.dispenser.NextBlock(nesting)
	priority, err := strconv.Atoi(p.dispenser.Val())
	if err != nil {
		return fmt.Errorf("invalid priority: %w", err)
	}
	provider.Priority = priority
	return nil
}

// parseProviderName parses provider name
func (p *caddyfileParser) parseProviderName(provider *provider, nesting int) error {
	p.dispenser.NextBlock(nesting)
	provider.Name = p.dispenser.Val()
	return nil
}

// parseChainId parses and validates chain ID
func (p *caddyfileParser) parseChainId(network *network, networkName string) error {
	p.dispenser.Next()
	chainId := p.dispenser.Val()
	if chainId == "" {
		return fmt.Errorf("chain ID cannot be empty for network %s", networkName)
	}
	network.ChainId = chainId
	if network.CaddyfileFlags != nil {
		network.CaddyfileFlags.ChainIdSetInCaddyfile = true
	}
	return nil
}

// parseCustomConfig parses custom configuration block
func (p *caddyfileParser) parseCustomConfig(network *network, parentNesting int) error {
	if network.CustomConfig == nil {
		network.CustomConfig = make(map[string]interface{})
	}

	for p.dispenser.NextBlock(parentNesting + 1) {
		key := p.dispenser.Val()
		if !p.dispenser.Next() {
			return p.dispenser.Errf("custom_config key '%s' has no value", key)
		}
		value := p.dispenser.Val()

		// Try to parse as int first, then bool, then keep as string
		if intVal, err := strconv.Atoi(value); err == nil {
			network.CustomConfig[key] = intVal
		} else if boolVal, err := strconv.ParseBool(value); err == nil {
			network.CustomConfig[key] = boolVal
		} else {
			network.CustomConfig[key] = value
		}
	}

	return nil
}

// parseDinRegistry handles din_registry configuration
func (p *caddyfileParser) parseDinRegistry() error {
	for n1 := p.dispenser.Nesting(); p.dispenser.NextBlock(n1); {
		switch p.dispenser.Val() {
		case "registry_enabled":
			if err := p.parseBoolField(&p.middleware.Registry.Enabled, "registry enabled"); err != nil {
				return err
			}
		case "registry_block_epoch":
			if err := p.parseUint64Field(&p.middleware.Registry.BlockEpoch, "registry block epoch"); err != nil {
				return err
			}
		case "registry_block_check_interval_sec":
			if err := p.parseUint64Field(&p.middleware.Registry.BlockCheckIntervalSec, "registry block check interval"); err != nil {
				return err
			}
		case "registry_endpoint_url":
			if err := p.parseStringField(&p.middleware.Registry.EndpointUrl); err != nil {
				return err
			}
		case "registry_contract_address":
			if err := p.parseStringField(&p.middleware.Registry.ContractAddress); err != nil {
				return err
			}
		case "registry_priority":
			if err := p.parseIntField(&p.middleware.Registry.Priority, "registry priority"); err != nil {
				return err
			}
		case "registry_client_type":
			if err := p.parseStringField(&p.middleware.Registry.ClientType); err != nil {
				return err
			}
		default:
			return p.dispenser.Errf("unrecognized registry option: %s", p.dispenser.Val())
		}
	}
	return nil
}

func (p *caddyfileParser) parseDynamicLoadBalancing() error {
	for n1 := p.dispenser.Nesting(); p.dispenser.NextBlock(n1); {
		switch p.dispenser.Val() {
		case "enabled":
			p.dispenser.Next()
			smartRoutingEnabledVal := p.dispenser.Val()
			// Convert string to bool
			boolValue, err := strconv.ParseBool(smartRoutingEnabledVal)
			if err != nil {
				return p.dispenser.Errf("Error while parsing dynamic_load_balancing.enabled: %v", err)
			}
			p.middleware.DynamicLoadBalancing.Enabled = boolValue
		case "watcher_endpoint":
			p.dispenser.Next()
			p.middleware.DynamicLoadBalancing.WatcherApiEndpoint = p.dispenser.Val()
		case "watcher_api_key":
			p.dispenser.Next()
			p.middleware.DynamicLoadBalancing.WatcherApiKey = p.dispenser.Val()
		case "sync_score_interval_secs":
			p.dispenser.Next()
			syncScoreIntervalSecs := p.dispenser.Val()
			uint64Value, err := strconv.ParseUint(syncScoreIntervalSecs, 10, 64)
			if err != nil {
				return p.dispenser.Errf("Error parsing dynamic_load_balancing.sync_score_interval_secs: %v", err)
			}
			p.middleware.DynamicLoadBalancing.WatcherScoreSyncIntervalSec = uint64Value
		default:
			return p.dispenser.Errf("unrecognized option while parsing dynamic_load_balancing directive: %s", p.dispenser.Val())
		}
	}

	// Validate dynamic load balancing configuration after all values are set
	return p.validateDynamicLoadBalancing()
}

func (p *caddyfileParser) validateDynamicLoadBalancing() error {
	if p.middleware.DynamicLoadBalancing.Enabled {
		if p.middleware.DynamicLoadBalancing.WatcherApiEndpoint == "" {
			return p.dispenser.Errf("watcher_endpoint is required when dynamic_load_balancing.enabled is true")
		}
		if p.middleware.DynamicLoadBalancing.WatcherApiKey == "" {
			return p.dispenser.Errf("watcher_api_key is required when dynamic_load_balancing.enabled is true")
		}
	}

	return nil
}

func (p *caddyfileParser) parseAPIKeys() error {
	for n1 := p.dispenser.Nesting(); p.dispenser.NextBlock(n1); {
		key := p.dispenser.Val()
		if !p.dispenser.Next() {
			return p.dispenser.Errf("API keys must specify user IDs")
		}
		p.middleware.ApiKeys[key] = p.dispenser.Val()
	}
	return nil
}

func (p *caddyfileParser) parseAPISalt() error {
	if !p.dispenser.Next() {
		return p.dispenser.Errf("unknown_api_key_salt must be followed with a value")
	}
	p.middleware.ApiSalt = p.dispenser.Val()
	return nil
}

// validateNetwork validates network configuration after parsing
func (p *caddyfileParser) validateNetwork(networkName string) error {
	network := p.middleware.Networks[networkName]

	// Validate chain ID is set
	if network.ChainId == "" {
		return fmt.Errorf("chain ID is not set for network %s", networkName)
	}

	// Set default handler type if not specified
	if network.HandlerType == "" {
		p.middleware.logger.Info("No explicit handler type specified, defaulting to EVM",
			zap.String("network", networkName))
		network.HandlerType = EVMHandler
	}

	return nil
}

// Helper functions for parsing common field types

// configFieldMapping maps Caddyfile directive names to their corresponding struct field names and flag field names
var configFieldMapping = map[string]struct {
	fieldName string
	flagName  string
}{
	"healthcheck_threshold":                   {"HCThreshold", "HCThresholdSetInCaddyfile"},
	"healthcheck_timeout":                     {"HCTimeout", "HCTimeoutSetInCaddyfile"},
	"healthcheck_interval":                    {"HCInterval", "HCIntervalSetInCaddyfile"},
	"healthcheck_blocklag_limit":              {"BlockLagLimit", "BlockLagLimitSetInCaddyfile"},
	"healthcheck_blockjump_limit":             {"BlockJumpLimit", "BlockJumpLimitSetInCaddyfile"},
	"healthcheck_provider_block_history_size": {"ProviderBlockHistorySize", "ProviderBlockHistorySizeSetInCaddyfile"},
	"network_block_history_size":              {"NetworkBlockHistorySize", "NetworkBlockHistorySizeSetInCaddyfile"},
	"max_request_payload_size_kb":             {"MaxRequestPayloadSizeKB", "MaxRequestPayloadSizeKBSetInCaddyfile"},
	"request_attempt_count":                   {"RequestAttemptCount", "RequestAttemptCountSetInCaddyfile"},
	"archive_enabled":                         {"ArchiveEnabled", "ArchiveEnabledSetInCaddyfile"},
}

// parseConfigField uses reflection to parse a configuration field and automatically set the corresponding flag
func (p *caddyfileParser) parseConfigField(network *network, directive string) error {
	mapping, exists := configFieldMapping[directive]
	if !exists {
		return fmt.Errorf("unknown configuration directive: %s", directive)
	}

	// Get the next value from the dispenser
	p.dispenser.Next()
	value := p.dispenser.Val()

	// Use reflection to access the network struct fields
	networkValue := reflect.ValueOf(network).Elem()
	fieldValue := networkValue.FieldByName(mapping.fieldName)

	if !fieldValue.IsValid() {
		return fmt.Errorf("field %s not found in network struct", mapping.fieldName)
	}

	// Parse and set the value based on the field type
	switch fieldValue.Kind() {
	case reflect.Int:
		val, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid %s: %w", directive, err)
		}
		fieldValue.SetInt(int64(val))

	case reflect.Int64:
		val, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid %s: %w", directive, err)
		}
		fieldValue.SetInt(val)

	case reflect.Bool:
		val, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid %s: %w", directive, err)
		}
		fieldValue.SetBool(val)

	default:
		return fmt.Errorf("unsupported field type %v for %s", fieldValue.Kind(), mapping.fieldName)
	}

	// Set the corresponding flag field if CaddyfileFlags exists
	if network.CaddyfileFlags != nil {
		flagsValue := reflect.ValueOf(network.CaddyfileFlags).Elem()
		flagField := flagsValue.FieldByName(mapping.flagName)
		if flagField.IsValid() && flagField.Kind() == reflect.Bool {
			flagField.SetBool(true)
		}
	}

	return nil
}

func (p *caddyfileParser) parseStringField(field *string) error {
	p.dispenser.Next()
	*field = p.dispenser.Val()
	return nil
}

func (p *caddyfileParser) parseIntField(field *int, fieldName string) error {
	p.dispenser.Next()
	val, err := strconv.Atoi(p.dispenser.Val())
	if err != nil {
		return fmt.Errorf("invalid %s: %w", fieldName, err)
	}
	*field = val
	return nil
}

func (p *caddyfileParser) parseIntFieldToInt(field *int, fieldName string) error {
	p.dispenser.Next()
	val, err := strconv.Atoi(p.dispenser.Val())
	if err != nil {
		return fmt.Errorf("invalid %s: %w", fieldName, err)
	}
	*field = val
	return nil
}

func (p *caddyfileParser) parseInt64Field(field *int64, fieldName string) error {
	p.dispenser.Next()
	val, err := strconv.Atoi(p.dispenser.Val())
	if err != nil {
		return fmt.Errorf("invalid %s: %w", fieldName, err)
	}
	*field = int64(val)
	return nil
}

func (p *caddyfileParser) parseUint64Field(field *uint64, fieldName string) error {
	p.dispenser.Next()
	val, err := strconv.ParseUint(p.dispenser.Val(), 10, 64)
	if err != nil {
		return p.dispenser.Errf("Error converting string to int: %v", err)
	}
	*field = val

	return nil
}

func (p *caddyfileParser) parseBoolField(field *bool, fieldName string) error {
	p.dispenser.Next()
	val, err := strconv.ParseBool(p.dispenser.Val())
	if err != nil {
		return fmt.Errorf("invalid %s: %w", fieldName, err)
	}
	*field = val
	return nil
}

func (p *caddyfileParser) parseSecretKey(hexKey string) ([]byte, error) {
	hexKey = strings.TrimSpace(strings.TrimPrefix(hexKey, "0x"))
	return hex.DecodeString(hexKey)
}

func (p *caddyfileParser) parseSecretKeyFromFile(filename string) ([]byte, error) {

	hexKeyBytes, err := os.ReadFile(filepath.Clean(filename))
	if err != nil {
		return nil, err
	}
	hexKey := string(hexKeyBytes)
	return p.parseSecretKey(hexKey)
}
