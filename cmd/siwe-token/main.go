package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/DIN-center/din-caddy-plugins/lib/auth/siwe"
	"os"
	"strings"
	"go.uber.org/zap"
)

func main() {
	// Define flags
	help := flag.Bool("help", false, "Display help text")

	// Parse flags
	flag.Parse()

	if *help || len(flag.Args()) < 2 {
		fmt.Println("Usage: app <SIWE client URL> <Private key file path>")
		flag.PrintDefaults()
		os.Exit(1)
	}

	url := flag.Arg(0)
	privateKeyFile := flag.Arg(1)

	// Read the private key file
	hexKeyBytes, err := os.ReadFile(privateKeyFile)
	if err != nil {
		panic(fmt.Sprintf("Failed to read private key file: %v", err.Error()))
	}
	// Trim the 0x prefix and any whitespace
	hexKey := string(hexKeyBytes)
	hexKey = strings.TrimSpace(strings.TrimPrefix(hexKey, "0x"))
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		panic(fmt.Sprintf("Failed to decode secret key: %v", err.Error()))
	}

	// Create a new signer with the key
	signer := &siwe.SigningConfig{
		PrivateKey: key,
		SignerURL:  "http://localhost",
	}

	// Generate a new keypair
	(&siwe.SIWESignerClient{}).GenPrivKey(signer)

	// Print the address
	os.Stderr.WriteString("Your signing address: ")
	os.Stderr.WriteString(signer.Address)
	os.Stderr.WriteString("\n")

	// Create a new SIWE client
	client := siwe.NewSIWEClient(url, 0, signer)

	// Start the client
	client.Start(zap.NewNop())

	// Get a token from the client
	token, err := client.GetToken(nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to get token: %v", err.Error()))
	}
	result := make([]string, 0, len(token.Headers))
	for k, v := range token.Headers {
		result = append(result, fmt.Sprintf("%v: %v", k, v))
	}
	os.Stderr.WriteString("Add to CURL:\n-H '")
	os.Stdout.WriteString(strings.Join(result, " "))
	os.Stderr.WriteString("'\n")
}
