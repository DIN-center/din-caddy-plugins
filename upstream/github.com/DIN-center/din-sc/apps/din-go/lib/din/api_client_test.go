package din

import (
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestApiDingoClientGetRegistryData(t *testing.T) {
	responseJSON := `{
		"ok": true,
		"count": 1,
		"skip": 0,
		"limit": 10,
		"data": [
			{
				"service_id": "ethereum://mainnet",
				"name": "eth",
				"status": "Active",
				"route": "",
				"config": {
					"handler": "ethereum",
					"health_check_interval_sec": 10,
					"health_check_threshold": 2,
					"health_check_timeout": 5,
					"block_lag_limit": 3,
					"block_jump_limit": 4,
					"request_attempt_count": 2,
					"max_request_payload_size_kb": 1024,
					"registry_block_epoch": 20,
					"archive_enabled": true,
					"provider_block_history_size": 5,
					"network_block_history_size": 6,
					"chain_id": "0x1"
				},
				"paths": [
					{
						"name": "eth_blockNumber",
						"bit": 1,
						"deactivated": false
					}
				],
				"providers": [
					{
						"provider_id": "provider-1",
						"name": "Provider One",
						"status": "Active",
						"auth_config": {
							"type": "ApiKey",
							"url": "https://auth.example.com"
						},
						"endpoints": [
							{
								"url": "https://endpoint.example.com",
								"status": "Active"
							}
						]
					}
				]
			}
		]
	}`
	var response apiServicesResponse
	if err := json.Unmarshal([]byte(responseJSON), &response); err != nil {
		t.Fatalf("failed to unmarshal response json: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/services" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	var client IDinReader

	client = NewApiDingoClient(server.URL, server.Client())
	registryData, err := client.GetRegistryData()
	if err != nil {
		t.Fatalf("GetRegistryData error: %v", err)
	}

	network, ok := registryData.Networks["eth"]
	if !ok {
		t.Fatalf("expected network to be present")
	}

	if network.Status != NetworkStatusActive {
		t.Fatalf("expected network status active, got %s", network.Status)
	}
	if network.ProxyName != "eth" {
		t.Fatalf("expected proxy name eth, got %s", network.ProxyName)
	}
	if network.Capabilities == nil || network.Capabilities.Cmp(big.NewInt(0)) != 0 {
		t.Fatalf("expected zero capabilities")
	}
	if network.NetworkConfig == nil || network.NetworkConfig.Handler != "ethereum" {
		t.Fatalf("expected network config to be populated")
	}
	if len(network.MethodsByName) != 1 {
		t.Fatalf("expected methods mapping")
	}
	method, ok := network.MethodsByName["eth_blockNumber"]
	if !ok || method.Bit != 1 {
		t.Fatalf("expected method mapping with bit 1")
	}

	provider, ok := network.Providers["Provider One"]
	if !ok {
		t.Fatalf("expected provider to be mapped")
	}
	if provider.AuthConfig == nil || provider.AuthConfig.Type != ProviderAuthTypeAPIKEY {
		t.Fatalf("expected provider auth config to be mapped")
	}

	networkService, ok := provider.NetworkServices["https://endpoint.example.com"]
	if !ok {
		t.Fatalf("expected network service to be mapped")
	}
	if networkService.Status != NetworkServiceStatusActive {
		t.Fatalf("expected network service status active")
	}
	if networkService.Capabilities == nil || networkService.Capabilities.Cmp(big.NewInt(0)) != 0 {
		t.Fatalf("expected network service capabilities to be zero")
	}

}
