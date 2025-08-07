package dincli

import (
	"encoding/json"
	"os"

	"github.com/DIN-center/din-sc/apps/din-go/lib/din"
)

// JSONFormatter implements OutputFormatter for JSON output
type JSONFormatter struct{}

// NewJSONFormatter creates a new JSON formatter
func NewJSONFormatter() *JSONFormatter {
	return &JSONFormatter{}
}

func (j *JSONFormatter) FormatNetworks(networks []*din.Network, options FormatOptions) error {
	return j.outputJSON(networks)
}

func (j *JSONFormatter) FormatNetwork(network *din.Network, options FormatOptions) error {
	return j.outputJSON(network)
}

func (j *JSONFormatter) FormatProviders(providers []*din.Provider, options FormatOptions) error {
	return j.outputJSON(providers)
}

func (j *JSONFormatter) FormatProvider(provider *din.Provider, options FormatOptions) error {
	return j.outputJSON(provider)
}

// Helper methods for JSONFormatter
func (j *JSONFormatter) outputJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}
