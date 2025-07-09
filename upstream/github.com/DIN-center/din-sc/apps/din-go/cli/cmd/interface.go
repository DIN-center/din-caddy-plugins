package dincli

import "github.com/DIN-center/din-sc/apps/din-go/lib/din"

// IOutputFormatter defines the interface for formatting and displaying DIN registry data
// in different output formats (human-friendly, JSON, etc.)
type IOutputFormatter interface {
	// FormatNetworks formats a collection of networks for output
	FormatNetworks(networks []*din.Network, options FormatOptions) error

	// FormatNetwork formats a single network for output
	FormatNetwork(network *din.Network, options FormatOptions) error

	// FormatProviders formats a collection of providers for output
	FormatProviders(providers []*din.Provider, options FormatOptions) error

	// FormatProvider formats a single provider for output
	FormatProvider(provider *din.Provider, options FormatOptions) error
}

// FormatOptions defines configuration options for output formatting
type FormatOptions struct {
	// whether to include method details in verbose output
	ShowMethods bool

	// whether to include provider details in verbose output
	ShowProviders bool

	// Defines if the output should be verbose or not
	Verbose bool
}
