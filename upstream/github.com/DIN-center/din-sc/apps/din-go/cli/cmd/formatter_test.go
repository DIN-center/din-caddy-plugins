package dincli

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/DIN-center/din-sc/apps/din-go/lib/din"
	"github.com/stretchr/testify/assert"
)

func TestJSONFormatter(t *testing.T) {
	t.Run("JSON formatter creation", func(t *testing.T) {
		formatter := NewJSONFormatter()
		assert.NotNil(t, formatter)
		assert.IsType(t, &JSONFormatter{}, formatter)
	})

	t.Run("JSON formatter format networks", func(t *testing.T) {
		formatter := NewJSONFormatter()

		// Create test networks
		networks := []*din.Network{
			{
				Name:    "TestNetwork1",
				Address: "0x1234567890123456789012345678901234567890",
			},
			{
				Name:    "TestNetwork2",
				Address: "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
			},
		}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		defer func() {
			os.Stdout = oldStdout
		}()

		// Format networks
		err := formatter.FormatNetworks(networks, FormatOptions{})

		w.Close()
		output, _ := io.ReadAll(r)

		assert.NoError(t, err)
		assert.NotEmpty(t, output)

		// Verify it's valid JSON
		var result []din.Network
		err = json.Unmarshal(output, &result)
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "TestNetwork1", result[0].Name)
		assert.Equal(t, "TestNetwork2", result[1].Name)
	})

	t.Run("JSON formatter format single network", func(t *testing.T) {
		formatter := NewJSONFormatter()

		// Create test network
		network := &din.Network{
			Name:    "TestNetwork",
			Address: "0x1234567890123456789012345678901234567890",
		}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		defer func() {
			os.Stdout = oldStdout
		}()

		// Format network
		err := formatter.FormatNetwork(network, FormatOptions{})

		w.Close()
		output, _ := io.ReadAll(r)

		assert.NoError(t, err)
		assert.NotEmpty(t, output)

		// Verify it's valid JSON
		var result din.Network
		err = json.Unmarshal(output, &result)
		assert.NoError(t, err)
		assert.Equal(t, "TestNetwork", result.Name)
		assert.Equal(t, "0x1234567890123456789012345678901234567890", result.Address)
	})

	t.Run("JSON formatter format providers", func(t *testing.T) {
		formatter := NewJSONFormatter()

		// Create test providers
		providers := []*din.Provider{
			{
				Name: "TestProvider1",
			},
			{
				Name: "TestProvider2",
			},
		}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		defer func() {
			os.Stdout = oldStdout
		}()

		// Format providers
		err := formatter.FormatProviders(providers, FormatOptions{})

		w.Close()
		output, _ := io.ReadAll(r)

		assert.NoError(t, err)
		assert.NotEmpty(t, output)

		// Verify it's valid JSON
		var result []din.Provider
		err = json.Unmarshal(output, &result)
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "TestProvider1", result[0].Name)
		assert.Equal(t, "TestProvider2", result[1].Name)
	})

	t.Run("JSON formatter format single provider", func(t *testing.T) {
		formatter := NewJSONFormatter()

		// Create test provider
		provider := &din.Provider{
			Name: "TestProvider",
		}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		defer func() {
			os.Stdout = oldStdout
		}()

		// Format provider
		err := formatter.FormatProvider(provider, FormatOptions{})

		w.Close()
		output, _ := io.ReadAll(r)

		assert.NoError(t, err)
		assert.NotEmpty(t, output)

		// Verify it's valid JSON
		var result din.Provider
		err = json.Unmarshal(output, &result)
		assert.NoError(t, err)
		assert.Equal(t, "TestProvider", result.Name)
	})

	t.Run("JSON formatter ignores format options", func(t *testing.T) {
		formatter := NewJSONFormatter()

		// Create test network
		network := &din.Network{
			Name:    "TestNetwork",
			Address: "0x1234567890123456789012345678901234567890",
			MethodsByName: map[string]*din.Method{
				"TestMethod": {
					Name: "TestMethod",
				},
			},
			Providers: map[string]*din.Provider{
				"TestProvider": {
					Name: "TestProvider",
					NetworkServices: map[string]*din.NetworkService{
						"TestNetworkService": {
							Url: "https://test.com",
						},
					},
				},
			},
		}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		defer func() {
			os.Stdout = oldStdout
		}()

		// Format with different options - should produce same output
		options1 := FormatOptions{ShowMethods: true, ShowProviders: true, Verbose: true}
		err1 := formatter.FormatNetwork(network, options1)

		w.Close()
		output1, _ := io.ReadAll(r)

		// Reset pipe for second test
		r, w, _ = os.Pipe()
		os.Stdout = w

		options2 := FormatOptions{ShowMethods: false, ShowProviders: false, Verbose: false}
		err2 := formatter.FormatNetwork(network, options2)

		w.Close()
		output2, _ := io.ReadAll(r)

		assert.NoError(t, err1)
		assert.NoError(t, err2)
		assert.Equal(t, output1, output2, "JSON formatter should ignore format options")
	})

	t.Run("JSON formatter marshals maps as lists with correct field names", func(t *testing.T) {
		formatter := NewJSONFormatter()

		// Create test network with methods and providers
		network := &din.Network{
			Name:    "TestNetwork",
			Address: "0x1234567890123456789012345678901234567890",
			MethodsByName: map[string]*din.Method{
				"method1": {
					Name: "method1",
				},
				"method2": {
					Name: "method2",
				},
			},
			Providers: map[string]*din.Provider{
				"provider1": {
					Name:    "provider1",
					Address: "0xprovider1",
					NetworkServices: map[string]*din.NetworkService{
						"service1": {
							Address: "0xservice1",
							Url:     "https://service1.com",
						},
					},
				},
			},
		}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		defer func() {
			os.Stdout = oldStdout
		}()

		// Format network
		err := formatter.FormatNetwork(network, FormatOptions{})

		w.Close()
		output, _ := io.ReadAll(r)

		assert.NoError(t, err)
		assert.NotEmpty(t, output)

		// Parse JSON to verify structure
		var result map[string]interface{}
		err = json.Unmarshal(output, &result)
		assert.NoError(t, err)

		// Verify methods are marshaled as a list with correct field name
		methods, exists := result["methods"]
		assert.True(t, exists, "methods field should exist")
		methodsList, ok := methods.([]interface{})
		assert.True(t, ok, "methods should be a list")
		assert.Len(t, methodsList, 2, "should have 2 methods")

		// Verify providers are marshaled as a list with correct field name
		providers, exists := result["providers"]
		assert.True(t, exists, "providers field should exist")
		providersList, ok := providers.([]interface{})
		assert.True(t, ok, "providers should be a list")
		assert.Len(t, providersList, 1, "should have 1 provider")

		// Verify provider's network services are marshaled as a list
		provider := providersList[0].(map[string]interface{})
		services, exists := provider["services"]
		assert.True(t, exists, "services field should exist in provider")
		servicesList, ok := services.([]interface{})
		assert.True(t, ok, "services should be a list")
		assert.Len(t, servicesList, 1, "should have 1 service")
	})

}

func TestPrettyPrinterFormatter(t *testing.T) {
	t.Run("PrettyPrinter formatter creation", func(t *testing.T) {
		formatter := NewPrettyPrinterFormatter()
		assert.NotNil(t, formatter)
		assert.IsType(t, &PrettyPrinterFormatter{}, formatter)
	})

	t.Run("PrettyPrinter formatter format networks", func(t *testing.T) {
		formatter := NewPrettyPrinterFormatter()

		// Create test networks
		networks := []*din.Network{
			{
				Name:    "TestNetwork1",
				Address: "0x1234567890123456789012345678901234567890",
			},
			{
				Name:    "TestNetwork2",
				Address: "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
			},
		}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		defer func() {
			os.Stdout = oldStdout
		}()

		// Format networks
		err := formatter.FormatNetworks(networks, FormatOptions{})

		w.Close()
		output, _ := io.ReadAll(r)

		assert.NoError(t, err)
		assert.NotEmpty(t, output)

		// Verify it contains network information
		outputStr := string(output)
		assert.Contains(t, outputStr, "TestNetwork1")
		assert.Contains(t, outputStr, "TestNetwork2")
		assert.Contains(t, outputStr, "0x1234567890123456789012345678901234567890")
		assert.Contains(t, outputStr, "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd")
	})

	t.Run("PrettyPrinter formatter format single network", func(t *testing.T) {
		formatter := NewPrettyPrinterFormatter()

		// Create test network
		network := &din.Network{
			Name:    "TestNetwork",
			Address: "0x1234567890123456789012345678901234567890",
			MethodsByName: map[string]*din.Method{
				"TestMethod": {
					Name: "TestMethod",
				},
			},
			Providers: map[string]*din.Provider{
				"TestProvider": {
					Name: "TestProvider",
					NetworkServices: map[string]*din.NetworkService{
						"TestNetworkService": {
							Url: "https://test.com",
						},
					},
				},
			},
		}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		defer func() {
			os.Stdout = oldStdout
		}()

		// Format network
		err := formatter.FormatNetwork(network, FormatOptions{})

		w.Close()
		output, _ := io.ReadAll(r)

		assert.NoError(t, err)
		assert.NotEmpty(t, output)

		// Verify it contains network information
		outputStr := string(output)
		assert.Contains(t, outputStr, "TestNetwork")
		assert.Contains(t, outputStr, "0x1234567890123456789012345678901234567890")
		assert.NotContains(t, outputStr, "TestMethod")
		assert.NotContains(t, outputStr, "TestProvider")
		assert.NotContains(t, outputStr, "TestNetworkService")
		assert.NotContains(t, outputStr, "https://test.com")
	})

	t.Run("PrettyPrinter formatter format providers", func(t *testing.T) {
		formatter := NewPrettyPrinterFormatter()

		// Create test providers
		providers := []*din.Provider{
			{
				Name: "TestProvider1",
				AuthConfig: &din.ProviderAuthConfig{
					Type: din.ProviderAuthType("test1"),
				},
			},
			{
				Name: "TestProvider2",
				AuthConfig: &din.ProviderAuthConfig{
					Type: din.ProviderAuthType("test2"),
				},
			},
		}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		defer func() {
			os.Stdout = oldStdout
		}()

		// Format providers
		err := formatter.FormatProviders(providers, FormatOptions{})

		w.Close()
		output, _ := io.ReadAll(r)

		assert.NoError(t, err)
		assert.NotEmpty(t, output)

		// Verify it contains provider information
		outputStr := string(output)
		assert.Contains(t, outputStr, "TestProvider1")
		assert.Contains(t, outputStr, "TestProvider2")
		assert.Contains(t, outputStr, "test1")
		assert.Contains(t, outputStr, "test2")
	})

	t.Run("PrettyPrinter formatter format single provider", func(t *testing.T) {
		formatter := NewPrettyPrinterFormatter()

		// Create test provider
		provider := &din.Provider{
			Name: "TestProvider",
			AuthConfig: &din.ProviderAuthConfig{
				Type: din.ProviderAuthType("test"),
			},
		}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		defer func() {
			os.Stdout = oldStdout
		}()

		// Format provider
		err := formatter.FormatProvider(provider, FormatOptions{})

		w.Close()
		output, _ := io.ReadAll(r)

		assert.NoError(t, err)
		assert.NotEmpty(t, output)

		// Verify it contains provider information
		outputStr := string(output)
		assert.Contains(t, outputStr, "TestProvider")
	})

	t.Run("PrettyPrinter formatter with verbose options", func(t *testing.T) {
		formatter := NewPrettyPrinterFormatter()

		// Create test network
		network := &din.Network{
			Name:    "TestNetwork",
			Address: "0x1234567890123456789012345678901234567890",
			MethodsByName: map[string]*din.Method{
				"TestMethod": {
					Name: "TestMethod",
				},
			},
			Providers: map[string]*din.Provider{
				"TestProvider": {
					Name: "TestProvider",
					AuthConfig: &din.ProviderAuthConfig{
						Type: din.ProviderAuthType("AuthTypeTest"),
					},
					NetworkServices: map[string]*din.NetworkService{
						"TestNetworkService": {
							Url: "https://test.com",
						},
					},
				},
			},
		}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		defer func() {
			os.Stdout = oldStdout
		}()

		// Format with verbose options
		options := FormatOptions{ShowMethods: true, ShowProviders: true, Verbose: true}
		err := formatter.FormatNetwork(network, options)

		w.Close()
		output, _ := io.ReadAll(r)

		assert.NoError(t, err)
		assert.NotEmpty(t, output)

		// Verify it contains network information
		outputStr := string(output)
		assert.Contains(t, outputStr, "TestNetwork")
		assert.Contains(t, outputStr, "0x1234567890123456789012345678901234567890")
		assert.Contains(t, outputStr, "TestMethod")
		assert.Contains(t, outputStr, "TestProvider")
		assert.Contains(t, outputStr, "https://test.com")
		assert.Contains(t, outputStr, "AuthTypeTest")
	})
}
