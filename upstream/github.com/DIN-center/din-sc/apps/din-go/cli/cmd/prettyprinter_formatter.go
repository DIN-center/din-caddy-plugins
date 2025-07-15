package dincli

import (
	"fmt"
	"os"
	"strings"

	"github.com/DIN-center/din-sc/apps/din-go/lib/din"
	"github.com/jedib0t/go-pretty/v6/list"
	"github.com/jedib0t/go-pretty/v6/table"
)

// PrettyPrinterFormatter implements OutputFormatter for human-readable console output
type PrettyPrinterFormatter struct{}

// NewPrettyPrinterFormatter creates a new human-friendly formatter
func NewPrettyPrinterFormatter() *PrettyPrinterFormatter {
	return &PrettyPrinterFormatter{}
}

func (h *PrettyPrinterFormatter) FormatNetworks(networks []*din.Network, options FormatOptions) error {
	if options.Verbose {
		return h.formatNetworksFull(networks, options)
	}
	return h.formatNetworksCompact(networks)
}

func (h *PrettyPrinterFormatter) FormatNetwork(network *din.Network, options FormatOptions) error {
	return h.formatNetworkFull(network, options)
}

func (h *PrettyPrinterFormatter) FormatProviders(providers []*din.Provider, options FormatOptions) error {
	if options.Verbose {
		return h.formatProvidersFull(providers)
	}
	return h.formatProvidersCompact(providers)
}

func (h *PrettyPrinterFormatter) FormatProvider(provider *din.Provider, options FormatOptions) error {
	return h.formatProviderFull(provider)
}

// Helper methods for PrettyPrinterFormatter
func (h *PrettyPrinterFormatter) formatNetworksCompact(networks []*din.Network) error {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{
		"#", "Network URI", "Contract Address", "ProxyName", "Status", "Archive?", "ChainId", "Providers",
	})

	for i, network := range networks {
		t.AppendRow(table.Row{
			i,
			network.Name,
			strings.ToLower(network.Address),
			network.ProxyName,
			network.Status,
			func() string {
				if network.NetworkConfig != nil && network.NetworkConfig.ArchiveEnabled {
					return "true"
				}
				return "false"
			}(),
			func() string {
				if network.NetworkConfig != nil {
					chainId := network.NetworkConfig.ChainId
					if len(chainId) > 14 {
						return chainId[:11] + "..."
					}
					return chainId
				}
				return "N/A"
			}(),
			len(network.Providers),
		})
	}
	t.SetStyle(table.StyleLight)
	t.Render()
	return nil
}

func (h *PrettyPrinterFormatter) formatNetworksFull(networks []*din.Network, options FormatOptions) error {
	fmt.Printf("# of Networks in Registry: %d\n", len(networks))
	for i, network := range networks {
		fmt.Printf("Index: %d\n", i)
		fmt.Println(strings.Repeat("-", 30))
		if err := h.formatNetworkFull(network, options); err != nil {
			return err
		}
	}
	return nil
}

func (h *PrettyPrinterFormatter) formatNetworkFull(network *din.Network, options FormatOptions) error {
	l := list.NewWriter()
	l.SetOutputMirror(os.Stdout)
	l.SetStyle(list.StyleConnectedRounded)

	l.AppendItem("Name (URI): " + network.Name)
	l.Indent()
	l.AppendItem("Description: " + network.Description)
	l.AppendItem("Status: " + network.Status)
	l.AppendItem("Contract Address: " + strings.ToLower(network.Address))
	l.AppendItem("Owner Address: " + strings.ToLower(network.Owner))
	l.AppendItem("ProxyName: " + network.ProxyName)
	l.AppendItem("Capabilities Bitmask: " + network.Capabilities.String())

	if network.NetworkConfig != nil {
		l.UnIndent()
		l.AppendItem("Network Configuration:")
		l.Indent()
		l.AppendItem("ChainId: " + network.NetworkConfig.ChainId)
		l.AppendItem("ArchiveEnabled: " + fmt.Sprintf("%t", network.NetworkConfig.ArchiveEnabled))
		l.AppendItem("HealthcheckMethod: " + network.NetworkConfig.HealthcheckMethod)
		l.AppendItem("HealthcheckIntervalSec: " + fmt.Sprintf("%d", network.NetworkConfig.HealthcheckIntervalSec))
		l.AppendItem("GetBlockByNumberMethod: " + network.NetworkConfig.GetBlockByNumberMethod)
		l.AppendItem("CallContractMethod: " + network.NetworkConfig.CallContractMethod)
		l.AppendItem("ChainIdMethod: " + network.NetworkConfig.ChainIdMethod)
		l.AppendItem("BlockJumpLimit: " + fmt.Sprintf("%d", network.NetworkConfig.BlockJumpLimit))
		l.AppendItem("BlockLagLimit: " + fmt.Sprintf("%d", network.NetworkConfig.BlockLagLimit))
		l.AppendItem("RequestAttemptCount: " + fmt.Sprintf("%d", network.NetworkConfig.RequestAttemptCount))
		l.AppendItem("MaxRequestPayloadSizeKb: " + fmt.Sprintf("%d", network.NetworkConfig.MaxRequestPayloadSizeKb))
		l.AppendItem("RegistryBlockEpoch: " + fmt.Sprintf("%d", network.NetworkConfig.RegistryBlockEpoch))
		l.UnIndent()
	}

	if options.ShowMethods && len(network.MethodsByName) > 0 {
		l.AppendItem("Methods:")
		l.Indent()
		methodsByNamespace := make(map[string][]string)
		for _, method := range network.MethodsByName {
			namespace := strings.Split(method.Name, "_")[0]
			if _, ok := methodsByNamespace[namespace]; !ok {
				methodsByNamespace[namespace] = []string{}
			}
			methodsByNamespace[namespace] = append(methodsByNamespace[namespace], method.Name)
		}

		for namespace, methods := range methodsByNamespace {
			l.AppendItem(namespace)
			l.Indent()
			for _, method := range methods {
				l.AppendItem(method)
			}
			l.UnIndent()
		}
		l.UnIndent()
	}

	if options.ShowProviders {
		if len(network.Providers) > 0 {
			l.AppendItem("Providers:")
			l.Indent()
			providers := make([]*din.Provider, 0, len(network.Providers))
			for _, provider := range network.Providers {
				providers = append(providers, provider)
			}
			h.fillListWithProviders(providers, l)
		} else {
			l.AppendItem("No providers registered for this network")
		}
	}

	l.Render()
	return nil
}

func (h *PrettyPrinterFormatter) formatProvidersCompact(providers []*din.Provider) error {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{
		"#", "Provider Name", "Contract Address", "Status", "Auth Type", "Use Header?", "Owner", "Networks", "Services", "Locations Covered",
	})

	for i, provider := range providers {
		networksCovered := make(map[string]bool)
		locationsCovered := make(map[string]bool)
		for _, networkService := range provider.NetworkServices {
			networksCovered[networkService.NetworkAddress] = true
			for _, location := range networkService.Locations {
				locationsCovered[location.String()] = true
			}
		}
		locationsCoveredList := []string{}
		for location := range locationsCovered {
			locationsCoveredList = append(locationsCoveredList, location)
		}
		t.AppendRow(table.Row{
			i,
			provider.Name,
			strings.ToLower(provider.Address),
			provider.Status,
			provider.AuthConfig.Type.String(),
			provider.AuthConfig.UseHeader,
			provider.Owner,
			len(networksCovered),
			len(provider.NetworkServices),
			strings.Join(locationsCoveredList, ", "),
		})
	}
	t.SetStyle(table.StyleLight)
	t.Render()
	return nil
}

func (h *PrettyPrinterFormatter) formatProvidersFull(providers []*din.Provider) error {
	l := list.NewWriter()
	l.SetOutputMirror(os.Stdout)
	l.SetStyle(list.StyleConnectedRounded)
	h.fillListWithProviders(providers, l)
	l.Render()
	return nil
}

func (h *PrettyPrinterFormatter) formatProviderFull(provider *din.Provider) error {
	l := list.NewWriter()
	l.SetOutputMirror(os.Stdout)
	l.SetStyle(list.StyleConnectedRounded)
	h.fillListWithProviders([]*din.Provider{provider}, l)
	l.Render()
	return nil
}

func (h *PrettyPrinterFormatter) fillListWithProviders(providers []*din.Provider, l list.Writer) {
	for _, provider := range providers {
		l.AppendItem("Name: " + provider.Name)
		l.Indent()
		l.AppendItem("Status: " + provider.Status)
		l.AppendItem("Contract Address: " + strings.ToLower(provider.Address))
		l.AppendItem("Owner Address: " + strings.ToLower(provider.Owner))
		l.AppendItem("Auth Type: " + provider.AuthConfig.Type)
		l.AppendItem("Auth URL: " + provider.AuthConfig.Url)
		l.AppendItem("API Key Placeholder: " + provider.AuthConfig.ApiKeyPlaceholder)
		l.AppendItem("Use Header?: " + fmt.Sprintf("%t", provider.AuthConfig.UseHeader))

		if len(provider.NetworkServices) > 0 {
			l.AppendItem("Endpoints:")

			for _, networkService := range provider.NetworkServices {
				l.Indent()
				l.AppendItem("Network Name: " + networkService.NetworkName)
				l.Indent()
				l.AppendItem("URL: " + networkService.Url)
				l.AppendItem("Status: " + networkService.Status)
				l.AppendItem("Contract Address: " + networkService.Address)
				locations := make([]string, len(networkService.Locations))
				for i, location := range networkService.Locations {
					locations[i] = location.String()
				}
				l.AppendItem("Locations: " + strings.Join(locations, ", "))
				l.AppendItem("Capabilities Bitmask: " + networkService.Capabilities.String())
				l.UnIndent()
				l.UnIndent()
			}
		}
		l.UnIndent()
	}
}
