package main

import (
	"fmt"
	"log"
	"os"

	"github.com/DIN-center/din-sc/apps/din-go/lib/din"
	"github.com/DIN-center/din-sc/apps/din-go/lib/watcher"
	"go.uber.org/zap"
)

func main() {
	arg := os.Args[1]
	dinClient, err := din.NewDinClient(zap.NewNop(), "", "")
	if err != nil {
		log.Fatalf("Error creating DinClient: %v", err)
	}

	var defaultNetwork = "gyro://mainnet"

	switch arg {
	case "KickTheTires":
		err := dinClient.KickTheTires(defaultNetwork)
		if err != nil {
			log.Fatalf("Error printing DIN registry data: %v", err)
		}
	case "PrintGetAllMethodsByNetwork":
		err := dinClient.PrintGetAllMethodsByNetwork(defaultNetwork)
		if err != nil {
			log.Fatalf("Error printing all methods by network: %v", err)
		}
	case "PrintListAllMethodsByNetwork":
		err := dinClient.PrintListAllMethodsByNetwork(defaultNetwork)
		if err != nil {
			log.Fatalf("Error printing all methods by network: %v", err)
		}
	case "PrintGetAllNetworks":
		err := dinClient.PrintGetAllNetworks()
		if err != nil {
			log.Fatalf("Error printing all networks: %v", err)
		}
	case "PrintGetNetworkCapabilities":
		err := dinClient.PrintGetNetworkCapabilities(defaultNetwork)
		if err != nil {
			log.Fatalf("Error printing network capabilities: %v", err)
		}
	case "PrintGetAllProviders":
		err := dinClient.PrintGetAllProviders(defaultNetwork)
		if err != nil {
			log.Fatalf("Error printing all providers: %v", err)
		}
	case "PrintGetProvidersByNetwork":
		err := dinClient.PrintGetProvidersByNetwork(defaultNetwork)
		if err != nil {
			log.Fatalf("Error printing providers by network: %v", err)
		}
	case "GetRegistryData":
		dinClient.PrintRegistryData()
		if err != nil {
			log.Fatalf("Error printing registry data: %v", err)
		}
	case "WatcherClient":
		baseURL := os.Args[2]
		apiKey := os.Args[3]

		watcherClient := watcher.NewClient(baseURL, apiKey)

		endpoint := os.Args[4]

		switch endpoint {
		case "check":
			fmt.Println("Calling check endpoint...")
			checkID := os.Args[5]
			network := os.Args[6]

			err := watcherClient.PrintCheckResult(watcher.CheckQueryParams{
				Network: network,
				CheckID: checkID,
			})
			if err != nil {
				log.Fatalf("Error printing check result: %v\n", err)
			}

		case "latency":
			network := os.Args[5]

			err := watcherClient.PrintLatencyResult(watcher.LatencyQueryParams{
				Network: network,
			})
			if err != nil {
				log.Fatalf("Error printing latency result: %v\n", err)
			}
		default:
			log.Fatalf("Invalid endpoint: %v", endpoint)
		}

	default:
		log.Fatalf("Invalid argument: %v", arg)
	}
}
